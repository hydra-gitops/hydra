package commands

import (
	"cmp"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// TreeCandidateIdsLocal returns all resource ids known from rendered templates and the ref graph
// (same universe as hydra local tree with an explicit id).
func TreeCandidateIdsLocal(cluster *hydra.Cluster, networkMode types.HelmNetworkMode) ([]types.Id, error) {
	refs, ents, err := clusterRefsAllFromTemplates(cluster, networkMode)
	if err != nil {
		return nil, err
	}
	return sortIdSet(knownResourceIds(ents, refs)), nil
}

// TreeCandidateIdsCluster returns all resource ids from templates, the ref graph, and the live cluster inventory.
func TreeCandidateIdsCluster(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
) ([]types.Id, error) {
	g, err := LoadClusterTreeGraph(cluster, networkMode, bootstrap)
	if err != nil {
		return nil, err
	}
	return g.CandidateIds(), nil
}

func sortIdSet(s sets.Set[types.Id]) []types.Id {
	out := s.UnsortedList()
	slices.SortFunc(out, func(a, b types.Id) int {
		return cmp.Compare(string(a), string(b))
	})
	return out
}

// PickerRowStatusLocalTemplateOnly is the status label for each id in the local tree id picker.
// Candidates come only from rendered templates (no live cluster inventory), so every row is "missing"
// relative to a template+cluster merge (same vocabulary as the tree Status column).
func PickerRowStatusLocalTemplateOnly(ids []types.Id) map[types.Id]string {
	m := make(map[types.Id]string, len(ids))
	for _, id := range ids {
		m[id] = "missing"
	}
	return m
}
