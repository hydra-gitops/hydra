package hydra

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// CloneRulesFromHydraConfigMaps extracts clone rules from v1/ConfigMap objects that have
// annotation AnnotationHydraConfig set to a truthy value and a data.hydra entry
// containing YAML with a top-level clones map (same shape as global.hydra.clones in chart values).
//
// universe and selectedAppIds gate ConfigMaps with a scope the same way as [RefParsersFromHydraConfigMaps].
//
// If seen is non-nil, ConfigMaps whose resource id is already in seen are skipped; new ids are
// inserted after a ConfigMap is processed. Pass the same seen set across calls when merging
// template and live-cluster entity sets to avoid duplicate rules for the same ConfigMap.
func CloneRulesFromHydraConfigMaps(
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	seen sets.Set[types.Id],
	universe sets.Set[types.AppId],
	selectedAppIds sets.Set[types.AppId],
) ([]types.HydraCloneRuleEntry, error) {
	docs, err := HydraConfigDocumentsFromEntities(entities, key)
	if err != nil {
		return nil, err
	}
	var out []types.HydraCloneRuleEntry
	for _, doc := range docs {
		if seen != nil && seen.Has(doc.Id) {
			continue
		}
		if !ScopeMatchesAnySelected(doc.Scope, universe, selectedAppIds) {
			continue
		}
		mergedFrag := types.ValuesMap{}
		if doc.Hydra["clones"] != nil {
			mergedFrag["clones"] = doc.Hydra["clones"]
		}
		clonesMap := extractClonesFromMergedMap(mergedFrag)
		if len(clonesMap) == 0 {
			if seen != nil {
				seen.Insert(doc.Id)
			}
			continue
		}
		entries, err := cloneRulesFromHydraCloneMap(clonesMap, "")
		if err != nil {
			return nil, fmt.Errorf("ConfigMap %s data.hydra clones: %w", doc.Id, err)
		}
		out = append(out, entries...)
		if seen != nil {
			seen.Insert(doc.Id)
		}
	}
	return out, nil
}
