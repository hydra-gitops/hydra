package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// AugmentClusterScaleEntitiesForRefs returns a shallow copy of entities where live-only built-in
// workload objects in global.hydra.ownerNamespaces namespaces also carry KeyTemplateEntity
// (mirrored from live) so ref-parser predicates run for operator-created workloads that never
// appear in Helm renders. Template-only entities and non-workloads are unchanged. Used only as
// the entity input to references.Refs for cluster scale / scale status; scale mutations still use
// the un-augmented merged entity set.
func AugmentClusterScaleEntitiesForRefs(
	entities entity.Entities,
	ownerNamespaces map[types.Namespace]types.AppId,
) (entity.Entities, error) {
	if len(ownerNamespaces) == 0 || entities.Len() == 0 {
		return entities, nil
	}

	out := make([]entity.Entity, 0, entities.Len())
	for _, e := range entities.Items {
		aug, err := augmentClusterScaleEntityForRefs(e, ownerNamespaces)
		if err != nil {
			return entity.Entities{}, err
		}
		out = append(out, aug)
	}
	return entity.NewEntities(out)
}

func augmentClusterScaleEntityForRefs(
	e entity.Entity,
	ownerNamespaces map[types.Namespace]types.AppId,
) (entity.Entity, error) {
	if e.HasKey(types.KeyTemplateEntity) {
		return e, nil
	}
	if !e.HasKey(types.KeyClusterEntity) {
		return e, nil
	}
	gvk, err := e.GVKString()
	if err != nil {
		return entity.Entity{}, err
	}
	if !isClusterScaleRefAugmentWorkloadGVK(gvk) {
		return e, nil
	}
	ns, err := e.Namespace()
	if err != nil {
		return e, nil
	}
	if ns == "" {
		return e, nil
	}
	if _, owned := ownerNamespaces[ns]; !owned {
		return e, nil
	}
	u, ok := e.Unstructured(types.KeyClusterEntity)
	if !ok {
		return e, nil
	}
	dup := u.DeepCopy()
	newEnt, err := e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
		return b.WithUnstructured(types.KeyTemplateEntity, *dup)
	})
	if err != nil {
		return entity.Entity{}, err
	}
	return newEnt, nil
}

func isClusterScaleRefAugmentWorkloadGVK(gvk types.GVKString) bool {
	switch gvk {
	case types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1ReplicaSet,
		types.KubernetesGvkAppsV1StatefulSet,
		types.KubernetesGvkAppsV1DaemonSet:
		return true
	default:
		return false
	}
}
