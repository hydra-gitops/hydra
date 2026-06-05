package commands

import (
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/workloadclosure"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterUntrackedEntities returns cluster inventory entities that are not explained by
// (1) merged Hydra app templates (templateCatalog ids), (2) enabled global.hydra.presets
// cluster-defaults evaluation (same as uninstall warn-list preset filtering), or (3)
// Kubernetes standard ref-ownership exemptions (minimal builtins, bootstrap audit ids,
// injected namespace defaults).
//
// Only inventory roots are considered: an object with an ownerReference whose UID matches
// another object in the same inventory is treated as a child and omitted. Dangling or
// empty owner UIDs keep the object as a root candidate.
func ClusterUntrackedEntities(
	clusterEntities entity.Entities,
	templateCatalog entity.Entities,
	effective []hydra.ClusterDefaultsPresetEffective,
	presetEnv cel.Env,
	k8sMinor int,
	presetWorkloadClosure workloadclosure.MatchInput,
) (entity.Entities, error) {
	rootEntities, err := clusterEntities.ClusterInventoryRootEntities(types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, err
	}
	clusterEntities = rootEntities

	var cache *hydra.ClusterDefaultsPresetEvalCache
	if len(effective) > 0 {
		var err error
		cache, err = hydra.NewClusterDefaultsPresetEvalCache(effective, k8sMinor, presetEnv)
		if err != nil {
			return entity.Entities{}, err
		}
	}
	var out []entity.Entity
	for _, e := range clusterEntities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if templateCatalog.IdSet.Has(id) {
			continue
		}
		if IsKubernetesStandardRefOwnershipExempt(id, k8sMinor) {
			continue
		}
		if cache != nil {
			ids, err := cache.MatchingPresetIDsWithRegarding(e, presetWorkloadClosure, nil)
			if err != nil {
				return entity.Entities{}, err
			}
			if len(ids) > 0 {
				continue
			}
		}
		out = append(out, e)
	}
	return entity.NewEntities(out)
}

// namespacesFromClusterInventoryEntities returns distinct non-empty namespaces of entities that
// carry live cluster data (KeyClusterEntity), used for uninstall-safe CEL's `ns in namespaces`.
func namespacesFromClusterInventoryEntities(inventory entity.Entities) sets.Set[types.Namespace] {
	out := sets.New[types.Namespace]()
	for _, e := range inventory.Items {
		if !e.HasKey(types.KeyClusterEntity) {
			continue
		}
		ns, err := e.Namespace()
		if err != nil || ns == "" {
			continue
		}
		out.Insert(ns)
	}
	return out
}

func matchedClusterEntityIdsUninstallStyle(
	env cel.Env,
	entities entity.Entities,
	predicates []string,
) (sets.Set[types.Id], error) {
	out := sets.New[types.Id]()
	for _, raw := range predicates {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		programs, err := env.CompilePredicate("clusterEntity != null", types.CelPredicate(p))
		if err != nil {
			return nil, err
		}
		for _, e := range entities.Items {
			ok, err := programs.EvalBool(e, types.MissingKeysReject)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			id, err := e.Id()
			if err != nil {
				return nil, err
			}
			out.Insert(id)
		}
	}
	return out, nil
}

func matchedClusterEntityIdsForceStyle(
	env cel.Env,
	entities entity.Entities,
	predicates []string,
) (sets.Set[types.Id], error) {
	out := sets.New[types.Id]()
	for _, raw := range predicates {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		programs, err := env.CompilePredicate(types.CelPredicate(p))
		if err != nil {
			return nil, err
		}
		for _, e := range entities.Items {
			ok, err := programs.EvalBool(e, types.MissingKeysReject)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			id, err := e.Id()
			if err != nil {
				return nil, err
			}
			out.Insert(id)
		}
	}
	return out, nil
}

func matchedClusterEntityIdsSafeStyle(
	env cel.Env,
	entities entity.Entities,
	predicates []string,
) (sets.Set[types.Id], error) {
	out := sets.New[types.Id]()
	for _, raw := range predicates {
		p := strings.TrimSpace(raw)
		if p == "" {
			continue
		}
		programs, err := env.CompilePredicate(
			"clusterEntity != null",
			"ns in namespaces",
			types.CelPredicate(p),
		)
		if err != nil {
			return nil, err
		}
		for _, e := range entities.Items {
			ok, err := programs.EvalBool(e, types.MissingKeysReject)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			id, err := e.Id()
			if err != nil {
				return nil, err
			}
			out.Insert(id)
		}
	}
	return out, nil
}

// ExcludeEntitiesMatchingUninstallTaggedPredicates removes cluster entities whose ids match any
// enabled ref-parser predicate from a ref group tagged **uninstall**, **uninstall-force**,
// **uninstall-safe**, or **backup** (backup predicates are merged into uninstall selection during
// cluster uninstall). This aligns `hydra gitops untracked` with uninstall planning for resources
// that exist only at runtime but are still owned for teardown by Hydra ref rules.
func ExcludeEntitiesMatchingUninstallTaggedPredicates(
	cluster *hydra.Cluster,
	entities entity.Entities,
	appIds sets.Set[types.AppId],
	rendered entity.Entities,
	networkMode types.HelmNetworkMode,
	liveInventory entity.Entities,
) (entity.Entities, error) {
	if entities.Len() == 0 {
		return entities, nil
	}

	matched, err := ClusterEntityIdsMatchingUninstallTaggedPredicates(
		cluster, appIds, rendered, networkMode, liveInventory, entities)
	if err != nil {
		return entity.Entities{}, err
	}

	var kept []entity.Entity
	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if matched.Has(id) {
			continue
		}
		kept = append(kept, e)
	}
	return entity.NewEntities(kept)
}

// ExcludeUntrackedReachableFromAccountedInventoryRefs drops untracked inventory roots that appear as
// the To endpoint of a live [ClusterInventoryRefs] edge when the inventory root of From is not in the
// current untracked id set. The step repeats until stable so transitive runtime targets (for example
// PVC→PV, then PV→…) are covered generically for every ref edge emitted from the snapshot.
func ExcludeUntrackedReachableFromAccountedInventoryRefs(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	renderedAllApps entity.Entities,
	fullClusterInventory entity.Entities,
	untracked entity.Entities,
) (entity.Entities, error) {
	if untracked.Len() == 0 {
		return untracked, nil
	}
	refs, err := ClusterInventoryRefs(cluster, networkMode, renderedAllApps, fullClusterInventory)
	if err != nil {
		return entity.Entities{}, err
	}
	return pruneUntrackedByInventoryRefClosure(refs, untracked, fullClusterInventory)
}

func pruneUntrackedByInventoryRefClosure(
	refs []types.Ref,
	untracked entity.Entities,
	fullClusterInventory entity.Entities,
) (entity.Entities, error) {
	key := types.KeyClusterEntity
	uidMap := fullClusterInventory.UidMap(key)
	byID := make(map[types.Id]entity.Entity, fullClusterInventory.Len())
	for _, e := range fullClusterInventory.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		byID[id] = e
	}
	untrackedIDs := sets.New[types.Id]()
	for _, e := range untracked.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		untrackedIDs.Insert(id)
	}
	changed := true
	for changed {
		changed = false
		for _, ref := range refs {
			if ref.From == ref.To {
				continue
			}
			if !untrackedIDs.Has(ref.To) {
				continue
			}
			fromEnt, ok := byID[ref.From]
			if !ok {
				continue
			}
			rootEnt := entity.ClusterInventoryRootOf(fromEnt, key, uidMap)
			rootID, err := rootEnt.Id()
			if err != nil {
				return entity.Entities{}, err
			}
			if !untrackedIDs.Has(rootID) {
				untrackedIDs.Delete(ref.To)
				changed = true
			}
		}
	}
	var kept []entity.Entity
	for _, e := range untracked.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if untrackedIDs.Has(id) {
			kept = append(kept, e)
		}
	}
	return entity.NewEntities(kept)
}
