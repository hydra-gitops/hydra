package hydra

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"hydra-gitops.org/hydra/hydra-go/core/workloadclosure"
)

// ClusterDefaultsPresetMatchTiming accumulates per-preset wall time for each evaluation of one entity
// inside [ClusterDefaultsPresetEvalCache.MatchingPresetIDsWithRegarding] (one outer preset iteration).
// Typical usage: create one instance per inventory scan, pass it to MatchingPresetIDsWithRegarding for
// every entity, then call [ClusterDefaultsPresetMatchTiming.FormatReport].
type ClusterDefaultsPresetMatchTiming struct {
	mu       sync.Mutex
	byPreset map[string]*clusterDefaultsPresetDurAgg
}

type clusterDefaultsPresetDurAgg struct {
	minNanos int64
	maxNanos int64
	sumNanos int64
	count    int64
}

// Record adds one sample for presetID. Safe for concurrent calls.
func (t *ClusterDefaultsPresetMatchTiming) Record(presetID string, d time.Duration) {
	if t == nil {
		return
	}
	n := d.Nanoseconds()
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.byPreset == nil {
		t.byPreset = make(map[string]*clusterDefaultsPresetDurAgg)
	}
	a := t.byPreset[presetID]
	if a == nil {
		t.byPreset[presetID] = &clusterDefaultsPresetDurAgg{
			minNanos: n,
			maxNanos: n,
			sumNanos: n,
			count:    1,
		}
		return
	}
	if n < a.minNanos {
		a.minNanos = n
	}
	if n > a.maxNanos {
		a.maxNanos = n
	}
	a.sumNanos += n
	a.count++
}

type clusterDefaultsPresetTimingRow struct {
	id    string
	count int64
	min   time.Duration
	max   time.Duration
	sum   time.Duration
}

// FormatReport returns a multi-line summary sorted by sum (descending). entityCount is the number of
// entities scanned in this pass (for context in the header only).
func (t *ClusterDefaultsPresetMatchTiming) FormatReport(phase string, entityCount int) string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	rows := make([]clusterDefaultsPresetTimingRow, 0, len(t.byPreset))
	for id, a := range t.byPreset {
		rows = append(rows, clusterDefaultsPresetTimingRow{
			id:    id,
			count: a.count,
			min:   time.Duration(a.minNanos),
			max:   time.Duration(a.maxNanos),
			sum:   time.Duration(a.sumNanos),
		})
	}
	t.mu.Unlock()
	slices.SortFunc(rows, func(a, b clusterDefaultsPresetTimingRow) int {
		if c := cmp.Compare(b.sum, a.sum); c != 0 {
			return c
		}
		return cmp.Compare(a.id, b.id)
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s: preset match timing (entities=%d, samples per preset=entity evaluations)\n", phase, entityCount)
	for _, r := range rows {
		fmt.Fprintf(&b, "  preset=%s count=%d min=%s max=%s sum=%s\n",
			r.id, r.count, r.min, r.max, r.sum)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

// ClusterDefaultsBatchPresetProfile accumulates a detailed profile for one preset inside
// MatchingPresetIDsByEntityWithRegarding, intended for the visible "preset · batch match" step.
type ClusterDefaultsBatchPresetProfile struct {
	mu                  sync.Mutex
	presetID            string
	entityCount         int
	anchorPredicates    int
	directMatched       int64
	anchorCandidates    int64
	anchorRuleCalls     int64
	anchorRuleMatches   int64
	directAnchorHits    int64
	closureWalkCalls    int64
	closureVisitedSum   int64
	closureVisitedMax   int64
	ownerCandidates     int64
	regardingCandidates int64
	refCandidatesByVia  map[workloadclosure.ParentVia]int64
	explicitIDLookups   int64
	celSelects          int64
	anchorChecks        int64
	templateSkipped     int64
	resolvedByOwnerApp  int64
	resolvedByParent    int64
	explicitIDDuration  time.Duration
	celDuration         time.Duration
	anchorDuration      time.Duration
	totalDuration       time.Duration
	anchorRules         map[string]*clusterDefaultsBatchPresetAnchorRuleAgg
}

type clusterDefaultsBatchPresetAnchorRuleAgg struct {
	calls   int64
	matches int64
	sum     time.Duration
	max     time.Duration
}

// NewClusterDefaultsBatchPresetProfile returns a profiler that records detailed timings only for
// targetPresetID.
func NewClusterDefaultsBatchPresetProfile(targetPresetID string, entityCount int) *ClusterDefaultsBatchPresetProfile {
	targetPresetID = strings.TrimSpace(targetPresetID)
	if targetPresetID == "" {
		return nil
	}
	return &ClusterDefaultsBatchPresetProfile{
		presetID:           targetPresetID,
		entityCount:        entityCount,
		anchorRules:        make(map[string]*clusterDefaultsBatchPresetAnchorRuleAgg),
		refCandidatesByVia: make(map[workloadclosure.ParentVia]int64),
	}
}

// EnabledForPreset reports whether detailed profiling should be recorded for presetID.
func (p *ClusterDefaultsBatchPresetProfile) EnabledForPreset(presetID string) bool {
	return p != nil && p.presetID == presetID
}

// RecordExplicitIDs adds one explicit-id lookup segment.
func (p *ClusterDefaultsBatchPresetProfile) RecordExplicitIDs(d time.Duration, count int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.explicitIDDuration += d
	p.explicitIDLookups += int64(count)
}

// RecordCEL adds one CEL Select segment.
func (p *ClusterDefaultsBatchPresetProfile) RecordCEL(d time.Duration) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.celDuration += d
	p.celSelects++
}

// RecordAnchor adds one closure anchor predicate check segment.
func (p *ClusterDefaultsBatchPresetProfile) RecordAnchor(d time.Duration) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.anchorDuration += d
	p.anchorChecks++
}

// RecordTemplateSkippedEntity counts one live entity skipped entirely because a template already
// explains it.
func (p *ClusterDefaultsBatchPresetProfile) RecordTemplateSkippedEntity() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.templateSkipped++
}

// SetAnchorPredicateCount records how many compiled anchor predicates the preset has.
func (p *ClusterDefaultsBatchPresetProfile) SetAnchorPredicateCount(count int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.anchorPredicates = count
}

// RecordDirectMatchedEntity counts one entity that never needed anchor matching because direct
// id/CEL matching already succeeded.
func (p *ClusterDefaultsBatchPresetProfile) RecordDirectMatchedEntity() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.directMatched++
}

// RecordAnchorCandidateEntity counts one entity entering the anchor matching loop.
func (p *ClusterDefaultsBatchPresetProfile) RecordAnchorCandidateEntity() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.anchorCandidates++
}

// RecordResolvedByOwnerApp counts one entity resolved away from preset matching by inherited owner app.
func (p *ClusterDefaultsBatchPresetProfile) RecordResolvedByOwnerApp() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resolvedByOwnerApp++
}

// RecordResolvedByParentPreset counts one entity resolved to a preset via parent traversal.
func (p *ClusterDefaultsBatchPresetProfile) RecordResolvedByParentPreset() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.resolvedByParent++
}

// RecordClosureLayer adds one parent-layer traversal segment to the aggregate closure counters.
func (p *ClusterDefaultsBatchPresetProfile) RecordClosureLayer(size int) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closureWalkCalls++
	p.closureVisitedSum += int64(size)
	if int64(size) > p.closureVisitedMax {
		p.closureVisitedMax = int64(size)
	}
}

// RecordAnchorRule records one anchor predicate evaluation together with closure traversal stats.
func (p *ClusterDefaultsBatchPresetProfile) RecordAnchorRule(
	rule string,
	d time.Duration,
	matched bool,
	stats workloadclosure.PredicateMatchStats,
) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.anchorRuleCalls++
	if matched {
		p.anchorRuleMatches++
	}
	if stats.DirectMatch {
		p.directAnchorHits++
	}
	if stats.ClosureWalk {
		p.closureWalkCalls++
		p.closureVisitedSum += int64(stats.VisitedEntities)
		if int64(stats.VisitedEntities) > p.closureVisitedMax {
			p.closureVisitedMax = int64(stats.VisitedEntities)
		}
		p.ownerCandidates += int64(stats.OwnerCandidates)
		p.regardingCandidates += int64(stats.RegardingCandidates)
		for via, count := range stats.RefCandidatesByVia {
			p.refCandidatesByVia[via] += int64(count)
		}
	}
	a := p.anchorRules[rule]
	if a == nil {
		a = &clusterDefaultsBatchPresetAnchorRuleAgg{}
		p.anchorRules[rule] = a
	}
	a.calls++
	if matched {
		a.matches++
	}
	a.sum += d
	if d > a.max {
		a.max = d
	}
}

// RecordTotal adds the total wall time for the preset iteration.
func (p *ClusterDefaultsBatchPresetProfile) RecordTotal(d time.Duration) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.totalDuration += d
}

// FormatReport returns a multi-line summary for the profiled batch-match preset.
func (p *ClusterDefaultsBatchPresetProfile) FormatReport(phase string) string {
	if p == nil {
		return ""
	}
	p.mu.Lock()
	rows := make([]clusterDefaultsBatchPresetAnchorRuleRow, 0, len(p.anchorRules))
	for rule, agg := range p.anchorRules {
		rows = append(rows, clusterDefaultsBatchPresetAnchorRuleRow{
			rule:    rule,
			calls:   agg.calls,
			matches: agg.matches,
			sum:     agg.sum,
			max:     agg.max,
		})
	}
	anchorPredicates := p.anchorPredicates
	directMatched := p.directMatched
	anchorCandidates := p.anchorCandidates
	anchorRuleCalls := p.anchorRuleCalls
	anchorRuleMatches := p.anchorRuleMatches
	directAnchorHits := p.directAnchorHits
	closureWalkCalls := p.closureWalkCalls
	closureVisitedSum := p.closureVisitedSum
	closureVisitedMax := p.closureVisitedMax
	ownerCandidates := p.ownerCandidates
	regardingCandidates := p.regardingCandidates
	refCandidatesByVia := make(map[workloadclosure.ParentVia]int64, len(p.refCandidatesByVia))
	for via, count := range p.refCandidatesByVia {
		refCandidatesByVia[via] = count
	}
	explicitIDLookups := p.explicitIDLookups
	celSelects := p.celSelects
	anchorChecks := p.anchorChecks
	templateSkipped := p.templateSkipped
	resolvedByOwnerApp := p.resolvedByOwnerApp
	resolvedByParent := p.resolvedByParent
	explicitIDDuration := p.explicitIDDuration
	celDuration := p.celDuration
	anchorDuration := p.anchorDuration
	totalDuration := p.totalDuration
	p.mu.Unlock()
	slices.SortFunc(rows, func(a, b clusterDefaultsBatchPresetAnchorRuleRow) int {
		if c := cmp.Compare(b.sum, a.sum); c != 0 {
			return c
		}
		return cmp.Compare(a.rule, b.rule)
	})
	var b strings.Builder
	fmt.Fprintf(&b, "%s: batch preset profile (preset=%s, entities=%d)\n", phase, p.presetID, p.entityCount)
	fmt.Fprintf(&b, "  total=%s\n", totalDuration)
	fmt.Fprintf(&b, "  explicitIds count=%d sum=%s\n", explicitIDLookups, explicitIDDuration)
	fmt.Fprintf(&b, "  celSelects count=%d sum=%s\n", celSelects, celDuration)
	fmt.Fprintf(&b, "  anchorChecks count=%d sum=%s\n", anchorChecks, anchorDuration)
	fmt.Fprintf(&b, "  templateSkippedEntities=%d resolvedByOwnerApp=%d resolvedByParentPreset=%d\n",
		templateSkipped, resolvedByOwnerApp, resolvedByParent)
	fmt.Fprintf(&b, "  anchorPredicates=%d directMatchedEntities=%d anchorCandidateEntities=%d anchorRuleCalls=%d anchorRuleMatches=%d directAnchorHits=%d closureWalkCalls=%d\n",
		anchorPredicates, directMatched, anchorCandidates, anchorRuleCalls, anchorRuleMatches, directAnchorHits, closureWalkCalls)
	if anchorCandidates > 0 {
		fmt.Fprintf(&b, "  avgAnchorCallsPerCandidate=%.2f\n", float64(anchorRuleCalls)/float64(anchorCandidates))
	}
	if closureWalkCalls > 0 {
		fmt.Fprintf(&b, "  closureVisited sum=%d avg=%.2f max=%d ownerCandidates=%d regardingCandidates=%d",
			closureVisitedSum,
			float64(closureVisitedSum)/float64(closureWalkCalls),
			closureVisitedMax,
			ownerCandidates,
			regardingCandidates)
		if len(refCandidatesByVia) > 0 {
			viaKeys := make([]workloadclosure.ParentVia, 0, len(refCandidatesByVia))
			for via := range refCandidatesByVia {
				viaKeys = append(viaKeys, via)
			}
			slices.SortFunc(viaKeys, func(a, b workloadclosure.ParentVia) int {
				return cmp.Compare(string(a), string(b))
			})
			for _, via := range viaKeys {
				fmt.Fprintf(&b, " %sCandidates=%d", via, refCandidatesByVia[via])
			}
		}
		b.WriteString("\n")
	}
	limit := 8
	if len(rows) < limit {
		limit = len(rows)
	}
	for i := 0; i < limit; i++ {
		r := rows[i]
		fmt.Fprintf(&b, "  slowAnchor[%d] calls=%d matches=%d sum=%s max=%s rule=%q\n",
			i+1, r.calls, r.matches, r.sum, r.max, strings.ReplaceAll(r.rule, "\n", " | "))
	}
	return strings.TrimSuffix(b.String(), "\n")
}

type clusterDefaultsBatchPresetAnchorRuleRow struct {
	rule    string
	calls   int64
	matches int64
	sum     time.Duration
	max     time.Duration
}
