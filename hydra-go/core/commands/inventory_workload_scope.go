package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// isKubernetesClusterScopedUnionCandidate reports whether e is a cluster-scoped Kubernetes object
// that should be considered for template-id / uninstall-ref union in workload scope (excluding
// v1/Namespace, which is handled via WorkloadNamespace).
func isKubernetesClusterScopedUnionCandidate(e entity.Entity) bool {
	gvk, err := e.GVKString()
	if err != nil {
		return false
	}
	if gvk == types.KubernetesGvkV1Namespace {
		return false
	}
	nsd, err := e.Namespaced()
	if err != nil {
		return false
	}
	return nsd == types.NamespacedNo
}

// ClusterEntityIdsMatchingUninstallTaggedPredicates returns ids of entities in entitiesToScan that
// match any enabled ref-parser predicate tagged uninstall, uninstall-force, uninstall-safe, or backup.
// liveInventory is the full cluster inventory used to populate the CEL "namespaces" binding for
// uninstall-safe predicates (same as [ExcludeEntitiesMatchingUninstallTaggedPredicates]).
func ClusterEntityIdsMatchingUninstallTaggedPredicates(
	cluster *hydra.Cluster,
	appIds sets.Set[types.AppId],
	rendered entity.Entities,
	networkMode types.HelmNetworkMode,
	liveInventory entity.Entities,
	entitiesToScan entity.Entities,
) (sets.Set[types.Id], error) {
	if entitiesToScan.Len() == 0 {
		return sets.New[types.Id](), nil
	}

	hydra.WarnDuplicateBackupUninstallTags(cluster, appIds, networkMode, rendered)

	env, err := cel.NewEnvWithEntityInventory(rendered)
	if err != nil {
		return nil, err
	}

	uninstallPreds, err := hydra.HydraAppUninstallPredicates(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}
	backupPreds, err := hydra.HydraAppBackupPredicates(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}
	uninstallPreds = append(uninstallPreds, backupPreds...)

	forcePreds, err := hydra.HydraAppUninstallForcePredicates(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}

	safePreds, err := hydra.HydraAppUninstallSafePredicates(cluster, appIds, networkMode, rendered)
	if err != nil {
		return nil, err
	}

	uninstallMatched, err := matchedClusterEntityIdsUninstallStyle(env, entitiesToScan, uninstallPreds)
	if err != nil {
		return nil, err
	}
	forceMatched, err := matchedClusterEntityIdsForceStyle(env, entitiesToScan, forcePreds)
	if err != nil {
		return nil, err
	}

	nsSet := namespacesFromClusterInventoryEntities(liveInventory)
	envSafe, err := cel.NewEnvWithEntityInventoryOverlay(rendered, liveInventory, cel.SetSupport("namespaces", nsSet))
	if err != nil {
		return nil, err
	}
	safeMatched, err := matchedClusterEntityIdsSafeStyle(envSafe, entitiesToScan, safePreds)
	if err != nil {
		return nil, err
	}

	matched := sets.New[types.Id]()
	for id := range uninstallMatched {
		matched.Insert(id)
	}
	for id := range forceMatched {
		matched.Insert(id)
	}
	for id := range safeMatched {
		matched.Insert(id)
	}
	return matched, nil
}

// LiveEntitiesInHydraWorkloadScope returns the union of live entities that are either (1) in one of
// the given workload namespaces ([WorkloadNamespace]), or (2) cluster-scoped with a template id in
// templateCatalog, or (3) cluster-scoped and matching uninstall-family ref predicates.
// The second return value is the set of cluster-scoped ids that matched uninstall-family predicates
// (subset of the union); it is empty when fullLive is empty or no such matches exist.
func LiveEntitiesInHydraWorkloadScope(
	fullLive entity.Entities,
	namespaces sets.Set[types.Namespace],
	templateCatalog entity.Entities,
	cluster *hydra.Cluster,
	predicateAppIds sets.Set[types.AppId],
	renderedForPredicates entity.Entities,
	networkMode types.HelmNetworkMode,
) (entity.Entities, sets.Set[types.Id], error) {
	emptyPreds := sets.New[types.Id]()
	if fullLive.Len() == 0 {
		return fullLive, emptyPreds, nil
	}
	templateIds := templateCatalog.IdSet

	var clusterScoped []entity.Entity
	for _, e := range fullLive.Items {
		if isKubernetesClusterScopedUnionCandidate(e) {
			clusterScoped = append(clusterScoped, e)
		}
	}
	clusterScopedEnts, err := entity.NewEntities(clusterScoped)
	if err != nil {
		return entity.Entities{}, nil, err
	}

	predIds, err := ClusterEntityIdsMatchingUninstallTaggedPredicates(
		cluster, predicateAppIds, renderedForPredicates, networkMode, fullLive, clusterScopedEnts)
	if err != nil {
		return entity.Entities{}, nil, err
	}

	var out []entity.Entity
	seen := sets.New[types.Id]()
	for _, e := range fullLive.Items {
		if wn, ok := WorkloadNamespace(e); ok && namespaces.Has(wn) {
			id, err := e.Id()
			if err != nil {
				return entity.Entities{}, nil, err
			}
			if seen.Has(id) {
				continue
			}
			seen.Insert(id)
			out = append(out, e)
			continue
		}
		if !isKubernetesClusterScopedUnionCandidate(e) {
			continue
		}
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, nil, err
		}
		if !templateIds.Has(id) && !predIds.Has(id) {
			continue
		}
		if seen.Has(id) {
			continue
		}
		seen.Insert(id)
		out = append(out, e)
	}
	ents, err := entity.NewEntities(out)
	if err != nil {
		return entity.Entities{}, nil, err
	}
	return ents, predIds, nil
}
