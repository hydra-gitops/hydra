package entity

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// all predicates are ANDed together
// first false will skip remaining checks for that entity
func (entities Entities) Select(
	predicates ...func(Entity) (bool, error),
) (Entities, Entities, error) {
	all := []Entity{}
	matched := []Entity{}

	for _, e := range entities.Items {
		for _, predicate := range predicates {
			result, err := predicate(e)
			if err != nil {
				return Entities{}, Entities{}, err
			}

			if result {
				selected, err := e.Modify(func(b EntityBuilder) EntityBuilder {
					return b.WithSelected()
				})
				if err != nil {
					return Entities{}, Entities{}, err
				}
				e = selected
				matched = append(matched, e)
				break
			}
		}
		all = append(all, e)
	}

	allEntities, err := NewEntities(all)
	if err != nil {
		return Entities{}, Entities{}, err
	}

	matchedEntities, err := NewEntities(matched)
	if err != nil {
		return Entities{}, Entities{}, err
	}

	return allEntities, matchedEntities, nil
}

func (entities Entities) SelectByIds(ids ...types.Id) (Entities, Entities, error) {
	return entities.SelectByIdSet(sets.New(ids...))
}

func (entities Entities) SelectByIdSet(ids sets.Set[types.Id]) (Entities, Entities, error) {
	return entities.Select(
		func(e Entity) (bool, error) {
			id, err := e.Id()
			if err != nil {
				return false, err
			}
			return ids.Has(id), nil
		},
	)
}

func (entities Entities) SelectByContainsEntityKey(key types.EntityKey) (Entities, Entities, error) {
	return entities.Select(
		func(e Entity) (bool, error) {
			return e.HasKey(key), nil
		},
	)
}

func (entities Entities) SelectByGvk(gvk types.GVK) (Entities, Entities, error) {
	return entities.SelectByGvkString(gvk.GVKString())
}

func (entities Entities) SelectByGvkString(gvk types.GVKString) (Entities, Entities, error) {
	return entities.Select(
		func(e Entity) (bool, error) {
			entityGvk, err := e.GVKString()
			if err != nil {
				return false, err
			}
			return entityGvk == gvk, nil
		},
	)
}

func (entities Entities) SelectCrds() (Entities, Entities, error) {
	return entities.SelectByGvkString(types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition)
}

func (entities Entities) SelectNamespaces() (Entities, Entities, error) {
	return entities.SelectByGvkString(types.KubernetesGvkV1Namespace)
}

func (entities Entities) SelectByNamespaces(
	namespaces sets.Set[types.Namespace],
) (Entities, Entities, error) {
	return entities.Select(func(e Entity) (bool, error) {
		ns, err := e.Namespace()
		if err != nil {
			return false, nil
		}
		return namespaces.Has(ns), nil
	})
}

func (entities Entities) SelectByAppIds(
	appIds sets.Set[types.AppId],
) (Entities, Entities, error) {
	return entities.Select(func(e Entity) (bool, error) {
		entityAppIds, err := e.AppIds()
		if err != nil {
			return false, nil
		}
		return appIds.HasAny(entityAppIds...), nil
	})
}

// SelectByPrimaryTemplateAppId keeps entities whose primary template app id ([Entity.AppId]) is in appIds.
func (entities Entities) SelectByPrimaryTemplateAppId(
	appIds sets.Set[types.AppId],
) (Entities, Entities, error) {
	return entities.Select(func(e Entity) (bool, error) {
		appId, err := e.AppId()
		if err != nil {
			return false, nil
		}
		return appIds.Has(appId), nil
	})
}
