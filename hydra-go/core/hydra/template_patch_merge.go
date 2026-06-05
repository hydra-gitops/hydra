package hydra

import (
	"cmp"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HydraTemplatePatchRuleEntries merges global.hydra.templatePatches from Helm values and Hydra ConfigMap
// data.hydra documents using the same cluster-wide carrier catalog and scope semantics as [hydraAppMergedValuesMap].
//
// Pass renderedHydraConfigCatalog when the selected template render omits apps that still own Hydra config
// ConfigMaps needed for rule collection (e.g. Argo CD); use entity.Entities{} when the selected render
// already includes every cluster app.
func HydraTemplatePatchRuleEntries(
	cluster *Cluster,
	appIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	renderedSelected entity.Entities,
	renderedHydraConfigCatalog entity.Entities,
) ([]types.TemplatePatchRuleEntry, error) {
	allClusterAppIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return nil, err
	}

	partitionSource, err := MergeRenderedForHydraPartition(renderedSelected, renderedHydraConfigCatalog)
	if err != nil {
		return nil, err
	}

	perApp, global, err := PartitionHydraConfigDocumentsByApp(partitionSource, types.KeyTemplateEntity, allClusterAppIds)
	if err != nil {
		return nil, err
	}

	var out []types.TemplatePatchRuleEntry

	helmIds := helmChartBackedAppIds(appIds)
	appIdsSlice := helmIds.UnsortedList()
	slices.SortFunc(appIdsSlice, func(a, b types.AppId) int {
		return cmp.Compare(string(a), string(b))
	})

	for _, appId := range appIdsSlice {
		h, err := cluster.WithApp(appId)
		if err != nil {
			return nil, err
		}
		hv, err := HydraValues(h, networkMode)
		if err != nil {
			return nil, err
		}
		helmMap, err := HelmHydraMapFromValues(hv)
		if err != nil {
			return nil, err
		}
		docs := HydraConfigMapDocumentsForApp(perApp, global, allClusterAppIds, appId)
		merged := MergeHelmHydraWithConfigMapDocuments(helmMap, docs)
		entries, err := templatePatchEntriesFromMergedMap(merged, appId)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}

	return out, nil
}

// MergeRenderedForHydraPartition returns selected template entities plus catalog entities whose ids are not
// already present (so a full-cluster catalog can be merged with a subset render without duplicate ids).
func MergeRenderedForHydraPartition(selected, catalog entity.Entities) (entity.Entities, error) {
	if catalog.Len() == 0 {
		return selected, nil
	}
	seen := make(map[types.Id]struct{}, selected.Len()+catalog.Len())
	items := make([]entity.Entity, 0, selected.Len()+catalog.Len())
	for _, e := range selected.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		seen[id] = struct{}{}
		items = append(items, e)
	}
	for _, e := range catalog.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		items = append(items, e)
	}
	return entity.NewEntities(items)
}

func templatePatchEntriesFromMergedMap(merged types.ValuesMap, declaringApp types.AppId) ([]types.TemplatePatchRuleEntry, error) {
	hv, err := hydraValuesFromMergedMapLoose(merged)
	if err != nil {
		return nil, err
	}
	if hv == nil || len(hv.TemplatePatches) == 0 {
		return nil, nil
	}
	if err := types.ValidateHydraTemplatePatchRules(hv.TemplatePatches); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(hv.TemplatePatches))
	for n := range hv.TemplatePatches {
		names = append(names, n)
	}
	slices.Sort(names)
	var out []types.TemplatePatchRuleEntry
	for _, name := range names {
		rule := hv.TemplatePatches[name]
		out = append(out, types.TemplatePatchRuleEntry{
			Name:         name,
			DeclaringApp: declaringApp,
			Rule:         rule,
		})
	}
	return out, nil
}
