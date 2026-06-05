package commands

import (
	"sort"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// MergeBuiltinPresetAppsIntoRendered folds preset-derived builtin app template entities (from
// [PresetTemplateEntities]) into the rendered template universe used by the standard ownership
// pipeline. After merging, builtin apps are indistinguishable from regular apps for every
// downstream consumer (perAppRendered, renderedAllApps, allAppIds, [TemplateAppsByNamespace],
// [templateResourceIDToApp], [AssignClusterEntitiesToAtMostOneAppByRefs]).
//
// Template primacy: explicit anchor IDs that already appear in any real app's standalone render
// are dropped from the builtin app set so the real app keeps ownership. A builtin app that ends
// up with no entities after this filter is omitted entirely.
//
// Inputs are not mutated; new maps and entity sets are returned.
func MergeBuiltinPresetAppsIntoRendered(
	perAppRendered map[types.AppId]entity.Entities,
	renderedAllApps entity.Entities,
	allAppIds sets.Set[types.AppId],
	presetEntitiesByApp map[types.AppId]entity.Entities,
) (
	mergedPerApp map[types.AppId]entity.Entities,
	mergedAllApps entity.Entities,
	mergedAppIds sets.Set[types.AppId],
	err error,
) {
	mergedPerApp = make(map[types.AppId]entity.Entities, len(perAppRendered)+len(presetEntitiesByApp))
	for appId, ents := range perAppRendered {
		mergedPerApp[appId] = ents
	}
	mergedAppIds = sets.New[types.AppId]()
	for appId := range allAppIds {
		mergedAppIds.Insert(appId)
	}

	if len(presetEntitiesByApp) == 0 {
		return mergedPerApp, renderedAllApps, mergedAppIds, nil
	}

	realTemplateIDs := sets.New[types.Id]()
	for _, ents := range perAppRendered {
		for _, e := range ents.Items {
			id, idErr := e.Id()
			if idErr != nil {
				return nil, entity.Entities{}, nil, idErr
			}
			realTemplateIDs.Insert(id)
		}
	}

	appOrder := make([]types.AppId, 0, len(presetEntitiesByApp))
	for appId := range presetEntitiesByApp {
		appOrder = append(appOrder, appId)
	}
	sort.Slice(appOrder, func(i, j int) bool { return string(appOrder[i]) < string(appOrder[j]) })

	addedItems := make([]entity.Entity, 0)
	for _, appId := range appOrder {
		// Idempotency: if this builtin app is already present in the inputs (caller already
		// merged), skip — re-merging would duplicate template entities.
		if _, exists := mergedPerApp[appId]; exists && mergedAppIds.Has(appId) {
			continue
		}
		ents := presetEntitiesByApp[appId]
		kept := make([]entity.Entity, 0, ents.Len())
		for _, e := range ents.Items {
			id, idErr := e.Id()
			if idErr != nil {
				return nil, entity.Entities{}, nil, idErr
			}
			if realTemplateIDs.Has(id) {
				continue
			}
			kept = append(kept, e)
		}
		if len(kept) == 0 {
			continue
		}
		built, bErr := entity.NewEntities(kept)
		if bErr != nil {
			return nil, entity.Entities{}, nil, bErr
		}
		mergedPerApp[appId] = built
		mergedAppIds.Insert(appId)
		addedItems = append(addedItems, kept...)
	}

	if len(addedItems) == 0 {
		return mergedPerApp, renderedAllApps, mergedAppIds, nil
	}

	allItems := make([]entity.Entity, 0, renderedAllApps.Len()+len(addedItems))
	allItems = append(allItems, renderedAllApps.Items...)
	allItems = append(allItems, addedItems...)
	mergedAllApps, err = entity.NewEntities(allItems)
	if err != nil {
		return nil, entity.Entities{}, nil, err
	}
	return mergedPerApp, mergedAllApps, mergedAppIds, nil
}
