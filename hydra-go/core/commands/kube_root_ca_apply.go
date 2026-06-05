package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// IsKubernetesAPIServerManagedKubeRootCAConfigMap reports whether id is a namespaced
// core/v1 ConfigMap named kube-root-ca.crt. The API server injects this object per
// namespace; Hydra must not apply or delete it via cluster apply.
func IsKubernetesAPIServerManagedKubeRootCAConfigMap(id types.Id) bool {
	_, ver, kind, ns, name, err := id.Components()
	if err != nil {
		return false
	}
	if ver != types.KubernetesVersionV1 {
		return false
	}
	if kind != types.KubernetesKindConfigMap || ns == "" {
		return false
	}
	return name == types.Name("kube-root-ca.crt")
}

// ExcludeKubernetesAPIServerKubeRootCAConfigMaps removes per-namespace
// ConfigMap/kube-root-ca.crt entities from the set so they are not applied.
func ExcludeKubernetesAPIServerKubeRootCAConfigMaps(entities entity.Entities) (entity.Entities, error) {
	if entities.Len() == 0 {
		return entities, nil
	}
	out := make([]entity.Entity, 0, entities.Len())
	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if IsKubernetesAPIServerManagedKubeRootCAConfigMap(id) {
			continue
		}
		out = append(out, e)
	}
	return entity.NewEntities(out)
}
