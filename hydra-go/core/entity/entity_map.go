package entity

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type EntityMap map[types.Id]Entity

func NewEntityMap(entities []Entity) (EntityMap, error) {
	result := EntityMap{}
	for _, e := range entities {
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		result[id] = e
	}
	return result, nil
}

func EntityMapIds(entityMaps ...EntityMap) sets.Set[types.Id] {
	result := sets.New[types.Id]()
	for _, entityMap := range entityMaps {
		for id := range entityMap {
			result.Insert(id)
		}
	}
	return result
}
