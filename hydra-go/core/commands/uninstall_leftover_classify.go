package commands

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/workloadclosure"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ValidateNoCrossAppDuplicateTemplateResourceIds returns an error if the same template resource id
// is associated with more than one Hydra app (flat entity list, e.g. pre-[entity.NewEntities] merge).
func ValidateNoCrossAppDuplicateTemplateResourceIds(entries []entity.Entity) error {
	idToApps := make(map[types.Id]sets.Set[types.AppId])
	for _, e := range entries {
		id, err := e.Id()
		if err != nil {
			continue
		}
		appIds, err := e.AppIds()
		if err != nil {
			continue
		}
		if idToApps[id] == nil {
			idToApps[id] = sets.New[types.AppId]()
		}
		for _, a := range appIds {
			idToApps[id].Insert(a)
		}
	}
	return duplicateTemplateResourceIdsError(idToApps)
}

func duplicateTemplateResourceIdsError(idToApps map[types.Id]sets.Set[types.AppId]) error {
	var conflicts []string
	for id, apps := range idToApps {
		if apps.Len() <= 1 {
			continue
		}
		list := apps.UnsortedList()
		slices.SortFunc(list, func(a, b types.AppId) int {
			return cmp.Compare(string(a), string(b))
		})
		conflicts = append(conflicts, fmt.Sprintf("%s (apps: %v)", id, list))
	}
	if len(conflicts) == 0 {
		return nil
	}
	slices.Sort(conflicts)
	return log.CreateError(errors.ErrUninstallDuplicateTemplateResource,
		"the same template resource id is claimed by more than one app: {detail}",
		log.String("detail", strings.Join(conflicts, "; ")))
}

// ValidateNoDuplicateTemplateResourceIds returns an error if the same resource id appears in
// more than one app's standalone template render (mutually exclusive template ownership).
func ValidateNoDuplicateTemplateResourceIds(perApp map[types.AppId]entity.Entities) error {
	idToApps := make(map[types.Id]sets.Set[types.AppId])
	for appId, ents := range perApp {
		for _, e := range ents.Items {
			id, err := e.Id()
			if err != nil {
				continue
			}
			if idToApps[id] == nil {
				idToApps[id] = sets.New[types.AppId]()
			}
			idToApps[id].Insert(appId)
		}
	}
	return duplicateTemplateResourceIdsError(idToApps)
}

// AssignSingularTemplateAppIdMetadata sets [types.KeyAppId] from the first [types.KeyAppIds] entry
// for each entity (after cross-app duplicate validation).
func AssignSingularTemplateAppIdMetadata(entities entity.Entities) (entity.Entities, error) {
	out := make([]entity.Entity, 0, entities.Len())
	for _, e := range entities.Items {
		appIds, err := e.AppIds()
		if err != nil || len(appIds) == 0 {
			out = append(out, e)
			continue
		}
		ne, err := e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithAppId(appIds[0])
		})
		if err != nil {
			return entity.Entities{}, err
		}
		out = append(out, ne)
	}
	return entity.NewEntities(out)
}

// PartitionTemplateEntitiesByPrimaryApp groups template entities by primary app id ([entity.AppId]).
func PartitionTemplateEntitiesByPrimaryApp(entities entity.Entities) (map[types.AppId]entity.Entities, error) {
	buckets := make(map[types.AppId][]entity.Entity)
	for _, e := range entities.Items {
		appId, err := e.AppId()
		if err != nil {
			return nil, err
		}
		buckets[appId] = append(buckets[appId], e)
	}
	out := make(map[types.AppId]entity.Entities, len(buckets))
	for appId, items := range buckets {
		ents, err := entity.NewEntities(items)
		if err != nil {
			return nil, err
		}
		out[appId] = ents
	}
	return out, nil
}

// soleOriginAppFromGeneratedControllerRefs returns the unique origin:app from merged inspect
// refs that target eid and declare origin:generated controller (operator/chart materialization,
// including Kyverno and similar controllers). If no such ref exists or multiple distinct apps
// are attributed, it returns false.
func soleOriginAppFromGeneratedControllerRefs(refs []types.Ref, eid types.Id) (types.AppId, bool) {
	if len(refs) == 0 {
		return "", false
	}
	seen := sets.New[types.AppId]()
	for _, r := range refs {
		if r.To != eid {
			continue
		}
		if !r.HasGeneratedValue(types.RefGeneratedController) {
			continue
		}
		for _, a := range r.Attributes {
			if a.Type == types.RefAttributeOriginApp && a.Value != "" {
				seen.Insert(types.AppId(a.Value))
			}
		}
	}
	if seen.Len() != 1 {
		return "", false
	}
	return seen.UnsortedList()[0], true
}

// ClassifyLeftoversUninstallForce partitions cluster leftovers using per-app uninstall-force
// ref predicates. A leftover matching exactly one selected app is force-deletable; matching
// exactly one non-selected app is returned in ignoredEntities; matching zero apps is reported
// via warnEntities; matching more than one app is an error.
func ClassifyLeftoversUninstallForce(
	cluster *hydra.Cluster,
	leftovers entity.Entities,
	selectedAppIds sets.Set[types.AppId],
	allAppIds sets.Set[types.AppId],
	perAppRendered map[types.AppId]entity.Entities,
	mergedInspectRefs []types.Ref,
	clusterInventory entity.Entities,
) (forceEntities, warnEntities, ignoredEntities entity.Entities, err error) {
	_ = mergedInspectRefs
	if leftovers.Len() == 0 {
		empty, err := entity.NewEntities(nil)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
		}
		return empty, empty, empty, nil
	}

	closure := workloadclosure.EmptyMatchInput(types.KeyClusterEntity)
	if cluster != nil && clusterInventory.Len() > 0 {
		cluster.ResetPreferredVersionsCache()
		pref, perr := cluster.PreferredVersions(func() (types.ScopeInfoMap, error) {
			return ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
		})
		if perr != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, perr
		}
		var cerr error
		closure, cerr = WorkloadClosureMatchInputFromInventory(cluster.L(), clusterInventory, pref)
		if cerr != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, cerr
		}
	}

	envByApp := make(map[types.AppId]cel.Env)
	linesByApp := make(map[types.AppId][]types.RefOwnershipPredicateLine)
	for appId := range allAppIds {
		rend, ok := perAppRendered[appId]
		if !ok {
			continue
		}
		env, err := cel.NewEnvWithEntityInventory(rend)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
		}
		envByApp[appId] = env
		m, err := hydra.HydraAppUninstallForcePredicateLines(
			cluster, sets.New(appId), types.HelmNetworkModeOffline, rend, true)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
		}
		if lg := m[appId]; len(lg) > 0 {
			linesByApp[appId] = lg
		}
	}

	appOrder := allAppIds.UnsortedList()
	slices.SortFunc(appOrder, func(a, b types.AppId) int { return cmp.Compare(string(a), string(b)) })

	compiledByApp, err := compileRefOwnershipPredicateLines(appOrder, linesByApp, envByApp)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
	}

	var forceItems, warnItems, ignoredItems []entity.Entity
	for _, e := range leftovers.Items {
		matching, merr := refOwnershipMatchingAppsCompiled(e, appOrder, compiledByApp, nil, closure)
		if merr != nil {
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, merr
		}
		if matching.Len() == 0 && len(mergedInspectRefs) > 0 {
			eid, idErr := e.Id()
			if idErr != nil {
				return entity.Entities{}, entity.Entities{}, entity.Entities{}, idErr
			}
			if originApp, ok := soleOriginAppFromGeneratedControllerRefs(mergedInspectRefs, eid); ok && allAppIds.Has(originApp) {
				matching.Insert(originApp)
			}
		}

		ambiguous, addForce, addWarn := uninstallLeftoverOutcome(matching, selectedAppIds)
		switch {
		case ambiguous:
			ids := matching.UnsortedList()
			slices.SortFunc(ids, func(a, b types.AppId) int { return cmp.Compare(string(a), string(b)) })
			eid, idErr := e.Id()
			if idErr != nil {
				return entity.Entities{}, entity.Entities{}, entity.Entities{}, idErr
			}
			return entity.Entities{}, entity.Entities{}, entity.Entities{}, log.CreateError(errors.ErrUninstallAmbiguousLeftoverRef,
				"cluster resource {id} matches uninstall-force rules of more than one app ({apps}); fix ref predicates so exactly one app owns it",
				log.String("id", string(eid)),
				log.String("apps", fmt.Sprintf("%v", ids)))
		case addForce:
			forceItems = append(forceItems, e)
		case addWarn:
			warnItems = append(warnItems, e)
		default:
			// Matched exactly one app that is not in the uninstall selection — ignore for this run.
			ignoredItems = append(ignoredItems, e)
		}
	}

	forceEntities, err = entity.NewEntities(forceItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
	}
	warnEntities, err = entity.NewEntities(warnItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
	}
	ignoredEntities, err = entity.NewEntities(ignoredItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, entity.Entities{}, err
	}
	return forceEntities, warnEntities, ignoredEntities, nil
}

// MergeForceLeftoversOwnedByCloneRulesIntoUninstalls moves force-classified leftovers that are
// clone-owned by exactly one selected app into the primary uninstall entity set. Ownership can come
// from global.hydra.clones-derived CEL or generated-controller inspect refs. Runtime mirror Secrets
// and similar clone targets are then deleted with the normal plan without requiring --force / --force-all.
func MergeForceLeftoversOwnedByCloneRulesIntoUninstalls(
	cluster *hydra.Cluster,
	uninstalls entity.Entities,
	forceLeftovers entity.Entities,
	selectedAppIds sets.Set[types.AppId],
	perAppRendered map[types.AppId]entity.Entities,
	renderedAllApps entity.Entities,
	clusterInventory entity.Entities,
	mergedInspectRefs []types.Ref,
) (entity.Entities, entity.Entities, error) {
	if forceLeftovers.Len() == 0 {
		return uninstalls, forceLeftovers, nil
	}
	if selectedAppIds == nil || selectedAppIds.Len() == 0 {
		return uninstalls, forceLeftovers, nil
	}

	closure := workloadclosure.EmptyMatchInput(types.KeyClusterEntity)
	if cluster != nil && clusterInventory.Len() > 0 {
		cluster.ResetPreferredVersionsCache()
		pref, perr := cluster.PreferredVersions(func() (types.ScopeInfoMap, error) {
			return ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
		})
		if perr != nil {
			return entity.Entities{}, entity.Entities{}, perr
		}
		var cerr error
		closure, cerr = WorkloadClosureMatchInputFromInventory(cluster.L(), clusterInventory, pref)
		if cerr != nil {
			return entity.Entities{}, entity.Entities{}, cerr
		}
	}

	hvMap := map[types.AppId]*types.HydraValues{}
	if cluster != nil {
		var err error
		hvMap, err = hydra.HydraAppValues(cluster, selectedAppIds, types.HelmNetworkModeOffline, renderedAllApps)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
	}

	selectedOrder := selectedAppIds.UnsortedList()
	slices.SortFunc(selectedOrder, func(a, b types.AppId) int { return cmp.Compare(string(a), string(b)) })

	envByApp := make(map[types.AppId]cel.Env)
	for _, appId := range selectedOrder {
		rend, ok := perAppRendered[appId]
		if !ok {
			continue
		}
		env, envErr := cel.NewEnvWithEntityInventory(rend)
		if envErr != nil {
			return entity.Entities{}, entity.Entities{}, envErr
		}
		envByApp[appId] = env
	}

	cloneLinesByApp := make(map[types.AppId][]types.RefOwnershipPredicateLine)
	for _, appId := range selectedOrder {
		hv := hvMap[appId]
		if hv == nil {
			continue
		}
		for _, cel := range hydra.CloneOwnershipPredicatesFromHydraValues(hv) {
			cel = strings.TrimSpace(cel)
			if cel == "" {
				continue
			}
			cloneLinesByApp[appId] = append(cloneLinesByApp[appId], types.RefOwnershipPredicateLine{Cel: cel})
		}
	}

	compiledCloneByApp, err := compileRefOwnershipPredicateLines(selectedOrder, cloneLinesByApp, envByApp)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}

	var mergeInto []entity.Entity
	var keepForce []entity.Entity
	for _, e := range forceLeftovers.Items {
		matching, merr := refOwnershipMatchingAppsCompiled(e, selectedOrder, compiledCloneByApp, nil, closure)
		if merr != nil {
			return entity.Entities{}, entity.Entities{}, merr
		}
		if matching.Len() == 1 {
			mergeInto = append(mergeInto, e)
			continue
		}
		if len(mergedInspectRefs) > 0 {
			eid, idErr := e.Id()
			if idErr != nil {
				return entity.Entities{}, entity.Entities{}, idErr
			}
			if originApp, ok := soleOriginAppFromGeneratedControllerRefs(mergedInspectRefs, eid); ok && selectedAppIds.Has(originApp) {
				mergeInto = append(mergeInto, e)
				continue
			}
		}
		keepForce = append(keepForce, e)
	}
	if len(mergeInto) == 0 {
		return uninstalls, forceLeftovers, nil
	}
	toMerge, err := entity.NewEntities(mergeInto)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	outUninstalls, err := uninstalls.Merge(toMerge, types.KeyClusterEntity)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	outForce, err := entity.NewEntities(keepForce)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return outUninstalls, outForce, nil
}

// ClusterUIDClosureFromOwnerSeeds returns the union of seedUIDs and every cluster inventory UID that is
// transitively owned by a seed via metadata.ownerReferences (owners must exist in inventory). This is
// the same closure rule as [ExpandUninstallForceWarnLeftoversOwnedBySeededUIDs], but returns the full
// protected UID set instead of splitting warn vs ignored.
func ClusterUIDClosureFromOwnerSeeds(
	inventory entity.Entities,
	seedUIDs sets.Set[types.Uid],
	onRound func(round int, protectedCount int),
) sets.Set[types.Uid] {
	key := types.KeyClusterEntity
	out := sets.New[types.Uid]()
	for uid := range seedUIDs {
		out.Insert(uid)
	}
	if out.Len() == 0 || inventory.Len() == 0 {
		return out
	}
	uidMap := inventory.UidMap(key)
	round := 0
	for {
		round++
		if onRound != nil {
			onRound(round, out.Len())
		}
		added := false
		for _, e := range inventory.Items {
			uid, ok := e.Uid(key)
			if !ok || uid == "" || out.Has(uid) {
				continue
			}
			ownerUids := e.OwnerUids(key)
			if ownerUids == nil || ownerUids.Len() == 0 {
				continue
			}
			for ou := range ownerUids {
				if !out.Has(ou) {
					continue
				}
				if _, ok := uidMap[ou]; !ok {
					continue
				}
				out.Insert(uid)
				added = true
				break
			}
		}
		if !added {
			break
		}
	}
	return out
}

// ExpandUninstallForceWarnLeftoversOwnedByIgnoredParents moves warn leftovers onto the ignored list
// when they are owned (directly or transitively via metadata.ownerReferences) by an entity already
// classified as ignored by [ClassifyLeftoversUninstallForce], limited to the same leftover inventory
// snapshot (owner UIDs must resolve within allLeftovers).
func ExpandUninstallForceWarnLeftoversOwnedByIgnoredParents(
	allLeftovers entity.Entities,
	ignored entity.Entities,
	warn entity.Entities,
) (warnOut, ignoredOut entity.Entities, err error) {
	return ExpandUninstallForceWarnLeftoversOwnedBySeededUIDs(allLeftovers, ignored, warn, nil, entity.Entities{})
}

// ownerResolveUidMap is used when deciding whether an ownerReference UID exists in the snapshot.
// allLeftovers is always included; when ownerUIDInventory is non-empty, its UIDs are merged in so
// cluster-scoped owners (for example v1/Node) resolve even when they are absent from namespaced
// leftover slices.
func ownerResolveUidMap(
	allLeftovers, ownerUIDInventory entity.Entities,
	key types.EntityKeyUnstructured,
) map[types.Uid]entity.Entity {
	out := allLeftovers.UidMap(key)
	if ownerUIDInventory.Len() == 0 {
		return out
	}
	for uid, e := range ownerUIDInventory.UidMap(key) {
		if _, ok := out[uid]; !ok {
			out[uid] = e
		}
	}
	return out
}

// ExpandUninstallForceWarnLeftoversOwnedBySeededUIDs is like [ExpandUninstallForceWarnLeftoversOwnedByIgnoredParents]
// but seeds the owner-closure with extra UIDs (for example already ignored parents referenced only
// from cluster inventory) so child Pods/ReplicaSets/etc. are not reported as uninstall-force warn leftovers.
//
// When ownerUIDInventory is non-empty, ownerReferences may resolve to UIDs present only there
// (cluster-scoped parents such as Node), which is required for mirror/static Pods in kube-system.
func ExpandUninstallForceWarnLeftoversOwnedBySeededUIDs(
	allLeftovers entity.Entities,
	ignored entity.Entities,
	warn entity.Entities,
	extraSeedUIDs sets.Set[types.Uid],
	ownerUIDInventory entity.Entities,
) (warnOut, ignoredOut entity.Entities, err error) {
	key := types.KeyClusterEntity
	if warn.Len() == 0 {
		return warn, ignored, nil
	}
	uidMap := ownerResolveUidMap(allLeftovers, ownerUIDInventory, key)
	ignoredUIDs := sets.New[types.Uid]()
	for _, e := range ignored.Items {
		if uid, ok := e.Uid(key); ok && uid != "" {
			ignoredUIDs.Insert(uid)
		}
	}
	for uid := range extraSeedUIDs {
		ignoredUIDs.Insert(uid)
	}
	if ignoredUIDs.Len() == 0 {
		return warn, ignored, nil
	}
	for {
		added := false
		for _, e := range allLeftovers.Items {
			uid, ok := e.Uid(key)
			if !ok || uid == "" || ignoredUIDs.Has(uid) {
				continue
			}
			ownerUids := e.OwnerUids(key)
			if ownerUids == nil || ownerUids.Len() == 0 {
				continue
			}
			for ou := range ownerUids {
				if !ignoredUIDs.Has(ou) {
					continue
				}
				if _, ok := uidMap[ou]; !ok {
					continue
				}
				ignoredUIDs.Insert(uid)
				added = true
				break
			}
		}
		if !added {
			break
		}
	}
	var warnKept []entity.Entity
	var extraIgnored []entity.Entity
	for _, e := range warn.Items {
		uid, ok := e.Uid(key)
		if ok && uid != "" && ignoredUIDs.Has(uid) {
			extraIgnored = append(extraIgnored, e)
			continue
		}
		warnKept = append(warnKept, e)
	}
	if len(extraIgnored) == 0 {
		return warn, ignored, nil
	}
	warnOut, err = entity.NewEntities(warnKept)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	extraEntities, err := entity.NewEntities(extraIgnored)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	ignoredOut, err = ignored.Merge(extraEntities, key)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return warnOut, ignoredOut, nil
}

// uninstallLeftoverOutcome maps a set of apps whose uninstall-force predicates matched a cluster
// entity to classification flags. ignore (none true) means a single non-selected app matched.
func uninstallLeftoverOutcome(
	matchingApps sets.Set[types.AppId],
	selectedAppIds sets.Set[types.AppId],
) (ambiguous, addForce, addWarn bool) {
	switch matchingApps.Len() {
	case 0:
		return false, false, true
	case 1:
		only := matchingApps.UnsortedList()[0]
		if selectedAppIds.Has(only) {
			return false, true, false
		}
		return false, false, false
	default:
		return true, false, false
	}
}
