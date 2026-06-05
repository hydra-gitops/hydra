package commands

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type DependencyGraph struct {
	Adj      map[types.Id][]types.Id
	InDegree map[types.Id]int
	Entities map[types.Id]entity.Entity
}

func BuildDependencyGraph(entities entity.Entities, refs []types.Ref) (DependencyGraph, error) {
	g := DependencyGraph{
		Adj:      make(map[types.Id][]types.Id),
		InDegree: make(map[types.Id]int),
		Entities: make(map[types.Id]entity.Entity),
	}

	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return DependencyGraph{}, err
		}
		g.Entities[id] = e
		g.InDegree[id] = 0
	}

	for _, ref := range refs {
		from := types.Id(ref.From)
		to := types.Id(ref.To)

		if ref.Reverse {
			from, to = to, from
		}

		if from == to {
			continue
		}
		if _, ok := g.Entities[from]; !ok {
			continue
		}
		if _, ok := g.Entities[to]; !ok {
			continue
		}

		// From depends on To: when To is ready, unblock From.
		g.Adj[to] = append(g.Adj[to], from)
		g.InDegree[from]++
	}

	return g, nil
}

// ResolveTransitiveWorkloadDeps computes transitive dependencies between workloads
// by following ref chains through non-workload intermediaries. For example, if
// Deployment A depends on Secret X, and Secret X depends on Secret Y (via a clone
// ref), and Secret Y depends on SopsSecret Z (via a reversed ref), and SopsSecret Z
// eventually connects to Deployment B, then A transitively depends on B.
//
// The Reverse flag on refs is respected: a ref with Reverse=true means the dependency
// direction is To→From instead of From→To.
func ResolveTransitiveWorkloadDeps(refs []types.Ref, workloadIds map[types.Id]bool) []types.Ref {
	if len(refs) == 0 || len(workloadIds) == 0 {
		return refs
	}

	type dependency struct {
		id       types.Id
		optional bool
	}

	dependsOn := make(map[types.Id][]dependency)
	for _, ref := range refs {
		from := ref.From
		to := ref.To
		if ref.Reverse {
			from, to = to, from
		}
		if from == to {
			continue
		}
		dependsOn[from] = append(dependsOn[from], dependency{
			id:       to,
			optional: ref.HasTag(types.RefTagOptionalStartup),
		})
	}

	type edge struct{ from, to types.Id }
	edgeOptionality := make(map[edge]bool)

	for wId := range workloadIds {
		type queueItem struct {
			id       types.Id
			optional bool
		}

		visited := map[queueItem]bool{{id: wId, optional: false}: true}
		var queue []queueItem
		for _, dep := range dependsOn[wId] {
			item := queueItem(dep)
			if visited[item] {
				continue
			}
			visited[item] = true
			if !workloadIds[dep.id] {
				queue = append(queue, item)
			}
		}

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]

			for _, dep := range dependsOn[current.id] {
				next := queueItem{
					id:       dep.id,
					optional: current.optional || dep.optional,
				}
				if visited[next] {
					continue
				}
				visited[next] = true

				if workloadIds[dep.id] {
					e := edge{from: wId, to: dep.id}
					existingOptional, exists := edgeOptionality[e]
					if !exists || (existingOptional && !next.optional) {
						edgeOptionality[e] = next.optional
					}
				} else {
					queue = append(queue, next)
				}
			}
		}
	}

	if len(edgeOptionality) == 0 {
		return refs
	}

	var synthetic []types.Ref
	for e, optional := range edgeOptionality {
		syntheticRef := types.Ref{
			RefType:      types.RefTypeIndirect,
			EndpointType: types.RefEndpointTypeId,
			From:         e.from,
			To:           e.to,
		}
		if optional {
			syntheticRef.Tags = []string{types.RefTagOptionalStartup}
		}
		synthetic = append(synthetic, syntheticRef)
	}
	sort.Slice(synthetic, func(i, j int) bool {
		if synthetic[i].From != synthetic[j].From {
			return synthetic[i].From < synthetic[j].From
		}
		return synthetic[i].To < synthetic[j].To
	})

	result := make([]types.Ref, 0, len(refs)+len(synthetic))
	result = append(result, refs...)
	result = append(result, synthetic...)
	return result
}

func TopologicalExecute(
	ctx context.Context,
	l log.Logger,
	entities entity.Entities,
	refs []types.Ref,
	start func(ctx context.Context, e entity.Entity) error,
	waitReady func(ctx context.Context, e entity.Entity) error,
) error {
	if entities.Len() == 0 {
		return nil
	}

	g, err := BuildDependencyGraph(entities, refs)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		id  types.Id
		err error
	}

	resultCh := make(chan result, len(g.Entities))
	var wg sync.WaitGroup

	launch := func(id types.Id) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := g.Entities[id]
			if err := start(ctx, e); err != nil {
				resultCh <- result{id: id, err: fmt.Errorf("start %s: %w", id, err)}
				return
			}
			if err := waitReady(ctx, e); err != nil {
				resultCh <- result{id: id, err: fmt.Errorf("waitReady %s: %w", id, err)}
				return
			}
			resultCh <- result{id: id}
		}()
	}

	pending := 0
	for id, deg := range g.InDegree {
		if deg == 0 {
			launch(id)
			pending++
		}
	}

	processed := make(map[types.Id]bool)
	var firstErr error

	for pending > 0 {
		r := <-resultCh
		pending--

		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
				cancel()
			}
			processed[r.id] = true
			continue
		}

		processed[r.id] = true

		if firstErr != nil {
			continue
		}

		for _, dep := range g.Adj[r.id] {
			g.InDegree[dep]--
			if g.InDegree[dep] == 0 {
				launch(dep)
				pending++
			}
		}
	}

	if firstErr != nil {
		wg.Wait()
		return firstErr
	}

	var cyclic []types.Id
	for id := range g.Entities {
		if !processed[id] {
			cyclic = append(cyclic, id)
		}
	}

	if len(cyclic) > 0 {
		l.Warn(logIdCommands, "dependency cycle detected, {count} entities will be started in arbitrary order",
			log.Int("count", len(cyclic)))
		for _, id := range cyclic {
			launch(id)
			pending++
		}
		for pending > 0 {
			r := <-resultCh
			pending--
			if r.err != nil && firstErr == nil {
				firstErr = r.err
				cancel()
			}
		}
		wg.Wait()
		if firstErr != nil {
			return firstErr
		}
	}

	return nil
}

type PlanEntry struct {
	Name         string
	Dependencies []string
}

// PlanTopologicalOrder computes a deterministic topological order from the
// dependency graph. Nodes at the same level are sorted alphabetically by ID.
// Cyclic nodes are appended at the end with their unresolved dependencies.
func PlanTopologicalOrder(g DependencyGraph) []PlanEntry {
	if len(g.Entities) == 0 {
		return nil
	}

	inDeg := make(map[types.Id]int, len(g.InDegree))
	for id, deg := range g.InDegree {
		inDeg[id] = deg
	}

	// Reverse adjacency: for each node, which nodes does it depend on?
	directDeps := make(map[types.Id][]types.Id, len(g.Entities))
	for to, froms := range g.Adj {
		for _, from := range froms {
			directDeps[from] = append(directDeps[from], to)
		}
	}

	var plan []PlanEntry
	processed := make(map[types.Id]bool, len(g.Entities))

	for len(processed) < len(g.Entities) {
		var ready []types.Id
		for id := range g.Entities {
			if !processed[id] && inDeg[id] == 0 {
				ready = append(ready, id)
			}
		}

		if len(ready) == 0 {
			break
		}

		sort.Slice(ready, func(i, j int) bool {
			return ready[i] < ready[j]
		})

		for _, id := range ready {
			deps := directDeps[id]
			var depNames []string
			for _, d := range deps {
				depNames = append(depNames, string(d))
			}
			sort.Strings(depNames)

			plan = append(plan, PlanEntry{
				Name:         string(id),
				Dependencies: depNames,
			})
			processed[id] = true

			for _, dependent := range g.Adj[id] {
				inDeg[dependent]--
			}
		}
	}

	// Append cyclic nodes
	var cyclic []types.Id
	for id := range g.Entities {
		if !processed[id] {
			cyclic = append(cyclic, id)
		}
	}
	sort.Slice(cyclic, func(i, j int) bool {
		return cyclic[i] < cyclic[j]
	})
	for _, id := range cyclic {
		deps := directDeps[id]
		var depNames []string
		for _, d := range deps {
			depNames = append(depNames, string(d))
		}
		sort.Strings(depNames)

		plan = append(plan, PlanEntry{
			Name:         string(id),
			Dependencies: depNames,
		})
	}

	return plan
}

// ResolveAppBasedWorkloadDeps infers cross-app workload dependencies from
// cross-app entity refs. If an entity in App A depends on an entity in App B,
// all workloads in App A are assumed to depend on all workloads in App B.
// This captures operator dependencies automatically: CRs reference their CRD
// (via the built-in _all.yaml ref-parser), and the CRD belongs to the operator
// app. Without this, the startup order cannot detect that e.g. cert-manager
// must be running before apps that use Certificate CRs.
func ResolveAppBasedWorkloadDeps(allEntities entity.Entities, refs []types.Ref, workloadIds map[types.Id]bool) []types.Ref {
	if len(refs) == 0 || len(workloadIds) == 0 {
		return refs
	}

	entityAppMap := make(map[types.Id][]types.AppId, allEntities.Len())
	for _, e := range allEntities.Items {
		id, err := e.Id()
		if err != nil {
			continue
		}
		appIds, err := e.AppIds()
		if err != nil || len(appIds) == 0 {
			continue
		}
		entityAppMap[id] = appIds
	}

	appWorkloads := make(map[types.AppId][]types.Id)
	for wId := range workloadIds {
		for _, appId := range entityAppMap[wId] {
			appWorkloads[appId] = append(appWorkloads[appId], wId)
		}
	}
	for appId := range appWorkloads {
		sort.Slice(appWorkloads[appId], func(i, j int) bool {
			return appWorkloads[appId][i] < appWorkloads[appId][j]
		})
	}

	type appEdge struct{ from, to types.AppId }
	seenAppEdges := make(map[appEdge]bool)

	for _, ref := range refs {
		from := ref.From
		to := ref.To
		if ref.Reverse {
			from, to = to, from
		}
		if from == to {
			continue
		}

		fromApps := entityAppMap[from]
		toApps := entityAppMap[to]

		for _, fa := range fromApps {
			for _, ta := range toApps {
				if fa == ta {
					continue
				}
				if len(appWorkloads[fa]) == 0 || len(appWorkloads[ta]) == 0 {
					continue
				}
				seenAppEdges[appEdge{from: fa, to: ta}] = true
			}
		}
	}

	if len(seenAppEdges) == 0 {
		return refs
	}

	type wEdge struct{ from, to types.Id }
	seen := make(map[wEdge]bool)
	var synthetic []types.Ref

	for ae := range seenAppEdges {
		for _, fromW := range appWorkloads[ae.from] {
			for _, toW := range appWorkloads[ae.to] {
				if fromW == toW {
					continue
				}
				we := wEdge{from: fromW, to: toW}
				if seen[we] {
					continue
				}
				seen[we] = true
				synthetic = append(synthetic, types.Ref{
					RefType:      types.RefTypeIndirect,
					EndpointType: types.RefEndpointTypeId,
					From:         fromW,
					To:           toW,
				})
			}
		}
	}

	if len(synthetic) == 0 {
		return refs
	}

	result := make([]types.Ref, 0, len(refs)+len(synthetic))
	result = append(result, refs...)
	result = append(result, synthetic...)
	return result
}

func ReverseRefs(refs []types.Ref) []types.Ref {
	if refs == nil {
		return nil
	}
	reversed := make([]types.Ref, len(refs))
	for i, r := range refs {
		reversed[i] = types.Ref{
			RefType:      r.RefType,
			EndpointType: r.EndpointType,
			From:         r.To,
			To:           r.From,
			Labels:       r.Labels,
			Tags:         r.Tags,
			Desc:         r.Desc,
			Reverse:      r.Reverse,
		}
	}
	return reversed
}
