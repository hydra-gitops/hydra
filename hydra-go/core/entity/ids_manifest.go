package entity

import (
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// IdsFromManifest returns the set of entity IDs in a rendered manifest (after internal deduplication).
func IdsFromManifest(l log.Logger, manifest types.YamlString, key types.EntityKeyUnstructured) (sets.Set[types.Id], error) {
	e, err := NewEntitiesFromYaml(l, manifest, key)
	if err != nil {
		return nil, err
	}
	out := sets.New[types.Id]()
	for _, it := range e.Items {
		id, err := it.Id()
		if err != nil {
			continue
		}
		out.Insert(id)
	}
	return out, nil
}
