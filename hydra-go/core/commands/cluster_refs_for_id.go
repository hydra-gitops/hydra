package commands

import (
	"slices"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

const transitiveRefsMaxDistance = 10

// TransitiveRefEntry is the signed-distance reachability model used by tests and shared traversal logic.
type TransitiveRefEntry struct {
	ID       types.Id `yaml:"id"`
	Distance int      `yaml:"distance"`
}

// TransitiveRefRow is the YAML output model for hydra local refs / hydra gitops refs.
type TransitiveRefRow struct {
	ID        types.Id `yaml:"id"`
	Distance  int      `yaml:"distance"`
	Direction string   `yaml:"direction,omitempty"`
	Labels    []string `yaml:"labels,omitempty"`
	Desc      string   `yaml:"desc,omitempty"`
	Reverse   bool     `yaml:"reverse,omitempty"`
}

// TransitiveRefDetail carries one reachable id, its signed distance, and the merged last-hop ref metadata.
type TransitiveRefDetail struct {
	ID       types.Id
	Distance int
	Ref      types.Ref
}

type logicalRefEdge struct {
	id  types.Id
	ref types.Ref
}

// ClusterRefsTransitive returns the transitive local ref view for id using template-only refs.
func ClusterRefsTransitive(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	id types.Id,
) ([]TransitiveRefRow, error) {
	refs, sourceEntities, err := clusterRefsAllFromTemplates(cluster, networkMode)
	if err != nil {
		return nil, err
	}

	if err := ensureResourceIdKnown(sourceEntities, refs, id); err != nil {
		return nil, err
	}

	return TransitiveRefRowsFromId(refs, id), nil
}

// ClusterRefsTransitiveWithCluster returns the transitive cluster ref view for id using template+cluster refs.
func ClusterRefsTransitiveWithCluster(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	bootstrap types.Bootstrap,
	id types.Id,
) ([]TransitiveRefRow, error) {
	refs, templateEntities, clusterEntities, err := clusterRefsAllClusterTreeWithEntities(cluster, networkMode, bootstrap, nil)
	if err != nil {
		return nil, err
	}

	if err := ensureResourceIdKnownWithClusterEntities(templateEntities, clusterEntities, refs, id); err != nil {
		return nil, err
	}

	return TransitiveRefRowsFromId(refs, id), nil
}

// TransitiveRefsFromId returns the anchor id at distance 0 plus outgoing and incoming BFS reachability.
func TransitiveRefsFromId(refs []types.Ref, start types.Id) []TransitiveRefEntry {
	if start == "" {
		return nil
	}
	incoming, outgoing := TransitiveRefDetailsForID(refs, start)

	out := []TransitiveRefEntry{{ID: start, Distance: 0}}
	for _, detail := range outgoing {
		out = append(out, TransitiveRefEntry{
			ID:       detail.ID,
			Distance: detail.Distance,
		})
	}
	for _, detail := range incoming {
		out = append(out, TransitiveRefEntry{
			ID:       detail.ID,
			Distance: detail.Distance,
		})
	}
	return out
}

// TransitiveEdgesForID partitions transitive reachability into negative-distance incoming and positive-distance outgoing ids.
func TransitiveEdgesForID(refs []types.Ref, id types.Id) (incoming, outgoing []TransitiveRefEntry) {
	incomingDetails, outgoingDetails := TransitiveRefDetailsForID(refs, id)
	for _, detail := range incomingDetails {
		incoming = append(incoming, TransitiveRefEntry{ID: detail.ID, Distance: detail.Distance})
	}
	for _, detail := range outgoingDetails {
		outgoing = append(outgoing, TransitiveRefEntry{ID: detail.ID, Distance: detail.Distance})
	}
	return incoming, outgoing
}

// TransitiveRefRowsFromId builds the YAML output rows used by hydra local refs / hydra gitops refs.
func TransitiveRefRowsFromId(refs []types.Ref, start types.Id) []TransitiveRefRow {
	if start == "" {
		return nil
	}
	incoming, outgoing := TransitiveRefDetailsForID(refs, start)

	out := []TransitiveRefRow{{ID: start, Distance: 0}}
	for _, detail := range outgoing {
		out = append(out, transitiveRefRow(detail, "outgoing"))
	}
	for _, detail := range incoming {
		out = append(out, transitiveRefRow(detail, "incoming"))
	}
	return out
}

// TransitiveRefDetailsForID computes transitive reachability metadata for one id.
func TransitiveRefDetailsForID(refs []types.Ref, start types.Id) (incoming, outgoing []TransitiveRefDetail) {
	if start == "" {
		return nil, nil
	}
	incomingAdj, outgoingAdj := logicalRefAdjacency(refs)
	return transitiveRefFrontier(incomingAdj, start, -1), transitiveRefFrontier(outgoingAdj, start, 1)
}

func transitiveRefRow(detail TransitiveRefDetail, direction string) TransitiveRefRow {
	row := TransitiveRefRow{
		ID:        detail.ID,
		Distance:  detail.Distance,
		Direction: direction,
		Reverse:   detail.Ref.Reverse,
	}
	if len(detail.Ref.Labels) > 0 {
		row.Labels = slices.Clone(detail.Ref.Labels)
	}
	if detail.Ref.Desc != "" {
		row.Desc = detail.Ref.Desc
	}
	return row
}

func logicalRefAdjacency(refs []types.Ref) (incoming, outgoing map[types.Id][]logicalRefEdge) {
	incoming = make(map[types.Id][]logicalRefEdge)
	outgoing = make(map[types.Id][]logicalRefEdge)

	for _, ref := range refs {
		from := ref.From
		to := ref.To
		if ref.Reverse {
			from, to = to, from
		}
		if from == "" || to == "" || from == to {
			continue
		}
		outgoing[from] = append(outgoing[from], logicalRefEdge{id: to, ref: ref})
		incoming[to] = append(incoming[to], logicalRefEdge{id: from, ref: ref})
	}

	return incoming, outgoing
}

func transitiveRefFrontier(adjacency map[types.Id][]logicalRefEdge, start types.Id, sign int) []TransitiveRefDetail {
	visited := sets.New(start)
	current := nextTransitiveLevel(adjacency[start], visited)
	var out []TransitiveRefDetail

	for level := 1; level <= transitiveRefsMaxDistance; level++ {
		if len(current) == 0 {
			break
		}

		ids := mapsKeys(current)
		slices.Sort(ids)
		for _, id := range ids {
			merged := references.MergeRefLists(current[id])
			var ref types.Ref
			if len(merged) > 0 {
				ref = merged[0]
			}
			out = append(out, TransitiveRefDetail{
				ID:       id,
				Distance: sign * level,
				Ref:      ref,
			})
		}
		for _, id := range ids {
			visited.Insert(id)
		}

		next := map[types.Id][]types.Ref{}
		for _, id := range ids {
			for _, edge := range adjacency[id] {
				if visited.Has(edge.id) {
					continue
				}
				next[edge.id] = append(next[edge.id], edge.ref)
			}
		}
		current = next
	}

	return out
}

func nextTransitiveLevel(edges []logicalRefEdge, visited sets.Set[types.Id]) map[types.Id][]types.Ref {
	level := map[types.Id][]types.Ref{}
	for _, edge := range edges {
		if visited.Has(edge.id) {
			continue
		}
		level[edge.id] = append(level[edge.id], edge.ref)
	}
	return level
}

func mapsKeys[V any](m map[types.Id]V) []types.Id {
	keys := make([]types.Id, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	return keys
}

// clusterRefsAllFromTemplates renders all enabled apps and returns the full reference graph
// from templates only (no live cluster), with explicit key attribute enrichment applied.
func clusterRefsAllFromTemplates(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
) ([]types.Ref, entity.Entities, error) {
	l := cluster.L()
	appIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return nil, entity.Entities{}, err
	}
	if len(appIds) == 0 {
		return nil, entity.Entities{}, nil
	}

	cluster.ResetPreferredVersionsCache()
	preferredVersions, err := cluster.PreferredVersions(func() (types.ScopeInfoMap, error) {
		return ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
	})
	if err != nil {
		return nil, entity.Entities{}, err
	}

	sourceEntities, err := RenderClusterSelectedApps(cluster, networkMode, "", appIds, types.KeyTemplateEntity)
	if err != nil {
		return nil, entity.Entities{}, err
	}

	parsers, err := hydra.HydraAppRefParsers(cluster, appIds, networkMode, sourceEntities)
	if err != nil {
		return nil, entity.Entities{}, err
	}

	refs, err := references.Refs(l, sourceEntities, types.KeyTemplateEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, parsers)
	if err != nil {
		return nil, entity.Entities{}, err
	}
	refs = references.AnnotateRefsWithSource(refs, types.RefSourceTemplate)

	refs, err = augmentRefsWithExplicitKeyAttributes(sourceEntities, refs)
	if err != nil {
		return nil, entity.Entities{}, err
	}
	refs = references.EnsureRefsHaveOriginSource(refs, types.RefSourceTemplate)
	return refs, sourceEntities, nil
}

func ensureResourceIdKnown(entities entity.Entities, refs []types.Ref, id types.Id) error {
	if id == "" {
		return nil
	}
	known := knownResourceIds(entities, refs)
	if known.Has(id) {
		return nil
	}
	return log.CreateError(errors.ErrHydraConfigError,
		"unknown resource id {id}: not present in rendered manifests or ref graph",
		log.String("id", string(id)))
}

// ensureResourceIdKnownWithClusterEntities is like ensureResourceIdKnown but also treats
// ids present in the live cluster inventory (for example Pods created by operators) as known.
func ensureResourceIdKnownWithClusterEntities(
	templateEntities entity.Entities,
	clusterEntities entity.Entities,
	refs []types.Ref,
	id types.Id,
) error {
	if id == "" {
		return nil
	}
	known := knownResourceIds(templateEntities, refs)
	for _, e := range clusterEntities.Items {
		eid, err := e.Id()
		if err != nil {
			continue
		}
		known.Insert(eid)
	}
	if known.Has(id) {
		return nil
	}
	return log.CreateError(errors.ErrHydraConfigError,
		"unknown resource id {id}: not present in rendered manifests, live cluster inventory, or ref graph",
		log.String("id", string(id)))
}

func knownResourceIds(entities entity.Entities, refs []types.Ref) sets.Set[types.Id] {
	seen := sets.New[types.Id]()
	for _, e := range entities.Items {
		eid, err := e.Id()
		if err != nil {
			continue
		}
		seen.Insert(eid)
	}
	for _, r := range refs {
		seen.Insert(r.From)
		seen.Insert(r.To)
	}
	return seen
}
