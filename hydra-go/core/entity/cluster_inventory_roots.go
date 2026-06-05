package entity

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterInventoryRootEntities returns entities that are roots relative to the same inventory:
// an object is omitted when it has at least one ownerReference with a non-empty UID that matches
// another object in entities (by UID). Objects with no ownerReferences, only empty UIDs, or only
// references to UIDs absent from entities are kept — invalid or dangling owner links count as roots.
func (entities Entities) ClusterInventoryRootEntities(key types.EntityKeyUnstructured) (Entities, error) {
	return entities.ClusterInventoryEntitiesExcludingOwnedChildren(key, entities.UidMap(key))
}

// ClusterInventoryEntitiesExcludingOwnedChildren returns candidates that are not children relative
// to ownerUidMap: an object is omitted when it has at least one ownerReference with a non-empty UID
// that exists in ownerUidMap. Pass UidMap(fullInventory) so "valid" means the owner object is present
// in that snapshot (not only among candidates).
func (entities Entities) ClusterInventoryEntitiesExcludingOwnedChildren(
	key types.EntityKeyUnstructured,
	ownerUidMap map[types.Uid]Entity,
) (Entities, error) {
	var out []Entity
	for _, e := range entities.Items {
		u, ok := e.Unstructured(key)
		if !ok {
			out = append(out, e)
			continue
		}
		if clusterEntityChildOfInventoryParent(&u, ownerUidMap) {
			continue
		}
		out = append(out, e)
	}
	return NewEntities(out)
}

func clusterEntityChildOfInventoryParent(u *unstructured.Unstructured, uidMap map[types.Uid]Entity) bool {
	for _, ref := range u.GetOwnerReferences() {
		if ref.UID == "" {
			continue
		}
		if _, exists := uidMap[types.Uid(ref.UID)]; exists {
			return true
		}
	}
	return false
}

// ClusterInventoryRootOf walks ownerReferences within the same snapshot (uidMap) until it reaches an
// entity with no owning parent present in uidMap. It prefers a controller ownerRef when multiple
// parents exist in the snapshot. Cycles or missing UIDs stop the walk at the last stable entity.
func ClusterInventoryRootOf(e Entity, key types.EntityKeyUnstructured, uidMap map[types.Uid]Entity) Entity {
	seen := sets.New[types.Uid]()
	current := e
	for {
		uid, ok := current.Uid(key)
		if !ok || uid == "" {
			return current
		}
		if seen.Has(uid) {
			return current
		}
		seen.Insert(uid)
		u, ok := current.Unstructured(key)
		if !ok {
			return current
		}
		var controllerParent Entity
		controllerFound := false
		var anyParent Entity
		anyFound := false
		for _, owner := range u.GetOwnerReferences() {
			if owner.UID == "" {
				continue
			}
			ouid := types.Uid(owner.UID)
			parent, exists := uidMap[ouid]
			if !exists {
				continue
			}
			if owner.Controller != nil && *owner.Controller {
				controllerParent = parent
				controllerFound = true
				break
			}
			if !anyFound {
				anyParent = parent
				anyFound = true
			}
		}
		if controllerFound {
			current = controllerParent
			continue
		}
		if anyFound {
			current = anyParent
			continue
		}
		return current
	}
}
