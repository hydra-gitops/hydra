// Package workloadclosure matches Kubernetes workload-related entities against ownership predicates
// using metadata.ownerReferences first, then ref-based fallbacks such as Event.regarding.
package workloadclosure

import (
	"sync"
	"time"

	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// RefLabelObjectsetOwner is the ref label for Rancher wrangler objectset.rio.cattle.io
// child-to-owner edges.
const RefLabelObjectsetOwner = "objectset-owner"

// ParentVia describes how workload-closure or preset owner-resolution reached a parent candidate.
type ParentVia string

const (
	ParentViaOwnerRef                  ParentVia = "owner-ref"
	ParentViaRegardingRef              ParentVia = "regarding-ref"
	ParentViaWorkloadRegardingEventRef ParentVia = "workload-regarding-event-ref"
)

// ParentViaFromRefLabel converts a ref label into the generic parent-resolution via value.
func ParentViaFromRefLabel(label string) ParentVia {
	return ParentVia(label)
}

type ParentRefDirection string

const (
	ParentRefDirectionOutgoing ParentRefDirection = "outgoing"
	ParentRefDirectionIncoming ParentRefDirection = "incoming"
)

// ParentRefSource describes one ref-based parent source discovered from ref attributes.
type ParentRefSource struct {
	Via       ParentVia
	Direction ParentRefDirection
}

// ParentRefSourceFromRef returns the generic parent-source metadata declared on a ref, if any.
func ParentRefSourceFromRef(ref types.Ref) (ParentRefSource, bool) {
	var via ParentVia
	var direction ParentRefDirection
	for _, attr := range ref.Attributes {
		switch attr.Type {
		case types.RefAttributeHydraParentVia:
			via = ParentVia(attr.Value)
		case types.RefAttributeHydraParentDirection:
			direction = ParentRefDirection(attr.Value)
		}
	}
	if via == "" || direction == "" {
		return ParentRefSource{}, false
	}
	return ParentRefSource{Via: via, Direction: direction}, true
}

// MatchInput is a precomputed view of the live cluster for workload-closure predicate matching.
type MatchInput struct {
	Refs                     []types.Ref
	EntityByID               map[types.Id]entity.Entity
	UIDMap                   map[types.Uid]entity.Entity
	Index                    map[types.Id]entity.Entity
	Key                      types.EntityKeyUnstructured
	RefsByFrom               map[types.Id][]types.Ref
	RefsByTo                 map[types.Id][]types.Ref
	ImmediateParentIDsByID   map[types.Id][]types.Id
	ImmediateParentStatsByID map[types.Id]PredicateMatchStats
}

type closureWalkState struct {
	queue   []types.Id
	touched []types.Id
	state   map[types.Id]uint8
}

var closureWalkStatePool = sync.Pool{
	New: func() any {
		return &closureWalkState{
			state: make(map[types.Id]uint8),
		}
	},
}

// PredicateMatchStats captures what one PredicateMatches evaluation had to do.
type PredicateMatchStats struct {
	DirectMatch         bool
	ClosureWalk         bool
	VisitedEntities     int
	OwnerCandidates     int
	RegardingCandidates int
	RefCandidatesByVia  map[ParentVia]int
	Duration            time.Duration
}

// NewMatchInput builds indexes for MatchInput. Refs are the full ref list from references.Refs.
func NewMatchInput(
	refs []types.Ref,
	clusterEntities entity.Entities,
	key types.EntityKeyUnstructured,
) (MatchInput, error) {
	return NewMatchInputWithExtraEntities(refs, clusterEntities, entity.Entities{}, key)
}

// NewMatchInputWithExtraEntities builds indexes for MatchInput and includes extraEntities in the
// id lookup. This lets workload closure follow merged inventory refs whose source exists only as a
// template entity while keeping UID ownership resolution based on the live cluster inventory.
func NewMatchInputWithExtraEntities(
	refs []types.Ref,
	clusterEntities entity.Entities,
	extraEntities entity.Entities,
	key types.EntityKeyUnstructured,
) (MatchInput, error) {
	byID := make(map[types.Id]entity.Entity, clusterEntities.Len())
	uidMap := make(map[types.Uid]entity.Entity)
	index := make(map[types.Id]entity.Entity)
	refsByFrom := make(map[types.Id][]types.Ref)
	refsByTo := make(map[types.Id][]types.Ref)
	for _, ref := range refs {
		refsByFrom[ref.From] = append(refsByFrom[ref.From], ref)
		refsByTo[ref.To] = append(refsByTo[ref.To], ref)
	}
	for _, e := range clusterEntities.Items {
		id, err := e.Id()
		if err != nil {
			return MatchInput{}, err
		}
		byID[id] = e
		index[id] = e
		u, ok := e.ReadOnlyUid(key)
		if ok && u != "" {
			uidMap[u] = e
		}
	}
	for _, e := range extraEntities.Items {
		id, err := e.Id()
		if err != nil {
			return MatchInput{}, err
		}
		if _, exists := byID[id]; exists {
			continue
		}
		byID[id] = e
		index[id] = e
	}
	immediateParentIDs := make(map[types.Id][]types.Id, len(byID))
	immediateParentStats := make(map[types.Id]PredicateMatchStats, len(byID))
	for id, e := range byID {
		parentIDs, stats, err := buildImmediateParents(id, e, byID, uidMap, index, key, refsByFrom, refsByTo)
		if err != nil {
			return MatchInput{}, err
		}
		immediateParentIDs[id] = parentIDs
		immediateParentStats[id] = stats
	}
	return MatchInput{
		Refs:                     refs,
		EntityByID:               byID,
		UIDMap:                   uidMap,
		Index:                    index,
		Key:                      key,
		RefsByFrom:               refsByFrom,
		RefsByTo:                 refsByTo,
		ImmediateParentIDsByID:   immediateParentIDs,
		ImmediateParentStatsByID: immediateParentStats,
	}, nil
}

// EmptyMatchInput returns an input with no refs or entities (only direct EvalBool will succeed).
func EmptyMatchInput(key types.EntityKeyUnstructured) MatchInput {
	return MatchInput{
		EntityByID:               map[types.Id]entity.Entity{},
		UIDMap:                   map[types.Uid]entity.Entity{},
		Index:                    map[types.Id]entity.Entity{},
		RefsByFrom:               map[types.Id][]types.Ref{},
		RefsByTo:                 map[types.Id][]types.Ref{},
		ImmediateParentIDsByID:   map[types.Id][]types.Id{},
		ImmediateParentStatsByID: map[types.Id]PredicateMatchStats{},
		Key:                      key,
	}
}

// PredicateMatches evaluates pred against e. If it does not match directly, ownerReferences are
// walked first, then ref fallbacks (for example Event.regarding).
func (in MatchInput) PredicateMatches(e entity.Entity, pred cel.Predicate) (bool, error) {
	ok, _, err := in.PredicateMatchesWithStats(e, pred)
	return ok, err
}

// PredicateMatchesWithStats is like [PredicateMatches] but also returns per-call traversal stats.
func (in MatchInput) PredicateMatchesWithStats(e entity.Entity, pred cel.Predicate) (bool, PredicateMatchStats, error) {
	started := time.Now()
	ok, err := evalPred(pred, e)
	if err != nil {
		return false, PredicateMatchStats{Duration: time.Since(started)}, err
	}
	if ok {
		return true, PredicateMatchStats{
			DirectMatch: true,
			Duration:    time.Since(started),
		}, nil
	}
	ok, stats, err := in.matchViaClosure(e, pred)
	stats.Duration = time.Since(started)
	return ok, stats, err
}

func evalPred(pred cel.Predicate, e entity.Entity) (bool, error) {
	return pred.EvalBool(e, types.MissingKeysAccept)
}

// ImmediateParents returns the one-hop parent candidates used by the closure walk:
// ownerReferences first, then Event.regarding, then configured ref-based parent sources.
func (in MatchInput) ImmediateParents(e entity.Entity) ([]types.Id, PredicateMatchStats, error) {
	id, err := e.Id()
	if err != nil {
		return nil, PredicateMatchStats{}, err
	}
	return in.ImmediateParentIDsByID[id], in.ImmediateParentStatsByID[id], nil
}

func buildImmediateParents(
	id types.Id,
	e entity.Entity,
	byID map[types.Id]entity.Entity,
	uidMap map[types.Uid]entity.Entity,
	clusterIndex map[types.Id]entity.Entity,
	key types.EntityKeyUnstructured,
	refsByFrom map[types.Id][]types.Ref,
	refsByTo map[types.Id][]types.Ref,
) ([]types.Id, PredicateMatchStats, error) {
	var stats PredicateMatchStats
	seen := sets.New[types.Id]()
	var out []types.Id
	appendParent := func(item entity.Entity, counter *int, via ParentVia) error {
		pid, idErr := item.Id()
		if idErr != nil {
			return idErr
		}
		if seen.Has(pid) {
			return nil
		}
		seen.Insert(pid)
		out = append(out, pid)
		if counter != nil {
			*counter++
		}
		if via != "" {
			if stats.RefCandidatesByVia == nil {
				stats.RefCandidatesByVia = make(map[ParentVia]int)
			}
			stats.RefCandidatesByVia[via]++
		}
		return nil
	}

	owners, err := kubernetesImmediateOwners(e, key, uidMap, clusterIndex)
	if err != nil {
		return nil, stats, err
	}
	for _, parent := range owners {
		if err := appendParent(parent, &stats.OwnerCandidates, ""); err != nil {
			return nil, stats, err
		}
	}

	gvk, err := e.GVKString()
	if err != nil {
		return nil, stats, err
	}
	if isKubernetesEventGVK(string(gvk)) {
		fromRefs := refsByFrom[id]
		for i := range fromRefs {
			r := &fromRefs[i]
			if r.EndpointType != types.RefEndpointTypeId || r.RefType != types.RefTypeRegarding {
				continue
			}
			if subj, ok := byID[r.To]; ok {
				if err := appendParent(subj, &stats.RegardingCandidates, ""); err != nil {
					return nil, stats, err
				}
			} else if subj, err := minimalClusterEntityFromID(r.To); err == nil {
				if err := appendParent(subj, &stats.RegardingCandidates, ""); err != nil {
					return nil, stats, err
				}
			}
		}

		toRefs := refsByTo[id]
		for i := range toRefs {
			r := &toRefs[i]
			if r.EndpointType != types.RefEndpointTypeId || !refLabelsContain(r.Labels, "workloadRegardingEvent") {
				continue
			}
			if subj, ok := byID[r.From]; ok {
				if err := appendParent(subj, &stats.RegardingCandidates, ""); err != nil {
					return nil, stats, err
				}
			} else if subj, err := minimalClusterEntityFromID(r.From); err == nil {
				if err := appendParent(subj, &stats.RegardingCandidates, ""); err != nil {
					return nil, stats, err
				}
			}
		}
	}

	fromRefs := refsByFrom[id]
	for i := range fromRefs {
		r := &fromRefs[i]
		source, ok := ParentRefSourceFromRef(*r)
		if !ok || source.Direction != ParentRefDirectionOutgoing || r.EndpointType != types.RefEndpointTypeId {
			continue
		}
		if subj, ok := byID[r.To]; ok {
			if err := appendParent(subj, nil, source.Via); err != nil {
				return nil, stats, err
			}
		} else if subj, err := minimalClusterEntityFromID(r.To); err == nil {
			if err := appendParent(subj, nil, source.Via); err != nil {
				return nil, stats, err
			}
		}
	}

	toRefs := refsByTo[id]
	for i := range toRefs {
		r := &toRefs[i]
		source, ok := ParentRefSourceFromRef(*r)
		if !ok || source.Direction != ParentRefDirectionIncoming || r.EndpointType != types.RefEndpointTypeId {
			continue
		}
		if src, ok := byID[r.From]; ok {
			if err := appendParent(src, nil, source.Via); err != nil {
				return nil, stats, err
			}
		}
	}

	return out, stats, nil
}

func (in MatchInput) matchViaClosure(start entity.Entity, pred cel.Predicate) (bool, PredicateMatchStats, error) {
	stats := PredicateMatchStats{ClosureWalk: true}
	startID, err := start.Id()
	if err != nil {
		return false, stats, err
	}
	state := closureWalkStatePool.Get().(*closureWalkState)
	defer func() {
		for _, id := range state.touched {
			delete(state.state, id)
		}
		state.queue = state.queue[:0]
		state.touched = state.touched[:0]
		closureWalkStatePool.Put(state)
	}()
	mark := func(id types.Id, value uint8) {
		if _, ok := state.state[id]; !ok {
			state.touched = append(state.touched, id)
		}
		state.state[id] = value
	}
	mark(startID, 1)
	state.queue = append(state.queue[:0], startID)
	for len(state.queue) > 0 {
		cid := state.queue[0]
		state.queue = state.queue[1:]
		if state.state[cid] == 2 {
			continue
		}
		cur, ok := in.EntityByID[cid]
		if !ok {
			continue
		}
		state.state[cid] = 2
		stats.VisitedEntities++

		ok, err := evalPred(pred, cur)
		if err != nil || ok {
			return ok, stats, err
		}

		parentIDs, parentStats, err := in.ImmediateParents(cur)
		if err != nil {
			return false, stats, err
		}
		stats.OwnerCandidates += parentStats.OwnerCandidates
		stats.RegardingCandidates += parentStats.RegardingCandidates
		for via, count := range parentStats.RefCandidatesByVia {
			if stats.RefCandidatesByVia == nil {
				stats.RefCandidatesByVia = make(map[ParentVia]int)
			}
			stats.RefCandidatesByVia[via] += count
		}
		for _, pid := range parentIDs {
			if state.state[pid] != 0 {
				continue
			}
			mark(pid, 1)
			state.queue = append(state.queue, pid)
		}
	}
	return false, stats, nil
}

func refLabelsContain(labels []string, want string) bool {
	for _, l := range labels {
		if l == want {
			return true
		}
	}
	return false
}

func isKubernetesEventGVK(gvk string) bool {
	return gvk == string(types.KubernetesGvkEventsK8sIoV1Event) ||
		gvk == string(types.KubernetesGvkV1Event)
}

func kubernetesImmediateOwners(
	e entity.Entity,
	key types.EntityKeyUnstructured,
	uidMap map[types.Uid]entity.Entity,
	clusterIndex map[types.Id]entity.Entity,
) ([]entity.Entity, error) {
	u, ok := e.ReadOnlyUnstructured(key)
	if !ok {
		return nil, nil
	}
	refs := u.GetOwnerReferences()
	if len(refs) == 0 {
		return nil, nil
	}
	ownerNs, nsOK := workloadNamespaceForOwnerWalk(e, key)
	if !nsOK {
		ownerNs = ""
	}
	var out []entity.Entity
	for _, ref := range refs {
		if ref.APIVersion == "" || ref.Kind == "" || ref.Name == "" {
			continue
		}
		if ref.UID != "" {
			if parent, ok := uidMap[types.Uid(ref.UID)]; ok {
				out = append(out, parent)
				continue
			}
		}
		ownerID, idErr := clusterResourceIDFromOwnerReference(string(ref.APIVersion), string(ref.Kind), string(ref.Name), ownerNs)
		if idErr != nil {
			continue
		}
		if parent, ok := clusterIndex[ownerID]; ok {
			out = append(out, parent)
		}
	}
	return out, nil
}

func workloadNamespaceForOwnerWalk(e entity.Entity, key types.EntityKeyUnstructured) (types.Namespace, bool) {
	id, err := e.Id()
	if err != nil {
		return "", false
	}
	_, _, _, ns, _, err := id.Components()
	if err != nil {
		return "", false
	}
	if ns != "" {
		return ns, true
	}
	u, ok := e.ReadOnlyUnstructured(key)
	if !ok {
		return "", false
	}
	nsName := u.GetNamespace()
	if nsName == "" {
		return "", false
	}
	return types.Namespace(nsName), true
}

func clusterResourceIDFromOwnerReference(apiVersion, kind, name string, ownerNamespace types.Namespace) (types.Id, error) {
	av, err := types.ParseApiVersion(apiVersion)
	if err != nil {
		return "", err
	}
	return types.NewId(av.Group, av.Version, types.Kind(kind), ownerNamespace, types.Name(name)), nil
}

// MinimalClusterEntityFromID builds a minimal cluster-shaped entity for id-only predicate checks.
func MinimalClusterEntityFromID(id types.Id) (entity.Entity, error) {
	return minimalClusterEntityFromID(id)
}

func minimalClusterEntityFromID(id types.Id) (entity.Entity, error) {
	g, ver, k, ns, name, err := id.Components()
	if err != nil {
		return entity.Entity{}, err
	}
	b := entity.NewEntityBuilder().
		WithGroup(g).
		WithVersion(ver).
		WithKind(k).
		WithName(name)
	b = b.WithNamespace(ns)
	if ns == "" {
		b = b.WithNamespaced(types.NamespacedNo)
	} else {
		b = b.WithNamespaced(types.NamespacedYes)
	}
	return b.Build()
}
