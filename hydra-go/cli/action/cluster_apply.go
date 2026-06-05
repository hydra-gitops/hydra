package action

import (
	"context"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ClusterApplyFlags struct {
	flags.ClusterRESTClientFlags
	flags.ClusterApplyBehaviorFlags
	flags.ClusterApplyBootstrapNoFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.DryRunFlag
	flags.ReplaceFlag
	flags.NoClusterFlag
	flags.CrdModeFlag
	flags.KubernetesVersionFlag
	flags.BootstrapFlag
	flags.SkipBootstrapGuardFlag
	flags.SkipRefChecksFlag
	flags.ScaleTimeoutFlag
	flags.CrdTimeoutFlag
	flags.ForceBackupRestoreFlag
	flags.SkipBackupRestoreFlag
	flags.ExcludeAppFlag
	flags.PredicatesFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern

	// EffectiveSyncWindow is set in the command RunE from --sync (see resolveClusterApplySyncWindow); if empty, prepareApplyState treats it as default.
	EffectiveSyncWindow types.ClusterApplySyncWindow
}

func (f *ClusterApplyFlags) Flags() flags.Flags {
	return f
}

func zeroWorkloadsMerged(scaleMap map[types.GVKString]types.HydraScaleGroup, entities entity.Entities) (entity.Entities, error) {
	return commands.ZeroWorkloads(entities, types.KeyTemplateEntity, scaleMap)
}

// ClusterApply renders the selected app(s) and applies their manifests to the cluster.
// The flow is built from automatically numbered phases in buildApplyPhases (optional steps are omitted when their flags are unset), with backup restore modeled as a normal phase when enabled.
func ClusterApply(f ClusterApplyFlags) error {
	state, err := prepareApplyState(f)
	if err != nil {
		return err
	}

	if state.flags.NoCluster {
		if _, err := commands.LogStartupOrder(state.l, state.rendered, state.refs, types.KeyTemplateEntity, state.scaleMap); err != nil {
			return err
		}
		state.l.Info(logIdAction, "no-cluster mode: rendered {entities} entities ({crds} CRDs, {ns} namespaces, {resources} resources), resolved {refs} refs — skipping cluster operations",
			log.Int("entities", state.rendered.Len()),
			log.Int("crds", state.crds.Len()),
			log.Int("ns", state.namespaces.Len()),
			log.Int("resources", state.nonCrds.Len()),
			log.Int("refs", len(state.refs)))
		return nil
	}

	if err := logOrphans(state.l, bool(state.flags.Color), state.orphans); err != nil {
		return err
	}

	logApplyPlanSummary(state.l, state.ops)
	if err := ValidateReplaceFlag(state.flags.Replace, state.ops); err != nil {
		return err
	}

	if state.flags.Bootstrap != types.BootstrapYes && !state.flags.SkipRefChecks {
		var backupSecretIDs sets.Set[types.Id]
		if state.flags.BackupRestore && !state.flags.SkipBackupRestore {
			var err error
			backupSecretIDs, err = commands.BackupRestoreCandidateSecretIDs(
				state.cluster,
				state.appIdSlice,
				state.flags.HelmNetworkMode,
				state.flags.KubernetesVersion,
			)
			if err != nil {
				return err
			}
		}
		if err := commands.ValidateClusterApplyPostPlanRefs(
			state.l,
			state.cluster,
			state.flags.HelmNetworkMode,
			state.appIds,
			state.rendered,
			state.clusterEntities,
			state.orphans,
			state.refs,
			backupSecretIDs,
		); err != nil {
			return err
		}
	}

	phases := buildApplyPhases(state)

	err = runApplyPhases(context.Background(), phases, state)
	if err != nil {
		return err
	}

	if state.syncWindowMutationCount > 0 {
		state.l.Info(logIdAction, "ArgoCD AppProject sync was updated for {count} project(s). To change sync mode afterward, use `hydra argocd sync` or `hydra gitops sync` (see manual).",
			log.Int("count", state.syncWindowMutationCount))
	}

	return nil
}

// excludeBootstrapOutOfScopeBackupResources removes backup SopsSecret resources
// that are not part of the appId-selected backup manifest set. Ordinary
// selected-app SopsSecrets remain in the later bootstrap apply set.
func excludeBootstrapOutOfScopeBackupResources(
	entities entity.Entities,
) (entity.Entities, error) {
	filtered := make([]entity.Entity, 0, entities.Len())
	for _, e := range entities.Items {
		exclude, err := isBootstrapOutOfScopeBackupResource(e)
		if err != nil {
			return entity.Entities{}, err
		}
		if exclude {
			continue
		}
		filtered = append(filtered, e)
	}

	return entity.NewEntities(filtered)
}

func isBootstrapOutOfScopeBackupResource(
	e entity.Entity,
) (bool, error) {
	kind, err := e.Kind()
	if err != nil {
		return false, err
	}
	if kind != types.Kind("SopsSecret") {
		return false, nil
	}

	u, ok := e.Unstructured(types.KeyTemplateEntity)
	if !ok {
		return false, nil
	}

	if u.GetAnnotations()[hydra.AnnotationHydraBackup] != "true" {
		return false, nil
	}

	appIds, err := e.AppIds()
	if err != nil || len(appIds) == 0 {
		return true, nil
	}

	namespace, err := e.Namespace()
	if err != nil || namespace == "" {
		return true, nil
	}

	appNamespace, err := e.AppNamespace()
	if err != nil || appNamespace == "" {
		return true, nil
	}

	return namespace != types.Namespace(appNamespace), nil
}

func scaleUpPhase(
	l log.Logger,
	cc *k8s.ClusterClient,
	cluster *hydra.Cluster,
	renderedEntities entity.Entities,
	clusterEntities entity.Entities,
	refs []types.Ref,
	scaleMap map[types.GVKString]types.HydraScaleGroup,
	f *ClusterApplyFlags,
	phase int,
	totalPhases int,
	workflowID string,
) error {
	l.Info(logIdAction, commands.PhaseMessageWithID(phase, totalPhases, "scaling up workloads in dependency order", false, workflowID))
	if !f.DryRun {
		scaleEntities, err := renderedEntities.Merge(clusterEntities, types.KeyClusterEntity)
		if err != nil {
			return err
		}
		tree, err := commands.LoadInspectRefGraph(commands.InspectRefGraphParams{
			Cluster:                     cluster,
			NetworkMode:                 f.HelmNetworkMode,
			Bootstrap:                   f.Bootstrap,
			ClusterInventory:            &clusterEntities,
			IncludeTemplateRefs:         true,
			IncludeClusterRefs:          true,
			IncludeCloneMaterialization: true,
			SkipFoundDefinitionsInfoLog: true,
		})
		if err != nil {
			return err
		}
		var fullRefs []types.Ref
		if tree != nil {
			fullRefs = tree.Refs
		}
		readyEval, err := commands.ReadyEvaluatorFromHydra(cluster, f.HelmNetworkMode, scaleMap, scaleEntities, types.KeyClusterEntity)
		if err != nil {
			return err
		}
		return commands.ScaleUpWorkloads(context.Background(), l, cc.Dynamic,
			scaleEntities, refs, fullRefs, types.KeyTemplateEntity, types.KeyClusterEntity, f.DryRun, f.ScaleTimeout, readyEval, scaleMap)
	}
	l.Info(logIdAction, "[dry-run] would scale up workloads")
	return nil
}

func disableNonReadyWebhooksPhase(
	l log.Logger,
	cc *k8s.ClusterClient,
	webhookEntities entity.Entities,
	nonWebhookEntities entity.Entities,
	allEntities entity.Entities,
	f *ClusterApplyFlags,
	phase int,
	totalPhases int,
	workflowID string,
) error {
	if webhookEntities.Len() == 0 {
		l.Info(logIdAction, commands.PhaseMessageWithID(phase, totalPhases, "disabling non-ready webhook configurations", true, workflowID))
		return nil
	}

	// Log the phase banner before per-webhook analysis (FilterWebhooksToDisable) and cluster calls.
	l.Info(logIdAction, commands.PhaseMessageWithID(phase, totalPhases, "disabling non-ready webhook configurations", false, workflowID))

	isProviderReady := func(provider commands.WebhookProvider) (bool, error) {
		return k8s.IsWorkloadReady(context.Background(), cc.Dynamic, provider.Workload)
	}

	toDisable, _, err := commands.FilterWebhooksToDisable(
		l, webhookEntities, nonWebhookEntities, allEntities,
		types.KeyTemplateEntity, isProviderReady,
	)
	if err != nil {
		return err
	}

	if toDisable.Len() == 0 {
		l.Info(logIdAction, commands.PhaseMessageWithID(phase, totalPhases, "disabling non-ready webhook configurations (all providers ready)", true, workflowID))
		return nil
	}

	return k8s.DisableWebhookConfigs(context.Background(), l, cc.Dynamic,
		toDisable, types.KeyTemplateEntity, f.DryRun)
}

func diffAndApplyCRDs(
	l log.Logger,
	cc *k8s.ClusterClient,
	cluster *hydra.Cluster,
	crdOps ApplyOperations,
	f *ClusterApplyFlags,
	useFooter bool,
) error {
	if !crdOps.HasMutatingWork() {
		return nil
	}

	var appliedCrds []entity.Entity

	newCrds := crdOps.New
	if newCrds.Len() > 0 {
		l.Info(logIdAction, "applying {new} new CRDs (server-side apply)", log.Int("new", newCrds.Len()))
		k8s.FlushProgressLogBeforeFooter()
		if aErr := k8s.Apply(context.Background(), l, cc, newCrds, types.KeyTemplateEntity, f.DryRun, true, nil, nil, useFooter); aErr != nil {
			return aErr
		}
		appliedCrds = append(appliedCrds, newCrds.Items...)
	}

	existingMut, err := crdOps.MergeUpdateReplace()
	if err != nil {
		return err
	}
	if existingMut.Len() > 0 {
		crdPatchFailures := filterPatchFailures(crdOps.PatchFailures, crdOps.Replace)
		l.Info(logIdAction, "applying {count} existing CRD updates (server-side apply)", log.Int("count", existingMut.Len()))
		crdDeleteSet := logDeleteBeforeApplySet(l, f.Replace, crdPatchFailures)
		k8s.FlushProgressLogBeforeFooter()
		if aErr := k8s.Apply(context.Background(), l, cc, existingMut, types.KeyTemplateEntity, f.DryRun, true, crdDeleteSet, nil, useFooter); aErr != nil {
			return aErr
		}
		appliedCrds = append(appliedCrds, existingMut.Items...)
	}

	if !f.DryRun && len(appliedCrds) > 0 {
		appliedCrdEntities, nErr := entity.NewEntities(appliedCrds)
		if nErr != nil {
			return nErr
		}
		crdNames := collectCRDNames(appliedCrdEntities)
		return k8s.WaitForCRDsEstablished(context.Background(), l, cc.Dynamic, crdNames, f.CrdTimeout)
	}

	return nil
}

// ensureBatchWorkloadUnsuspendForApply sets spec.suspend=false on the template manifest when the live cluster
// object is still suspended but the template expects scheduling (suspend absent or false). Server-side apply
// often retains suspend when the field is omitted from the apply configuration.
func ensureBatchWorkloadUnsuspendForApply(ents entity.Entities) (entity.Entities, error) {
	if ents.Len() == 0 {
		return ents, nil
	}
	out := make([]entity.Entity, 0, ents.Len())
	for _, e := range ents.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}
		if gvk != types.KubernetesGvkBatchV1Job && gvk != types.KubernetesGvkBatchV1CronJob {
			out = append(out, e)
			continue
		}
		clusterU, hasCluster := e.Unstructured(types.KeyClusterEntity)
		tplU, hasTpl := e.Unstructured(types.KeyTemplateEntity)
		if !hasCluster || !hasTpl {
			out = append(out, e)
			continue
		}
		liveSuspended, liveOk := values.Lookup(clusterU.Object, "spec", "suspend").(bool)
		if !liveOk || !liveSuspended {
			out = append(out, e)
			continue
		}
		if v, ok := values.Lookup(tplU.Object, "spec", "suspend").(bool); ok && v {
			out = append(out, e)
			continue
		}
		modified := *tplU.DeepCopy()
		if specMap, ok := modified.Object["spec"].(map[string]any); ok {
			specMap["suspend"] = false
		} else {
			modified.Object["spec"] = map[string]any{"suspend": false}
		}
		ne, mErr := e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(types.KeyTemplateEntity, modified)
		})
		if mErr != nil {
			return entity.Entities{}, mErr
		}
		out = append(out, ne)
	}
	return entity.NewEntities(out)
}

// applyWorkloadFromOps applies new, update, and replace workload resources from a pre-classified ApplyOperations slice.
// When zeroWorkloads is false, template replica counts are preserved. AppProject syncWindows are adjusted per
// f.EffectiveSyncWindow before apply; syncWindowMutations accumulates how many AppProjects were actually modified.
func applyWorkloadFromOps(
	l log.Logger,
	cc *k8s.ClusterClient,
	workloadOps ApplyOperations,
	f *ClusterApplyFlags,
	scaleMap map[types.GVKString]types.HydraScaleGroup,
	zeroWorkloads bool,
	disableWebhookConfigs bool,
	clusterEntities entity.Entities,
	syncWindowMutations *int,
	useFooter bool,
) error {
	zeroOrSelf := func(ents entity.Entities) (entity.Entities, error) {
		out := ents
		if zeroWorkloads {
			var err error
			out, err = zeroWorkloadsMerged(scaleMap, out)
			if err != nil {
				return entity.Entities{}, err
			}
		}
		if disableWebhookConfigs {
			var err error
			out, err = commands.SetWebhookFailurePolicy(out, types.KeyTemplateEntity, "Ignore")
			if err != nil {
				return entity.Entities{}, err
			}
		}
		return out, nil
	}

	applySyncWindow := func(ents entity.Entities, isNew bool) (entity.Entities, error) {
		out, n, pErr := commands.ApplyClusterApplySyncWindowToEntities(
			l, ents, clusterEntities, types.KeyTemplateEntity, types.KeyClusterEntity, f.EffectiveSyncWindow, isNew)
		if pErr != nil {
			return entity.Entities{}, pErr
		}
		if syncWindowMutations != nil && !f.DryRun {
			*syncWindowMutations += n
		}
		return out, nil
	}

	newEntities := workloadOps.New
	if newEntities.Len() > 0 {
		ents, zErr := zeroOrSelf(newEntities)
		if zErr != nil {
			return zErr
		}
		ents, zErr = applySyncWindow(ents, true)
		if zErr != nil {
			return zErr
		}
		l.Info(logIdAction, "applying {count} new resources", log.Int("count", ents.Len()))
		if err := applyEntitiesBySSA(l, cc, ents, types.KeyTemplateEntity, f.DryRun, nil, useFooter); err != nil {
			return err
		}
	}

	changed, err := workloadOps.MergeUpdateReplace()
	if err != nil {
		return err
	}
	if changed.Len() == 0 {
		return nil
	}

	ents, zErr := zeroOrSelf(changed)
	if zErr != nil {
		return zErr
	}
	ents, zErr = applySyncWindow(ents, false)
	if zErr != nil {
		return zErr
	}
	if !zeroWorkloads {
		ents, zErr = ensureBatchWorkloadUnsuspendForApply(ents)
		if zErr != nil {
			return zErr
		}
	}
	l.Info(logIdAction, "applying {count} changed resources", log.Int("count", ents.Len()))
	patchFailures := filterPatchFailures(workloadOps.PatchFailures, workloadOps.Replace)
	deleteSet := logDeleteBeforeApplySet(l, f.Replace, patchFailures)
	return applyEntitiesBySSA(l, cc, ents, types.KeyTemplateEntity, f.DryRun, deleteSet, useFooter)
}

func splitNewAndExisting(
	templateEntities entity.Entities,
	clusterEntities entity.Entities,
) (newEntities entity.Entities, existingEntities entity.Entities, err error) {
	merged, err := templateEntities.Merge(
		clusterEntities,
		types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}

	compareResult, err := merged.Compare(types.KeyTemplateEntity, types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}

	return compareResult.LeftOnly, compareResult.Both, nil
}

// findChangedEntities compares the cluster state (KeyClusterEntity) against the
// server-side dry-run result (KeyDryRunEntity) for each entity. Returns only
// entities where the YAML representation differs after global.hydra.diff.ignore
// normalization (including the built-in rule that ignores spec.replicas for
// Deployment/ReplicaSet/StatefulSet). As an exception, when the live object has
// spec.replicas == 0 and the dry-run desired count is > 0, the entity is still
// treated as changed so a scaled-down workload can be restored to template scale.
// Entities without KeyDryRunEntity (e.g. due to SSA failure) are conservatively treated as changed.
func findChangedEntities(l log.Logger, entities entity.Entities, diffIgnore *commands.DiffIgnorePipeline) (entity.Entities, error) {
	replicaWorkloadGVKs := []types.GVKString{
		types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1ReplicaSet,
		types.KubernetesGvkAppsV1StatefulSet,
	}

	var changed []entity.Entity
	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}

		clusterU, hasCluster := e.Unstructured(types.KeyClusterEntity)
		if !hasCluster {
			l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: skipped (no cluster entity in merge)",
				log.String("id", string(id)), log.String("result", "skipped"))
			continue
		}

		dryRunU, hasDryRun := e.Unstructured(types.KeyDryRunEntity)
		if !hasDryRun {
			l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: {result} (no SSA dry-run object; treated as mutating)",
				log.String("id", string(id)), log.String("result", "changed"),
				log.String("reason", "missing_dry_run"))
			changed = append(changed, e)
			continue
		}

		clusterCopy := *clusterU.DeepCopy()
		dryRunCopy := *dryRunU.DeepCopy()

		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}

		var liveReplicas, desiredReplicas int64
		var liveReplicasOk, desiredReplicasOk bool
		if slices.Contains(replicaWorkloadGVKs, gvk) {
			liveReplicas, liveReplicasOk = specReplicasIfPresent(&clusterCopy)
			desiredReplicas, desiredReplicasOk = specReplicasIfPresent(&dryRunCopy)
		}

		if diffIgnore != nil {
			if err := diffIgnore.ApplyToUnstructured(e, &clusterCopy); err != nil {
				return entity.Entities{}, err
			}
			if err := diffIgnore.ApplyToUnstructured(e, &dryRunCopy); err != nil {
				return entity.Entities{}, err
			}
		}

		clusterYaml, err := yaml.PrintObject(types.KeepServerFieldsNo, nil, &clusterCopy)
		if err != nil {
			return entity.Entities{}, err
		}
		dryRunYaml, err := yaml.PrintObject(types.KeepServerFieldsNo, nil, &dryRunCopy)
		if err != nil {
			return entity.Entities{}, err
		}

		if clusterYaml != dryRunYaml {
			l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: {result} (cluster vs SSA dry-run YAML differs)",
				log.String("id", string(id)), log.String("result", "changed"),
				log.String("reason", "yaml_diff"))
			changed = append(changed, e)
			continue
		}

		// Restoring a workload that was scaled to 0 while the template expects >0 replicas.
		if slices.Contains(replicaWorkloadGVKs, gvk) && liveReplicasOk && desiredReplicasOk &&
			liveReplicas == 0 && desiredReplicas > 0 {
			l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: {result} (live replicas 0, template wants >0)",
				log.String("id", string(id)), log.String("result", "changed"),
				log.String("reason", "restore_replicas_after_scale_zero"))
			changed = append(changed, e)
			continue
		}

		// Restoring a Job/CronJob suspended in the cluster while the template allows scheduling (spec.suspend
		// absent or false). SSA dry-run can match live YAML while the field stays true, same class of issue as
		// scaled-to-zero Deployments above.
		if gvk == types.KubernetesGvkBatchV1Job || gvk == types.KubernetesGvkBatchV1CronJob {
			liveSuspended, liveOk := values.Lookup(clusterCopy.Object, "spec", "suspend").(bool)
			if liveOk && liveSuspended {
				tplSuspended := false
				if tu, ok := e.Unstructured(types.KeyTemplateEntity); ok {
					if v, ok := values.Lookup(tu.Object, "spec", "suspend").(bool); ok && v {
						tplSuspended = true
					}
				}
				if !tplSuspended {
					l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: {result} (live batch workload suspended, template wants scheduling)",
						log.String("id", string(id)), log.String("result", "changed"),
						log.String("reason", "restore_batch_suspend_after_scale_zero"))
					changed = append(changed, e)
					continue
				}
			}
		}

		l.DebugLog(logIdApplyDryRunDiff, "apply dry-run diff {id}: {result}",
			log.String("id", string(id)), log.String("result", "unchanged"))
	}

	return entity.NewEntities(changed)
}

func specReplicasIfPresent(u *unstructured.Unstructured) (int64, bool) {
	r := values.Lookup(u.Object, "spec", "replicas")
	if r == nil {
		return 0, false
	}
	switch n := r.(type) {
	case int64:
		return n, true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case float64:
		return int64(n), true
	case float32:
		return int64(n), true
	default:
		return 0, false
	}
}

func newClusterClient(cluster *hydra.Cluster) (*k8s.ClusterClient, error) {
	configFlags, err := hydra.HydraClusterAccess(cluster)
	if err != nil {
		return nil, err
	}
	lim := cluster.RESTClientLimits
	return k8s.NewClusterClientWithRESTOverrides(configFlags, lim.QPS, lim.Burst)
}

func splitCRDs(entities entity.Entities) (crds entity.Entities, nonCrds entity.Entities, err error) {
	var crdItems []entity.Entity
	var nonCrdItems []entity.Entity
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if gvk == types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition {
			crdItems = append(crdItems, e)
		} else {
			nonCrdItems = append(nonCrdItems, e)
		}
	}
	crds, err = entity.NewEntities(crdItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	nonCrds, err = entity.NewEntities(nonCrdItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return crds, nonCrds, nil
}

func splitNamespaces(entities entity.Entities) (namespaces entity.Entities, rest entity.Entities, err error) {
	var nsItems []entity.Entity
	var restItems []entity.Entity
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if gvk == types.KubernetesGvkV1Namespace {
			nsItems = append(nsItems, e)
		} else {
			restItems = append(restItems, e)
		}
	}
	namespaces, err = entity.NewEntities(nsItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	rest, err = entity.NewEntities(restItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return namespaces, rest, nil
}

func splitWebhooks(entities entity.Entities) (webhooks entity.Entities, rest entity.Entities, err error) {
	var webhookItems []entity.Entity
	var restItems []entity.Entity
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration ||
			gvk == types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration {
			webhookItems = append(webhookItems, e)
		} else {
			restItems = append(restItems, e)
		}
	}
	webhooks, err = entity.NewEntities(webhookItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	rest, err = entity.NewEntities(restItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return webhooks, rest, nil
}

func collectCRDNames(crdEntities entity.Entities) []string {
	var names []string
	for _, e := range crdEntities.Items {
		name, err := e.Name()
		if err != nil {
			continue
		}
		names = append(names, string(name))
	}
	return names
}

// logDeleteBeforeApplySet logs which resources get a delete-before-apply and returns their IDs as a set.
// Immutable conflicts are handled automatically when the API reports them; cliReplace (--replace)
// additionally covers non-immutable dry-run failures.
func logDeleteBeforeApplySet(l log.Logger, cliReplace bool, patchFailures []commands.DryRunPatchFailure) sets.Set[types.Id] {
	if len(patchFailures) == 0 {
		if cliReplace {
			l.Info(logIdAction, "replace flag: no SSA dry-run failures; applying without pre-delete")
		}
		return nil
	}
	ids := make([]types.Id, 0, len(patchFailures))
	idStrs := make([]string, 0, len(patchFailures))
	for _, pf := range patchFailures {
		ids = append(ids, pf.Id)
		idStrs = append(idStrs, string(pf.Id))
	}
	joined := strings.Join(idStrs, ", ")
	if cliReplace {
		l.Info(logIdAction, "deleting existing objects before apply for {count} resource(s) that failed SSA dry-run (--replace): {resources}",
			log.Int("count", len(patchFailures)),
			log.String("resources", joined))
	} else {
		l.Info(logIdAction, "deleting existing objects before apply for {count} resource(s) with immutable field conflicts (per API): {resources}",
			log.Int("count", len(patchFailures)),
			log.String("resources", joined))
	}
	return sets.New[types.Id](ids...)
}

func applyEntitiesBySSA(l log.Logger, cc *k8s.ClusterClient, entities entity.Entities, key types.EntityKeyUnstructured, dryRun types.DryRun, deleteBeforeApply sets.Set[types.Id], useFooter bool) error {
	var ssaItems []entity.Entity
	var regularItems []entity.Entity
	for _, e := range entities.Items {
		if commands.ShouldServerSideApply(e, key) {
			ssaItems = append(ssaItems, e)
		} else {
			regularItems = append(regularItems, e)
		}
	}

	if len(regularItems) > 0 {
		regularEntities, err := entity.NewEntities(regularItems)
		if err != nil {
			return err
		}
		l.Info(logIdAction, "applying {count} resources (regular)", log.Int("count", regularEntities.Len()))
		k8s.FlushProgressLogBeforeFooter()
		if err := k8s.Apply(context.Background(), l, cc, regularEntities, key, dryRun, false, deleteBeforeApply, nil, useFooter); err != nil {
			return err
		}
	}

	if len(ssaItems) > 0 {
		ssaEntities, err := entity.NewEntities(ssaItems)
		if err != nil {
			return err
		}
		l.Info(logIdAction, "applying {count} resources (server-side apply)", log.Int("count", ssaEntities.Len()))
		k8s.FlushProgressLogBeforeFooter()
		if err := k8s.Apply(context.Background(), l, cc, ssaEntities, key, dryRun, true, deleteBeforeApply, nil, useFooter); err != nil {
			return err
		}
	}

	return nil
}

// findOrphans identifies resources in the cluster that are managed by the given apps
// but are no longer present in the rendered templates.
func findOrphans(
	cluster *hydra.Cluster,
	clusterEntities entity.Entities,
	renderedEntities entity.Entities,
	appIds sets.Set[types.AppId],
) (entity.Entities, error) {
	l := cluster.L()

	merged, err := renderedEntities.Merge(
		clusterEntities,
		types.KeyTemplateEntity,
		types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, err
	}

	compareResult, err := merged.Compare(types.KeyTemplateEntity, types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, err
	}

	// RightOnly = in cluster but not in templates -> potential orphans
	if compareResult.RightOnly.Len() == 0 {
		return entity.Entities{}, nil
	}

	// filter to only resources managed by the selected apps (via ArgoCD tracking ID)
	orphanCandidates, err := compareResult.RightOnly.UnselectAll()
	if err != nil {
		return entity.Entities{}, err
	}

	orphanCandidates, err = commands.MarkAsSelectedArgoCdManagedResources(l, orphanCandidates, types.KeyClusterEntity, appIds)
	if err != nil {
		return entity.Entities{}, err
	}

	selected, err := orphanCandidates.Selected()
	if err != nil {
		return entity.Entities{}, err
	}

	// exclude resources with ownerReferences — these are controller-managed
	filtered := make([]entity.Entity, 0, selected.Len())
	for _, e := range selected.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if commands.IsKubernetesAPIServerManagedKubeRootCAConfigMap(id) {
			continue
		}
		owners := e.OwnerUids(types.KeyClusterEntity)
		if owners != nil && owners.Len() > 0 {
			continue
		}
		filtered = append(filtered, e)
	}

	orphans, err := entity.NewEntities(filtered)
	if err != nil {
		return entity.Entities{}, err
	}

	l.DebugLog(logIdAction, "identified {count} orphaned resources", log.Int("count", orphans.Len()))
	return orphans, nil
}
