package commands

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateClusterApplyPostPlanRefs ensures that after the planned apply completes, every reference
// from the rendered selection would resolve (targets exist and referenced secret/configmap keys exist).
// Pass live cluster inventory and orphans scheduled for deletion so the simulated post-apply state
// matches resources that would remain plus everything in the rendered apply set.
// backupRestoreSecretIDs adds v1/Secret ids that integrated backup restore would write before main
// workload apply (existence only; referenced keys inside those secrets are not read from backup files).
func ValidateClusterApplyPostPlanRefs(
	l log.Logger,
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	appIds sets.Set[types.AppId],
	rendered entity.Entities,
	clusterEntities entity.Entities,
	orphans entity.Entities,
	sourceRefs []types.Ref,
	backupRestoreSecretIDs sets.Set[types.Id],
) error {
	postApplyInventory, err := materializePostApplyInventory(rendered, clusterEntities, orphans)
	if err != nil {
		return err
	}

	syntheticNs, err := BuildSyntheticNamespaceDefaultTargets(l, postApplyInventory, types.KeyTemplateEntity)
	if err != nil {
		return err
	}
	if syntheticNs.Len() > 0 {
		postApplyInventory, err = postApplyInventory.Append(syntheticNs)
		if err != nil {
			return err
		}
	}

	preferredVersions, err := cluster.PreferredVersions(nil)
	if err != nil {
		return err
	}

	appRefParsers, err := hydra.HydraAppRefParsers(cluster, appIds, networkMode, rendered)
	if err != nil {
		return err
	}

	refs, err := augmentRefsWithExplicitKeyAttributes(rendered, sourceRefs)
	if err != nil {
		return err
	}

	expandedTargets, targetRefs, err := references.ResolveVirtualRefs(
		l,
		postApplyInventory,
		types.KeyTemplateEntity,
		nil,
		entity.Entities{},
		preferredVersions,
		appRefParsers,
	)
	if err != nil {
		return err
	}

	targetKeys, generatedTargets, err := reviewTargetKeys(expandedTargets, types.KeyTemplateEntity, targetRefs)
	if err != nil {
		return err
	}

	allTargetIds := sets.New[types.Id]()
	for _, item := range expandedTargets.Items {
		id, err := item.Id()
		if err != nil {
			return err
		}
		allTargetIds.Insert(id)
	}

	existenceTargetIds := allTargetIds
	if backupRestoreSecretIDs != nil && backupRestoreSecretIDs.Len() > 0 {
		existenceTargetIds = allTargetIds.Union(backupRestoreSecretIDs)
	}

	selectedSourceIds := sets.New[types.Id]()
	for _, item := range rendered.Items {
		id, err := item.Id()
		if err != nil {
			return err
		}
		appIdsFromEnt, err := item.AppIds()
		if err == nil && appIds.HasAny(appIdsFromEnt...) {
			selectedSourceIds.Insert(id)
		}
	}

	groupedSources := map[reviewFindingKey]sets.Set[types.Id]{}
	addFinding := func(target types.Id, message string, source types.Id) {
		key := reviewFindingKey{Target: target, Message: message}
		if groupedSources[key] == nil {
			groupedSources[key] = sets.New[types.Id]()
		}
		groupedSources[key].Insert(source)
	}

	for _, ref := range refs {
		if !selectedSourceIds.Has(ref.From) {
			continue
		}
		if shouldSkipReviewRef(ref) {
			continue
		}

		targetExists := existenceTargetIds.Has(ref.To) || generatedTargets.Has(ref.To)
		if !targetExists {
			if hasIncomingTargetSuppressionRef(refs, targetRefs, ref.To) {
				continue
			}
			addFinding(ref.To, missingTargetResourceFinding, ref.From)
			continue
		}

		keys := refAttributesByType(ref.Attributes, "key")
		if len(keys) == 0 {
			continue
		}

		availableKeys, ok := targetKeys[ref.To]
		if !ok {
			continue
		}
		for _, key := range keys {
			if !availableKeys.Has(key) {
				addFinding(ref.To, fmt.Sprintf("missing referenced key %q", key), ref.From)
			}
		}
	}

	findings := make([]ReviewFinding, 0, len(groupedSources))
	for key, sources := range groupedSources {
		sourceList := sources.UnsortedList()
		slices.SortFunc(sourceList, func(a, b types.Id) int {
			return cmp.Compare(string(a), string(b))
		})
		findings = append(findings, ReviewFinding{
			Target:  key.Target,
			Message: key.Message,
			Sources: sourceList,
		})
	}

	slices.SortFunc(findings, func(a, b ReviewFinding) int {
		if c := cmp.Compare(a.Target, b.Target); c != 0 {
			return c
		}
		return cmp.Compare(a.Message, b.Message)
	})

	if len(findings) == 0 {
		return nil
	}

	var b strings.Builder
	b.WriteString("apply aborted: after the planned changes, references would still be unresolved:\n")
	for _, f := range findings {
		src := strings.Join(stringIds(f.Sources), ", ")
		fmt.Fprintf(&b, "  - %s: %s (from sources: %s)\n", f.Target, f.Message, src)
	}
	b.WriteString("Use --bootstrap for initial cluster setup (for example SOPS secret materialization) or --skip-ref-checks to proceed anyway.")
	return log.CreateError(errors.ErrAborted, strings.TrimSuffix(b.String(), "\n"))
}

func stringIds(ids []types.Id) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func materializePostApplyInventory(
	rendered entity.Entities,
	clusterEntities entity.Entities,
	orphans entity.Entities,
) (entity.Entities, error) {
	orphanIDs := sets.New[types.Id]()
	for _, e := range orphans.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		orphanIDs.Insert(id)
	}

	byID := make(map[types.Id]entity.Entity)

	for _, e := range clusterEntities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if orphanIDs.Has(id) {
			continue
		}
		u, ok := e.Unstructured(types.KeyClusterEntity)
		if !ok {
			continue
		}
		mod, err := e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(types.KeyTemplateEntity, u)
		})
		if err != nil {
			return entity.Entities{}, err
		}
		byID[id] = mod
	}

	for _, e := range rendered.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		byID[id] = e
	}

	ids := make([]types.Id, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	slices.SortFunc(ids, func(a, b types.Id) int {
		return cmp.Compare(string(a), string(b))
	})

	items := make([]entity.Entity, 0, len(ids))
	for _, id := range ids {
		items = append(items, byID[id])
	}

	return entity.NewEntities(items)
}
