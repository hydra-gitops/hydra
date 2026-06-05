package hydra

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// AnnotationHydraConfig marks a ConfigMap whose data.hydra field contributes ref-parsers
// (same refs shape as global.hydra in chart values). See annotations.go.

// RefParsersFromHydraConfigMaps extracts ref-parsers from v1/ConfigMap objects that have
// annotation AnnotationHydraConfig set to a truthy value (strconv.ParseBool) and a data.hydra entry
// containing YAML with a top-level refs map (HydraRefGroup entries).
//
// universe is the full app-id set used to evaluate scope rules; selectedAppIds determines which apps
// must match scope for this CM to contribute parsers (typically the CLI-selected apps for this operation).
//
// If seen is non-nil, ConfigMaps whose resource id is already in seen are skipped; new ids are
// inserted after a ConfigMap is processed. Pass the same seen set across calls when merging
// template and live-cluster entity sets to avoid duplicate rules for the same ConfigMap.
func RefParsersFromHydraConfigMaps(
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	seen sets.Set[types.Id],
	universe sets.Set[types.AppId],
	selectedAppIds sets.Set[types.AppId],
) ([]types.RefParser, error) {
	docs, err := HydraConfigDocumentsFromEntities(entities, key)
	if err != nil {
		return nil, err
	}
	var out []types.RefParser
	for _, doc := range docs {
		if seen != nil && seen.Has(doc.Id) {
			continue
		}
		if !ScopeMatchesAnySelected(doc.Scope, universe, selectedAppIds) {
			continue
		}
		mergedFrag := types.ValuesMap{}
		if doc.Hydra["refs"] != nil {
			mergedFrag["refs"] = doc.Hydra["refs"]
		}
		refsMap := extractRefsFromMergedMap(mergedFrag)
		if len(refsMap) == 0 {
			if seen != nil {
				seen.Insert(doc.Id)
			}
			continue
		}
		parsers, err := refParsersFromHydraRefGroups(refsMap, "")
		if err != nil {
			return nil, fmt.Errorf("ConfigMap %s data.hydra refs: %w", doc.Id, err)
		}
		out = append(out, parsers...)
		if seen != nil {
			seen.Insert(doc.Id)
		}
	}
	return out, nil
}
