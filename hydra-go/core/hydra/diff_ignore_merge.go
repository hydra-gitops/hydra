package hydra

import (
	"cmp"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// BuiltinDiffIgnoreRuleEntries returns built-in rules applied before any user-defined global.hydra.diff.ignore entries.
// The workload replica rule matches hydra gitops apply's historical behavior: ignore pure replica drift for core workloads.
func BuiltinDiffIgnoreRuleEntries() []types.DiffIgnoreRuleEntry {
	return []types.DiffIgnoreRuleEntry{
		{
			Name:         "_hydra_builtin_workload_replicas",
			DeclaringApp: "",
			Rule: types.HydraDiffIgnoreRule{
				Predicate: `gvk == "apps/v1/Deployment" || gvk == "apps/v1/StatefulSet" || gvk == "apps/v1/ReplicaSet"`,
				Patches: []types.HydraDiffYqPatch{
					{Yq: `del(.spec.replicas)`},
				},
			},
		},
	}
}

// HydraDiffIgnoreRuleEntries merges built-in rules with global.hydra.diff.ignore from Helm values and Hydra ConfigMap
// data.hydra documents (same merge semantics as refs/clones).
func HydraDiffIgnoreRuleEntries(
	cluster *Cluster,
	appIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	rendered entity.Entities,
) ([]types.DiffIgnoreRuleEntry, error) {
	helmIds := helmChartBackedAppIds(appIds)
	perApp, global, err := PartitionHydraConfigDocumentsByApp(rendered, types.KeyTemplateEntity, helmIds)
	if err != nil {
		return nil, err
	}

	var out []types.DiffIgnoreRuleEntry
	out = append(out, BuiltinDiffIgnoreRuleEntries()...)

	if helmIds.Len() == 0 {
		return out, nil
	}

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
		docs := HydraConfigMapDocumentsForApp(perApp, global, appIds, appId)
		merged := MergeHelmHydraWithConfigMapDocuments(helmMap, docs)
		entries, err := diffIgnoreEntriesFromMergedMap(merged, appId)
		if err != nil {
			return nil, err
		}
		out = append(out, entries...)
	}

	return out, nil
}

func diffIgnoreEntriesFromMergedMap(merged types.ValuesMap, declaringApp types.AppId) ([]types.DiffIgnoreRuleEntry, error) {
	hv, err := hydraValuesFromMergedMapLoose(merged)
	if err != nil {
		return nil, err
	}
	if hv == nil || hv.Diff == nil || len(hv.Diff.Ignore) == 0 {
		return nil, nil
	}
	names := make([]string, 0, len(hv.Diff.Ignore))
	for n := range hv.Diff.Ignore {
		names = append(names, n)
	}
	slices.Sort(names)
	var out []types.DiffIgnoreRuleEntry
	for _, name := range names {
		rule := hv.Diff.Ignore[name]
		out = append(out, types.DiffIgnoreRuleEntry{
			Name:         name,
			DeclaringApp: declaringApp,
			Rule:         rule,
		})
	}
	return out, nil
}
