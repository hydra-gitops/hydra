package entity

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

func groupBy[T comparable](entities []Entity, keyFunc func(Entity) (T, error)) (map[T][]Entity, error) {
	result := map[T][]Entity{}
	for _, e := range entities {
		key, err := keyFunc(e)
		if err != nil {
			return nil, err
		}
		result[key] = append(result[key], e)
	}
	return result, nil
}

// groupById groups entities by their Id to find duplicates
// but duplicates ids are not allowed in Entities,
// because of that GroupById is not exposed publicly
func (entities Entities) groupById() (map[types.Id][]Entity, error) {
	return groupBy(entities.Items, func(e Entity) (types.Id, error) {
		return e.Id()
	})
}

func GroupBy[T comparable](entities Entities, keyFunc func(Entity) (T, error)) (map[T]Entities, error) {
	result, err := groupBy(entities.Items, keyFunc)
	if err != nil {
		return nil, err
	}

	finalResult := map[T]Entities{}
	for key, entities := range result {
		e, err := NewEntities(entities)
		if err != nil {
			return nil, err
		}
		finalResult[key] = e
	}
	return finalResult, nil
}

func (entities Entities) GroupByGVR() (map[types.GVR]Entities, error) {
	return GroupBy(entities, func(e Entity) (types.GVR, error) {
		return e.GVR()
	})
}

func (entities Entities) GroupByLabel(
	key types.EntityKeyUnstructured,
	label types.Label,
) (map[types.LabelValue]Entities, error) {
	return GroupBy(entities, func(e Entity) (types.LabelValue, error) {
		item, _ := e.Unstructured(key)
		labels := item.GetLabels()
		return types.LabelValue(labels[string(label)]), nil
	})
}

func (entities Entities) GroupByLabels(
	key types.EntityKeyUnstructured,
	labels []types.Label,
) (map[string]Entities, error) {
	if len(labels) == 0 {
		return nil, nil
	}
	grouped, err := entities.GroupByLabel(key, labels[0])
	if err != nil {
		return nil, err
	}
	result := map[string]Entities{}
	for k1, v1 := range grouped {
		if len(labels) == 1 {
			result[string(k1)] = v1
		} else {
			grouped2, err := v1.GroupByLabels(key, labels[1:])
			if err != nil {
				return nil, err
			}
			for k2, v2 := range grouped2 {
				combinedKey := fmt.Sprintf("%s/%s", k1, k2)
				result[combinedKey] = v2
			}
		}
	}
	return result, nil
}

func (entities Entities) GroupByComponentInstance(
	key types.EntityKeyUnstructured,
) (map[string]Entities, error) {
	return GroupBy(entities, func(e Entity) (string, error) {
		item, _ := e.Unstructured(key)
		labels := item.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}

		appComponent := labels["app.kubernetes.io/component"]
		appInstance := labels["app.kubernetes.io/instance"]
		appName := labels["app.kubernetes.io/name"]

		return fmt.Sprintf("%s/%s/%s", appInstance, appComponent, appName), nil
	})
}

func (entities Entities) GroupByNamespace() (map[types.Namespace]Entities, error) {
	return GroupBy(entities, func(e Entity) (types.Namespace, error) {
		namespace, _ := e.Namespace()
		return namespace, nil
	})
}

// AllOwnerUids creates a map from each entity's UID to all of its owner UIDs
// (both direct and indirect/transitive owners).
func (entities Entities) AllOwnerUids(key types.EntityKeyUnstructured) map[types.Uid]sets.Set[types.Uid] {
	return entities.allOwnerUids(key, nil)
}

// AllOwnerUidsWithProgress is like [Entities.AllOwnerUids] but calls onProgress after each inventory
// row is handled (done counts rows in Items, including rows without a UID). Heavy work may still occur
// inside one row when resolving transitive owners for that UID.
func (entities Entities) AllOwnerUidsWithProgress(key types.EntityKeyUnstructured, onProgress func(done, total int)) map[types.Uid]sets.Set[types.Uid] {
	return entities.allOwnerUids(key, onProgress)
}

func (entities Entities) allOwnerUids(key types.EntityKeyUnstructured, onProgress func(done, total int)) map[types.Uid]sets.Set[types.Uid] {
	uidMap := entities.UidMap(key)
	result := map[types.Uid]sets.Set[types.Uid]{}

	var collectAllOwners func(uid types.Uid, visited sets.Set[types.Uid]) sets.Set[types.Uid]
	collectAllOwners = func(uid types.Uid, visited sets.Set[types.Uid]) sets.Set[types.Uid] {
		if visited.Has(uid) {
			return sets.New[types.Uid]()
		}
		visited = visited.Insert(uid)

		e, exists := uidMap[uid]
		if !exists {
			return sets.New[types.Uid]()
		}

		directOwners := e.OwnerUids(key)
		if directOwners == nil || directOwners.Len() == 0 {
			return sets.New[types.Uid]()
		}

		allOwners := sets.New[types.Uid]()
		for ownerUid := range directOwners {
			allOwners = allOwners.Insert(ownerUid)
			transitiveOwners := collectAllOwners(ownerUid, visited)
			allOwners = allOwners.Union(transitiveOwners)
		}

		return allOwners
	}

	total := len(entities.Items)
	for i, e := range entities.Items {
		uid, ok := e.Uid(key)
		if !ok {
			if onProgress != nil {
				onProgress(i+1, total)
			}
			continue
		}

		result[uid] = collectAllOwners(uid, sets.New[types.Uid]())
		if onProgress != nil {
			onProgress(i+1, total)
		}
	}

	return result
}

// OrphanedEntities detects entities whose ownerReferences ALL point to UIDs
// that are not present in the entity collection.
func (entities Entities) OrphanedEntities(key types.EntityKeyUnstructured) Entities {
	uidMap := entities.UidMap(key)
	orphans := []Entity{}

	for _, e := range entities.Items {
		if _, ok := e.Unstructured(key); !ok {
			continue
		}

		ownerUids := e.OwnerUids(key)
		if ownerUids == nil || ownerUids.Len() == 0 {
			continue
		}

		allMissing := true
		for ownerUid := range ownerUids {
			if _, exists := uidMap[ownerUid]; exists {
				allMissing = false
				break
			}
		}

		if allMissing {
			orphans = append(orphans, e)
		}
	}

	result, err := NewEntities(orphans)
	if err != nil {
		return Entities{}
	}
	return result
}

// GroupByOwnerId groups entities by their root owner UID.
func (entities Entities) GroupByOwnerId(key types.EntityKeyUnstructured) (map[types.Uid]Entities, error) {
	rootOwnerMap := entities.RootOwnerUidMap(key)
	uidMap := entities.UidMap(key)
	result := map[types.Uid]Entities{}

	for rootUid, ownedUids := range rootOwnerMap {
		items := []Entity{}

		if rootEntity, exists := uidMap[rootUid]; exists {
			items = append(items, rootEntity)
		}

		for ownedUid := range ownedUids {
			if e, exists := uidMap[ownedUid]; exists {
				items = append(items, e)
			}
		}

		if len(items) > 0 {
			entitiesGroup, err := NewEntities(items)
			if err != nil {
				return nil, err
			}
			result[rootUid] = entitiesGroup
		}
	}

	return result, nil
}
