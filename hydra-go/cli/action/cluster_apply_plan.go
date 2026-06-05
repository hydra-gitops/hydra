package action

import (
	"context"
	"fmt"
	"strings"

	"slices"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	basephase "hydra-gitops.org/hydra/hydra-go/base/phase"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/sops"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

var backupRestoreWithOptions = commands.BackupRestoreWithOptions
var printBackupResults = commands.PrintBackupResults

type applyState struct {
	l               log.Logger
	flags           ClusterApplyFlags
	cluster         *hydra.Cluster
	appIds          sets.Set[types.AppId]
	appIdSlice      []types.AppId
	scaleMap        map[types.GVKString]types.HydraScaleGroup
	rendered        entity.Entities
	crds            entity.Entities
	namespaces      entity.Entities
	nonCrds         entity.Entities
	clusterEntities entity.Entities
	orphans         entity.Entities
	refs            []types.Ref
	cc              *k8s.ClusterClient

	// ops is the classified apply plan (new/update/replace/unchanged/delete) after template vs cluster comparison.
	ops *ApplyOperations

	currentPhase int
	totalPhases  int

	// currentWorkflowID is the stable id for the active phase (for logs).
	currentWorkflowID string

	// syncWindowMutationCount is the number of AppProjects whose syncWindows were mutated in this run (see --sync).
	syncWindowMutationCount int

	restoreBackups func(context.Context) basephase.Result
}

func (s *applyState) phaseMessage(description string, skipped bool) string {
	return commands.PhaseMessageWithID(s.currentPhase, s.totalPhases, description, skipped, s.currentWorkflowID)
}

// logClusterApplyResolvedAppIDs prints the final sorted app ID set after pattern and exclude resolution.
func logClusterApplyResolvedAppIDs(l log.Logger, appIds sets.Set[types.AppId]) {
	slice := make([]types.AppId, 0, len(appIds))
	for id := range appIds {
		slice = append(slice, id)
	}
	slices.SortFunc(slice, func(a, b types.AppId) int {
		return strings.Compare(string(a), string(b))
	})
	lines := make([]string, len(slice))
	for i, id := range slice {
		lines[i] = "  * " + string(id)
	}
	l.Info(logIdAction, "resolved {count} app ID(s) for this apply:\n{listing}",
		log.Int("count", len(slice)),
		log.String("listing", strings.Join(lines, "\n")))
}

func prepareApplyState(f ClusterApplyFlags) (*applyState, error) {
	l := log.Default()
	if f.EffectiveSyncWindow == "" {
		f.EffectiveSyncWindow = types.ClusterApplySyncWindowDefault
	}
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, true)
	if err != nil {
		return nil, err
	}
	if len(appIds) == 0 {
		return nil, log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for apply")
	}

	logClusterApplyResolvedAppIDs(l, appIds)
	logClusterApplyOptionalBehaviorsTable(l, bool(f.Color), f)

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return nil, err
	}

	allClusterAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return nil, err
	}

	skipRootApps := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	var appRenderOpts []commands.RenderClusterSelectedAppsOption
	if log.TerminalProgressUI() {
		appRenderOpts = []commands.RenderClusterSelectedAppsOption{commands.WithDefinitionsProgress(true)}
	}
	renderedEntities, _, _, err := commands.RenderCluster(cluster, appIds, f.KubernetesVersion, f.CrdMode, skipRootApps, appRenderOpts)
	if err != nil {
		return nil, err
	}

	prepTotal := 0
	if !f.NoCluster {
		prepTotal++
	}
	prepTotal++ // bootstrap guard
	if f.SopsDecode {
		prepTotal++
	}
	prepTotal += 8 // clones, diff-ignore, template-patch rules, ref parsers, preferred versions, refs, scale map, template patches
	if len(f.Predicates) > 0 {
		prepTotal++
	}
	prepTotal++ // exclude kube-root-ca ConfigMaps
	prepTotal++ // split CRDs and namespaces
	if f.BackupRestore && f.SopsDecode {
		prepTotal++
	}
	if f.SopsDecode && f.BackupRestore && !f.SkipBackupRestore {
		prepTotal++
	}
	if !f.NoCluster {
		prepTotal++
	}

	var prepBar log.Progress
	var prepDetailTask log.ProgressTask
	prepStep := 0
	if log.TerminalProgressUI() && prepTotal > 0 {
		prepBar, err = l.NewProgress("prepare · apply", prepTotal)
		if err != nil {
			return nil, err
		}
		prepDetailTask = prepBar.NewTask("")
		defer func() {
			if prepBar != nil {
				_ = prepBar.Close()
				l.Info(logIdAction, "prepare · apply: completed {count} step(s)", log.Int("count", prepTotal))
			}
		}()
	}
	advancePrep := func(detail string) {
		if prepBar == nil {
			return
		}
		prepStep++
		if prepDetailTask != nil {
			prepDetailTask.SetDetail(detail)
		}
		prepBar.Advance(prepStep, prepTotal)
	}

	if !f.NoCluster {
		liveScope, err := commands.ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, f.CrdMode)
		if err != nil {
			return nil, err
		}
		if err := commands.ValidateClusterApplyCrdEligibility(renderedEntities, liveScope, types.KeyTemplateEntity); err != nil {
			return nil, err
		}
		advancePrep("CRD scope")
	}

	if err := commands.ValidateClusterApplyBootstrapGuard(
		l,
		cluster,
		appIds,
		f.HelmNetworkMode,
		renderedEntities,
		f.Bootstrap,
		f.SkipBootstrapGuard,
		f.BootstrapGuard,
	); err != nil {
		return nil, err
	}
	advancePrep("bootstrap guard")

	if f.SopsDecode {
		renderedEntities, err = commands.ConvertSopsSecretsToSecrets(l, renderedEntities, types.KeyTemplateEntity, sops.DecryptSopsYaml)
		if err != nil {
			return nil, err
		}
		advancePrep("SOPS decode")
	}

	cloneBootstrap := types.BootstrapNo
	if f.BootstrapClones {
		cloneBootstrap = types.BootstrapYes
	}
	renderedEntities, _, err = commands.MaterializeHydraClonesForApply(
		l, cluster, appIds, renderedEntities, types.KeyTemplateEntity, cloneBootstrap, f.HelmNetworkMode, nil)
	if err != nil {
		return nil, err
	}
	advancePrep("Hydra clones")

	diffIgnoreEntries, err := hydra.HydraDiffIgnoreRuleEntries(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return nil, err
	}
	advancePrep("diff-ignore rules")

	var hydraConfigCatalog entity.Entities
	if !appIds.Equal(allClusterAppIds) {
		catalogOpts := []commands.RenderClusterSelectedAppsOption{commands.WithSkipFoundDefinitionsInfoLog()}
		hydraConfigCatalog, err = commands.RenderClusterSelectedApps(
			cluster, f.HelmNetworkMode, f.KubernetesVersion, allClusterAppIds, types.KeyTemplateEntity, catalogOpts...)
		if err != nil {
			return nil, err
		}
	}

	templatePatchEntries, err := hydra.HydraTemplatePatchRuleEntries(cluster, appIds, f.HelmNetworkMode, renderedEntities, hydraConfigCatalog)
	if err != nil {
		return nil, err
	}
	var templatePatchOwnerByNs map[types.Namespace]types.AppId
	if len(templatePatchEntries) > 0 {
		templatePatchOwnerByNs, err = commands.BuildTemplatePatchOwnerByNamespace(
			cluster, f.HelmNetworkMode, renderedEntities, hydraConfigCatalog, types.KeyTemplateEntity)
		if err != nil {
			return nil, err
		}
	}
	advancePrep("template-patch rules")

	scaleMap, err := commands.MergedScaleWorkloadMap(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return nil, err
	}
	advancePrep("scale workloads")

	templatePatchPipe, err := commands.NewTemplatePatchPipelineWithNamespaceOwners(templatePatchEntries, templatePatchOwnerByNs)
	if err != nil {
		return nil, err
	}
	renderedEntities, err = commands.ApplyTemplatePatchesToEntities(templatePatchPipe, renderedEntities, types.KeyTemplateEntity)
	if err != nil {
		return nil, err
	}
	advancePrep("template patches")

	var resourcePredicate cel.Predicate
	if len(f.Predicates) > 0 {
		env, err := cel.NewEnv()
		if err != nil {
			return nil, err
		}
		resourcePredicate, err = env.CompilePredicateAt(`hydra gitops apply plan --predicate`, f.Predicates...)
		if err != nil {
			return nil, err
		}
		beforeFilter := renderedEntities.Len()
		filteredRendered, err := filterDiffEntitiesByPredicate(renderedEntities.Items, resourcePredicate)
		if err != nil {
			return nil, err
		}
		renderedEntities, err = entity.NewEntities(filteredRendered)
		if err != nil {
			return nil, err
		}
		l.Info(logIdAction, "apply resource filter: {selected} rendered resource(s) match --include/--exclude (from {total} after template patches)",
			log.Int("selected", renderedEntities.Len()),
			log.Int("total", beforeFilter))
		advancePrep("resource filter")
	}

	beforeKubeRootCA := renderedEntities.Len()
	renderedEntities, err = commands.ExcludeKubernetesAPIServerKubeRootCAConfigMaps(renderedEntities)
	if err != nil {
		return nil, err
	}
	if n := beforeKubeRootCA - renderedEntities.Len(); n > 0 {
		l.Info(logIdAction, "excluded {count} Kubernetes apiserver-managed ConfigMap/kube-root-ca.crt object(s) from apply",
			log.Int("count", n))
	}
	advancePrep("exclude kube-root-ca")

	appRefParsers, err := hydra.HydraAppRefParsers(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return nil, err
	}
	advancePrep("ref parsers")

	preferredVersions, err := cluster.PreferredVersions(nil)
	if err != nil {
		return nil, err
	}
	advancePrep("preferred API versions")

	refs, err := references.Refs(l, renderedEntities, types.KeyTemplateEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, appRefParsers)
	if err != nil {
		return nil, err
	}
	refs = references.AnnotateRefsWithSource(refs, types.RefSourceTemplate)
	advancePrep("references")

	crdEntities, nonCrdEntities, err := splitCRDs(renderedEntities)
	if err != nil {
		return nil, err
	}
	namespaceEntities, nonCrdEntities, err := splitNamespaces(nonCrdEntities)
	if err != nil {
		return nil, err
	}
	advancePrep("split CRDs/namespaces")
	if f.BackupRestore && f.SopsDecode {
		nonCrdEntities, err = excludeBootstrapOutOfScopeBackupResources(nonCrdEntities)
		if err != nil {
			return nil, err
		}
		advancePrep("backup scope")
	}

	if f.SopsDecode && f.BackupRestore && !f.SkipBackupRestore {
		if err := validateBootstrapBackupSecretConflict(
			cluster,
			appIds,
			f.HelmNetworkMode,
			f.KubernetesVersion,
			nonCrdEntities,
		); err != nil {
			return nil, err
		}
		advancePrep("backup validation")
	}

	appIdSlice := make([]types.AppId, 0, len(appIds))
	for appId := range appIds {
		appIdSlice = append(appIdSlice, appId)
	}

	state := &applyState{
		l:          l,
		flags:      f,
		cluster:    cluster,
		appIds:     appIds,
		appIdSlice: appIdSlice,
		scaleMap:   scaleMap,
		rendered:   renderedEntities,
		crds:       crdEntities,
		namespaces: namespaceEntities,
		nonCrds:    nonCrdEntities,
		refs:       refs,
	}

	if f.NoCluster {
		return state, nil
	}

	advancePrep("cluster client")
	cc, err := newClusterClient(cluster)
	if err != nil {
		return nil, err
	}
	var clusterEntities entity.Entities
	if resourcePredicate != nil {
		clusterEntities, err = commands.ListClusterStrictPredicate(cluster, types.KeyClusterEntity, resourcePredicate, log.TerminalProgressUI(), f.Parallel)
	} else {
		clusterEntities, err = commands.ListClusterAll(cluster, types.KeyClusterEntity, log.TerminalProgressUI(), f.Parallel)
	}
	if err != nil {
		return nil, err
	}
	invModel, err := commands.BuildResourceModel(commands.ResourceModelInput{
		Cluster:         cluster,
		ClusterEntities: &clusterEntities,
		NetworkMode:     types.HelmNetworkModeOffline,
		Bootstrap:       f.Bootstrap,
		Parallel:        f.Parallel,
	}, log.TerminalProgressUI())
	if err != nil {
		return nil, err
	}
	clusterEntities = invModel.ClusterEntities()

	var orphans entity.Entities
	if len(f.Predicates) > 0 {
		orphans, err = entity.NewEntities(nil)
		if err != nil {
			return nil, err
		}
	} else {
		orphans, err = findOrphans(cluster, clusterEntities, renderedEntities, appIds)
		if err != nil {
			return nil, err
		}
	}

	state.cc = cc
	state.clusterEntities = clusterEntities
	state.orphans = orphans

	diffIgnorePipeline, err := commands.NewDiffIgnorePipeline(diffIgnoreEntries)
	if err != nil {
		return nil, err
	}

	ops, err := ClassifyApplyOperations(cluster, renderedEntities, clusterEntities, orphans, diffIgnorePipeline, f.EffectiveSyncWindow, f.Parallel,
		log.TerminalProgressUI())
	if err != nil {
		return nil, err
	}
	state.ops = ops

	state.restoreBackups = func(ctx context.Context) basephase.Result {
		return runRestoreBackups(ctx, state)
	}
	return state, nil
}

func validateBootstrapBackupSecretConflict(
	cluster *hydra.Cluster,
	appIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	nonCrds entity.Entities,
) error {
	appIdSlice := make([]types.AppId, 0, len(appIds))
	for id := range appIds {
		appIdSlice = append(appIdSlice, id)
	}
	slices.SortFunc(appIdSlice, func(a, b types.AppId) int {
		return strings.Compare(string(a), string(b))
	})

	candidates, err := commands.ListBackupRestoreCandidates(
		cluster, appIdSlice, networkMode, kubernetesVersion)
	if err != nil {
		return err
	}
	conflicts, err := commands.FindBootstrapBackupApplyConflicts(candidates, nonCrds)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		return nil
	}
	var b strings.Builder
	b.WriteString("apply aborted: the same v1/Secret is targeted by both backup restore and the rendered bootstrap apply set; remove or consolidate one of the definitions:\n")
	for _, c := range conflicts {
		fmt.Fprintf(&b, "  - %s: backup restore from %s; apply source: %s\n",
			c.SecretID, c.BackupFile, c.ApplySourceDesc)
	}
	return log.CreateError(errors.ErrAborted, strings.TrimSuffix(b.String(), "\n"))
}

func runApplyPhases(ctx context.Context, phases basephase.Items[applyState], state *applyState) error {
	state.totalPhases = len(phases.Items)
	for _, item := range phases.Items {
		state.currentPhase = item.Number
		state.currentWorkflowID = item.WorkflowID
		result := item.Run(ctx, state)
		if result.Status == "" {
			result = basephase.Next()
		}
		if result.Status == basephase.StatusAborted {
			if result.Err != nil {
				return result.Err
			}
			return log.CreateError(errors.ErrAborted, "phase aborted without an error")
		}
	}
	return nil
}

func buildApplyPhases(s *applyState) basephase.Items[applyState] {
	b := basephase.NewBuilder[applyState]().
		Add("apply-crds", "applying CRDs", phaseApplyCRDs).
		Add("apply-namespaces", "applying namespaces", phaseApplyNamespaces)

	if s.flags.BackupRestore && !s.flags.SkipBackupRestore {
		b = b.Add("restore-backups", "restoring backup secrets", phaseRestoreBackups)
	}

	if s.flags.DisableWebhooks {
		b = b.Add("disable-webhooks", "disabling non-ready webhook configurations", phaseDisableNonReadyWebhooks)
	}

	b = b.Add("apply-scale-zero", "applying main workload resources", phaseApplyMainWorkloadResources)

	if s.flags.ScaleUp {
		b = b.Add("scale-up-workloads", "scaling up workloads in dependency order", phaseScaleUpWorkloads)
	}

	webhookPhaseDesc := "applying webhook configurations"
	if s.flags.DisableWebhooks {
		webhookPhaseDesc = "enabling webhook configurations in provider dependency order"
	}
	b = b.Add("apply-webhooks", webhookPhaseDesc, phaseApplyWebhooks)

	if s.flags.OrphanScaleDown {
		b = b.Add("scale-down-orphans", "scaling down orphaned workloads", phaseScaleDownOrphans)
	}

	b = b.Add("delete-orphans", "deleting orphaned resources", phaseDeleteOrphans)

	return b.Build()
}

func phaseApplyCRDs(_ context.Context, s *applyState) basephase.Result {
	if s.crds.Len() == 0 {
		s.l.Info(logIdAction, s.phaseMessage("applying CRDs", true))
		return basephase.Skipped("no CRDs")
	}
	crdOps, err := s.ops.FilterCRDs()
	if err != nil {
		return basephase.Aborted(err)
	}
	if !crdOps.HasMutatingWork() {
		s.l.Info(logIdAction, s.phaseMessage("applying CRDs", true))
		return basephase.Skipped("no CRD changes")
	}
	if err := diffAndApplyCRDs(s.l, s.cc, s.cluster, crdOps, &s.flags, log.TerminalProgressUI()); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

func phaseApplyNamespaces(_ context.Context, s *applyState) basephase.Result {
	if s.namespaces.Len() == 0 {
		s.l.Info(logIdAction, s.phaseMessage("applying namespaces", true))
		return basephase.Skipped("no namespaces")
	}
	nsOps, err := s.ops.FilterNamespaces()
	if err != nil {
		return basephase.Aborted(err)
	}
	if !nsOps.HasMutatingWork() {
		s.l.Info(logIdAction, s.phaseMessage("applying namespaces", true))
		return basephase.Skipped("no namespace changes")
	}
	toApply, err := nsOps.MergeMutating()
	if err != nil {
		return basephase.Aborted(err)
	}
	s.l.Info(logIdAction, s.phaseMessage("applying {count} namespaces", false), log.Int("count", toApply.Len()))
	deleteSet := nsOps.DeleteReplaceIds()
	k8s.FlushProgressLogBeforeFooter()
	if err := k8s.Apply(context.Background(), s.l, s.cc, toApply, types.KeyTemplateEntity, s.flags.DryRun, false, deleteSet, nil, log.TerminalProgressUI()); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

func runRestoreBackups(_ context.Context, s *applyState) basephase.Result {
	restoreResults, err := backupRestoreWithOptions(
		s.cluster,
		s.appIdSlice,
		s.flags.HelmNetworkMode,
		s.flags.KubernetesVersion,
		s.flags.ForceBackupRestore,
		s.flags.Color,
		s.flags.DryRun,
		commands.BackupRestoreOptions{},
	)
	if err != nil {
		return basephase.Aborted(err)
	}
	if len(restoreResults) == 0 {
		s.l.Info(logIdAction, s.phaseMessage("restoring backup secrets", true))
		return basephase.Skipped("no backup restore candidates")
	}

	s.l.Info(logIdAction, s.phaseMessage("restoring backup secrets", false))
	printBackupResults(s.l, restoreResults, s.flags.Color)
	if commands.HasConflicts(restoreResults) {
		return basephase.Aborted(log.CreateError(
			errors.ErrAborted,
			"apply aborted: backup secrets would overwrite existing cluster secrets with different values, use 'hydra gitops backup diff' to inspect the differences, or --force-backup-restore to proceed",
		))
	}
	return basephase.Next()
}

func phaseRestoreBackups(ctx context.Context, s *applyState) basephase.Result {
	if s.restoreBackups == nil {
		return runRestoreBackups(ctx, s)
	}
	return s.restoreBackups(ctx)
}

func splitApplyWebhooks(s *applyState) (entity.Entities, entity.Entities, error) {
	return splitWebhooks(s.nonCrds)
}

func phaseDisableNonReadyWebhooks(_ context.Context, s *applyState) basephase.Result {
	webhookEntities, nonWebhookEntities, err := splitApplyWebhooks(s)
	if err != nil {
		return basephase.Aborted(err)
	}
	if err := disableNonReadyWebhooksPhase(s.l, s.cc, webhookEntities, nonWebhookEntities, s.rendered, &s.flags, s.currentPhase, s.totalPhases, s.currentWorkflowID); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

func phaseApplyMainWorkloadResources(_ context.Context, s *applyState) basephase.Result {
	phaseEntities := s.nonCrds
	skippedDesc := "applying main workload resources"
	desc := "applying {count} main resources at template scale"

	mainOps, err := s.ops.FilterMainWorkload()
	if err != nil {
		return basephase.Aborted(err)
	}

	if s.flags.DisableWebhooks {
		mainOps, err = s.ops.FilterPostNamespaceApply()
		if err != nil {
			return basephase.Aborted(err)
		}
		skippedDesc = "applying main resources and disabled webhook configurations"
		desc = "applying {count} main resources with disabled webhook configurations"
	} else {
		_, nonWebhookEntities, splitErr := splitApplyWebhooks(s)
		if splitErr != nil {
			return basephase.Aborted(splitErr)
		}
		phaseEntities = nonWebhookEntities
	}

	if phaseEntities.Len() == 0 {
		s.l.Info(logIdAction, s.phaseMessage(skippedDesc, true))
		return basephase.Skipped("no resources to apply")
	}
	if !mainOps.HasMutatingWork() {
		s.l.Info(logIdAction, s.phaseMessage(skippedDesc, true))
		return basephase.Skipped("no workload changes")
	}
	zero := s.flags.DownScaled
	if zero && s.flags.DisableWebhooks {
		desc = "applying {count} resources at scale zero (including disabled webhooks)"
	}
	if !zero && !s.flags.DisableWebhooks {
		desc = "applying {count} main resources at template scale"
	}
	toApply, err := mainOps.MergeMutating()
	if err != nil {
		return basephase.Aborted(err)
	}
	s.l.Info(logIdAction, s.phaseMessage(desc, false), log.Int("count", toApply.Len()))
	k8s.FlushProgressLogBeforeFooter()
	if err := applyWorkloadFromOps(s.l, s.cc, mainOps, &s.flags, s.scaleMap,
		zero, s.flags.DisableWebhooks, s.clusterEntities, &s.syncWindowMutationCount, log.TerminalProgressUI()); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

func phaseScaleUpWorkloads(_ context.Context, s *applyState) basephase.Result {
	if err := scaleUpPhase(s.l, s.cc, s.cluster, s.rendered, s.clusterEntities, s.refs, s.scaleMap, &s.flags, s.currentPhase, s.totalPhases, s.currentWorkflowID); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

func phaseApplyWebhooks(_ context.Context, s *applyState) basephase.Result {
	webhookEntities, _, err := splitApplyWebhooks(s)
	if err != nil {
		return basephase.Aborted(err)
	}
	if webhookEntities.Len() == 0 {
		s.l.Info(logIdAction, s.phaseMessage("applying webhook configurations", true))
		return basephase.Skipped("no webhook configurations")
	}
	whOps, err := s.ops.FilterWebhooks()
	if err != nil {
		return basephase.Aborted(err)
	}

	toApply := webhookEntities
	deleteSet := sets.Set[types.Id](nil)
	phaseDesc := "applying {count} webhook configurations"
	if !s.flags.DisableWebhooks {
		if !whOps.HasMutatingWork() {
			s.l.Info(logIdAction, s.phaseMessage("applying webhook configurations", true))
			return basephase.Skipped("no webhook changes")
		}
		toApply, err = whOps.MergeMutating()
		if err != nil {
			return basephase.Aborted(err)
		}
		patchFailures := filterPatchFailures(whOps.PatchFailures, whOps.Replace)
		deleteSet = logDeleteBeforeApplySet(s.l, s.flags.Replace, patchFailures)
	} else {
		phaseDesc = "enabling {count} webhook configurations in provider dependency order"
	}

	toApply, err = commands.PlanWebhookApplyOrder(s.l, toApply, s.rendered, s.refs, types.KeyTemplateEntity)
	if err != nil {
		return basephase.Aborted(err)
	}

	s.l.Info(logIdAction, s.phaseMessage(phaseDesc, false), log.Int("count", toApply.Len()))
	k8s.FlushProgressLogBeforeFooter()
	if err := applyEntitiesBySSA(s.l, s.cc, toApply, types.KeyTemplateEntity, s.flags.DryRun, deleteSet, log.TerminalProgressUI()); err != nil {
		return basephase.Aborted(err)
	}
	return basephase.Next()
}

type orphanScaleDownTarget struct {
	Id       types.Id
	Name     types.Name
	Ns       types.Namespace
	GVR      types.GVR
	Replicas int64
}

func isOwnedByWorkload(u unstructured.Unstructured) bool {
	workloadKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"ReplicaSet":  true,
	}
	for _, owner := range u.GetOwnerReferences() {
		if workloadKinds[owner.Kind] {
			return true
		}
	}
	return false
}

func collectOrphanScaleDownTargets(entities entity.Entities, key types.EntityKeyUnstructured) ([]orphanScaleDownTarget, error) {
	workloadGVKs := []types.GVKString{
		types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1ReplicaSet,
		types.KubernetesGvkAppsV1StatefulSet,
	}

	var targets []orphanScaleDownTarget
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}
		if !slices.Contains(workloadGVKs, gvk) {
			continue
		}
		u, err := item.UnstructuredOrError(key)
		if err != nil {
			continue
		}
		if isOwnedByWorkload(u) {
			continue
		}
		specMap, ok := u.Object["spec"].(map[string]any)
		if !ok {
			continue
		}
		replicas, ok := specMap["replicas"].(int64)
		if !ok || replicas == 0 {
			continue
		}
		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		name, err := item.Name()
		if err != nil {
			return nil, err
		}
		ns, _ := item.Namespace()
		gvr, err := item.GVR()
		if err != nil {
			return nil, err
		}
		targets = append(targets, orphanScaleDownTarget{
			Id:       id,
			Name:     name,
			Ns:       ns,
			GVR:      gvr,
			Replicas: replicas,
		})
	}
	return targets, nil
}

func phaseScaleDownOrphans(_ context.Context, s *applyState) basephase.Result {
	targets, err := collectOrphanScaleDownTargets(s.orphans, types.KeyClusterEntity)
	if err != nil {
		return basephase.Aborted(err)
	}
	if len(targets) == 0 {
		s.l.Info(logIdAction, s.phaseMessage("scaling down orphaned workloads", true))
		return basephase.Skipped("no orphaned workloads")
	}

	s.l.Info(logIdAction, s.phaseMessage("scaling down orphaned workloads", false))
	dryRunPrefix := commands.DryRunPrefix(bool(s.flags.DryRun))
	for _, tgt := range targets {
		s.l.Info(logIdAction, dryRunPrefix+"scaling down {entity} from {replicas} to {zero} replicas",
			log.String("entity", string(tgt.Id)),
			log.Int64("replicas", tgt.Replicas),
			log.Int("zero", 0))
		if s.flags.DryRun {
			continue
		}
		resourceClient := s.cc.Dynamic.Resource(tgt.GVR.K8s()).Namespace(string(tgt.Ns))
		_, err := resourceClient.Patch(
			context.Background(),
			string(tgt.Name),
			ktypes.MergePatchType,
			[]byte(`{"spec":{"replicas":0}}`),
			metav1.PatchOptions{},
		)
		if err != nil && !k8serrors.IsNotFound(err) {
			return basephase.Aborted(err)
		}
	}
	return basephase.Next()
}

func computeOrphanDeleteOrder(entities entity.Entities) ([]entity.Entity, error) {
	refs := make([]types.Ref, 0, entities.Len())
	for _, e := range entities.Items {
		ns, _ := e.Namespace()
		if ns == "" {
			continue
		}
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		refs = append(refs, types.Ref{
			RefType:      types.RefTypeDirect,
			EndpointType: types.RefEndpointTypeId,
			From:         id,
			To:           types.Id("v1/Namespace//" + string(ns)),
			Labels:       []string{types.RefLabelNamespace},
		})
	}

	graph, err := commands.BuildDependencyGraph(entities, refs)
	if err != nil {
		return nil, err
	}
	plan := commands.PlanTopologicalOrder(graph)
	slices.Reverse(plan)

	ordered := make([]entity.Entity, 0, len(plan))
	for _, entry := range plan {
		id := types.Id(entry.Name)
		if item, ok := graph.Entities[id]; ok {
			ordered = append(ordered, item)
		}
	}
	return ordered, nil
}

func phaseDeleteOrphans(_ context.Context, s *applyState) basephase.Result {
	if s.orphans.Len() == 0 {
		s.l.Info(logIdAction, s.phaseMessage("deleting orphaned resources", true))
		return basephase.Skipped("no orphaned resources")
	}

	s.l.Info(logIdAction, s.phaseMessage("deleting {count} orphaned resources", false),
		log.Int("count", s.orphans.Len()))
	orderedItems, err := computeOrphanDeleteOrder(s.orphans)
	if err != nil {
		return basephase.Aborted(err)
	}

	dryRunPrefix := commands.DryRunPrefix(bool(s.flags.DryRun))
	for _, item := range orderedItems {
		id, err := item.Id()
		if err != nil {
			return basephase.Aborted(err)
		}
		gvr, err := item.GVR()
		if err != nil {
			return basephase.Aborted(err)
		}
		ns, _ := item.Namespace()
		namespaceable := s.cc.Dynamic.Resource(gvr.K8s())
		var resourceClient dynamic.ResourceInterface
		if ns != "" {
			resourceClient = namespaceable.Namespace(string(ns))
		} else {
			resourceClient = namespaceable
		}
		name, err := item.Name()
		if err != nil {
			return basephase.Aborted(err)
		}
		u, err := item.UnstructuredOrError(types.KeyClusterEntity)
		if err != nil {
			return basephase.Aborted(err)
		}

		if metadata, ok := u.Object["metadata"].(map[string]any); ok {
			if _, hasFinalizers := metadata["finalizers"]; hasFinalizers {
				s.l.Info(logIdAction, dryRunPrefix+"removing finalizers from {entity}", log.String("entity", string(id)))
				if !s.flags.DryRun {
					_, err = resourceClient.Patch(
						context.Background(),
						string(name),
						ktypes.MergePatchType,
						[]byte(`{"metadata":{"finalizers":[]}}`),
						metav1.PatchOptions{},
					)
					if err != nil && !k8serrors.IsNotFound(err) {
						return basephase.Aborted(err)
					}
				}
			}
		}

		canDelete, err := item.VerbsContains(types.VerbDelete)
		if err == nil && !canDelete {
			continue
		}
		s.l.Info(logIdAction, dryRunPrefix+"deleting {entity}", log.String("entity", string(id)))
		if s.flags.DryRun {
			continue
		}
		propagation := metav1.DeletePropagationForeground
		err = resourceClient.Delete(context.Background(), string(name), metav1.DeleteOptions{
			PropagationPolicy: &propagation,
		})
		if err != nil && !k8serrors.IsNotFound(err) && !k8serrors.IsMethodNotSupported(err) {
			return basephase.Aborted(err)
		}
	}
	return basephase.Next()
}

func logOrphans(l log.Logger, color bool, orphans entity.Entities) error {
	if orphans.Len() == 0 {
		return nil
	}

	colorMessage := ""
	colorKey := ""
	colorReset := ""
	colorOrphan := ""
	if color {
		colorMessage = colors.LightWhite.String()
		colorKey = colors.LightMagenta.String()
		colorReset = colors.Reset.String()
		colorOrphan = colors.LightRed.String()
	}

	l.Info(logIdAction, "found {count} orphaned resources to delete:", log.Int("count", orphans.Len()))
	for _, e := range orphans.Items {
		id, err := e.Id()
		if err != nil {
			return err
		}
		fmt.Printf("%s  * %s%s%s:%s will be deleted %s☑%s\n",
			colorMessage, colorKey, string(id), colorMessage, colorOrphan, colorMessage, colorReset)
	}
	return nil
}
