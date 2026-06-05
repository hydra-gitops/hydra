package commands

import (
	"fmt"
	"io"
	"slices"
	"sort"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Scale workload sync states for [ComputeClusterScaleWorkloadStatusReport].
const (
	ClusterScaleWorkloadStateUp        = "up"
	ClusterScaleWorkloadStateDown      = "down"
	ClusterScaleWorkloadStateOutOfSync = "out_of_sync"
	ClusterScaleWorkloadStateMissing   = "missing"
	// ClusterScaleWorkloadStateOK is a Job with no live object whose out-like refs (produces/downstream) are
	// satisfied: all dependency rows are up+ready (e.g. TTL-cleaned init Job after secrets exist).
	ClusterScaleWorkloadStateOK = "ok"
	ClusterScaleWorkloadStateNA = "n/a"
)

// ClusterScaleWorkloadDependencyStatus is one dependency row for scale status (sync + optional ready).
type ClusterScaleWorkloadDependencyStatus struct {
	WorkloadId htypes.Id `yaml:"workloadId"`
	Optional   bool      `yaml:"optional,omitempty"`
	// RefRole: labeled direct edge (source / consumption labels), else GVK fallbacks (Secret/ConfigMap → prerequisite,
	// ReplicaSet/Pod → downstream). Omitted when unknown.
	RefRole       string   `yaml:"refRole,omitempty"`
	State         string   `yaml:"state"`
	Ready         string   `yaml:"ready,omitempty"`
	ReadyMessages []string `yaml:"readyMessages,omitempty"`
}

// ClusterScaleWorkloadStatus is one scale target and its dependency rows.
type ClusterScaleWorkloadStatus struct {
	AppId         htypes.AppId                           `yaml:"appId,omitempty"`
	WorkloadId    htypes.Id                              `yaml:"workloadId"`
	State         string                                 `yaml:"state"`
	Ready         string                                 `yaml:"ready,omitempty"`
	ReadyMessages []string                               `yaml:"readyMessages,omitempty"`
	Dependencies  []ClusterScaleWorkloadDependencyStatus `yaml:"dependencies,omitempty"`
}

// ClusterScaleStatusReport is the structured result for `hydra gitops scale status` (text or YAML).
type ClusterScaleStatusReport struct {
	Workloads []ClusterScaleWorkloadStatus `yaml:"workloads"`
}

func formatScaleStatusStateForText(state string) string {
	if state == ClusterScaleWorkloadStateOutOfSync {
		return "out of sync"
	}
	if state == ClusterScaleWorkloadStateMissing {
		return "missing"
	}
	if state == ClusterScaleWorkloadStateOK {
		return "ok"
	}
	if state == ClusterScaleWorkloadStateNA {
		return "n/a"
	}
	return state
}

func scaleStatusStateColor(state string) colors.Color {
	switch state {
	case ClusterScaleWorkloadStateUp, ClusterScaleWorkloadStateOK:
		return colors.Green
	case ClusterScaleWorkloadStateDown:
		return colors.Yellow
	case ClusterScaleWorkloadStateMissing:
		return colors.LightYellow
	case ClusterScaleWorkloadStateNA:
		return colors.Yellow
	default:
		return colors.Red
	}
}

func scaleReadyLabelColor(ready string) colors.Color {
	switch ready {
	case ClusterScaleReadyReady:
		return colors.Green
	case ClusterScaleReadyNotReady:
		return colors.Red
	default:
		return colors.Yellow
	}
}

// scaleStatusTextRow is one line for [WriteClusterScaleStatusText] (three-column block layout).
type scaleStatusTextRow struct {
	col1           string // plain-text first column (for width measurement and no-color output)
	state          string
	ready          string
	readyMessages  []string
	isRootWorkload bool // scale target row (not a dependency); styled in color mode
	// refFlowTag is "in" / "out" for a classified dependency edge (printed in light blue when color is on).
	refFlowTag string
}

func primaryAppIdFromEntity(e entity.Entity) htypes.AppId {
	ids, err := e.AppIds()
	if err != nil || len(ids) == 0 {
		return ""
	}
	sorted := slices.Clone(ids)
	slices.Sort(sorted)
	return sorted[0]
}

func sortScaleStatusWorkloadsByAppThenWorkload(ws []ClusterScaleWorkloadStatus) {
	slices.SortFunc(ws, func(a, b ClusterScaleWorkloadStatus) int {
		if c := strings.Compare(string(a.AppId), string(b.AppId)); c != 0 {
			return c
		}
		return strings.Compare(string(a.WorkloadId), string(b.WorkloadId))
	})
}

func scaleStatusAppHeaderLabel(appID htypes.AppId) string {
	if appID == "" {
		return "(no app)"
	}
	return string(appID)
}

func workloadRowSyncAndReadyOK(state, ready string) bool {
	if state == ClusterScaleWorkloadStateOK {
		return true
	}
	if state != ClusterScaleWorkloadStateUp {
		return false
	}
	if ready != "" && ready != ClusterScaleReadyReady {
		return false
	}
	return true
}

// scaleStatusWorkloadBlockFullyOK is true when the scale target and every dependency row are sync-up and,
// when a ready value is set, ready.
func scaleStatusWorkloadBlockFullyOK(w ClusterScaleWorkloadStatus) bool {
	if !workloadRowSyncAndReadyOK(w.State, w.Ready) {
		return false
	}
	for _, d := range w.Dependencies {
		if !workloadRowSyncAndReadyOK(d.State, d.Ready) {
			return false
		}
	}
	return true
}

// scaleStatusWorkloadOmitFromDefaultView is true when this scale-target block need not appear without --all:
// fully healthy (up/ok root and deps), or a **missing** Job whose listed dependencies are all up+ready
// (at least one dependency row required).
func scaleStatusWorkloadOmitFromDefaultView(w ClusterScaleWorkloadStatus) bool {
	if scaleStatusWorkloadBlockFullyOK(w) {
		return true
	}
	if w.State != ClusterScaleWorkloadStateMissing || len(w.Dependencies) == 0 {
		return false
	}
	for _, d := range w.Dependencies {
		if !workloadRowSyncAndReadyOK(d.State, d.Ready) {
			return false
		}
	}
	return true
}

// FilterClusterScaleStatusReportOmitFullyHealthyApps drops individual scale-target rows that need no attention
// for the default view: root+deps fully **up**/**ok** and ready, or a **missing** Job with all dependency rows
// **up**+**ready** (and at least one dependency). When showAll is true, returns report unchanged.
func FilterClusterScaleStatusReportOmitFullyHealthyApps(report ClusterScaleStatusReport, showAll bool) ClusterScaleStatusReport {
	if showAll || len(report.Workloads) == 0 {
		return report
	}
	out := make([]ClusterScaleWorkloadStatus, 0, len(report.Workloads))
	for _, w := range report.Workloads {
		if scaleStatusWorkloadOmitFromDefaultView(w) {
			continue
		}
		out = append(out, w)
	}
	sortScaleStatusWorkloadsByAppThenWorkload(out)
	return ClusterScaleStatusReport{Workloads: out}
}

// ClusterScaleStatusAllTargetsOmittedAsHealthy is true when the default filtered view dropped every
// scale-target row because each block needed no attention (--all not set), while the full report had
// at least one scale target. In that case the text UI prints an all-clear line instead of empty stdout.
func ClusterScaleStatusAllTargetsOmittedAsHealthy(filtered, full ClusterScaleStatusReport, showAll bool) bool {
	return !showAll && len(filtered.Workloads) == 0 && len(full.Workloads) > 0
}

func buildClusterScaleStatusTextBlocks(workloads []ClusterScaleWorkloadStatus) [][]scaleStatusTextRow {
	blocks := make([][]scaleStatusTextRow, 0, len(workloads))
	for _, wl := range workloads {
		rows := make([]scaleStatusTextRow, 0, 1+len(wl.Dependencies))
		rows = append(rows, scaleStatusTextRow{
			col1:           string(wl.WorkloadId),
			state:          wl.State,
			ready:          wl.Ready,
			readyMessages:  wl.ReadyMessages,
			isRootWorkload: true,
		})
		for _, dep := range wl.Dependencies {
			opt := ""
			if dep.Optional {
				opt = "  optional"
			}
			tag := scaleDependencyRefFlowTag(dep.RefRole)
			col1 := "    " + string(dep.WorkloadId) + opt
			if tag != "" {
				col1 = "    " + tag + " " + string(dep.WorkloadId) + opt
			}
			rows = append(rows, scaleStatusTextRow{
				col1:          col1,
				state:         dep.State,
				ready:         dep.Ready,
				readyMessages: dep.ReadyMessages,
				refFlowTag:    tag,
			})
		}
		blocks = append(blocks, rows)
	}
	return blocks
}

func measureClusterScaleStatusTextWidths(blocks [][]scaleStatusTextRow) (maxCol1, maxSync int, hasReady bool) {
	for _, rows := range blocks {
		for _, r := range rows {
			if len(r.col1) > maxCol1 {
				maxCol1 = len(r.col1)
			}
			sl := formatScaleStatusStateForText(r.state)
			if len(sl) > maxSync {
				maxSync = len(sl)
			}
			if r.ready != "" {
				hasReady = true
			}
		}
	}
	return maxCol1, maxSync, hasReady
}

// WriteClusterScaleStatusText writes the default human-oriented status report (optionally with ANSI colors).
// Workloads are grouped by AppId (sorted by app then workload id). Each group starts with an app header line
// (bold light magenta when color is enabled). Each scale target forms a block (root row + dependency rows) with columns:
// workload id, sync state, ready. Column widths are the same for the whole report (all blocks use one global alignment).
func WriteClusterScaleStatusText(w io.Writer, report ClusterScaleStatusReport, useColor htypes.Color) error {
	workloads := slices.Clone(report.Workloads)
	sortScaleStatusWorkloadsByAppThenWorkload(workloads)
	blocks := buildClusterScaleStatusTextBlocks(workloads)
	maxCol1, maxSync, hasReady := measureClusterScaleStatusTextWidths(blocks)

	var lastApp htypes.AppId
	var haveLastApp bool
	for i, rows := range blocks {
		appID := workloads[i].AppId
		if !haveLastApp || appID != lastApp {
			label := scaleStatusAppHeaderLabel(appID)
			if bool(useColor) {
				if _, err := fmt.Fprintf(w, "%s%s%s\n",
					colors.BoldLightMagenta(),
					label,
					colors.Reset.String(),
				); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintln(w, label); err != nil {
				return err
			}
			lastApp = appID
			haveLastApp = true
		}
		if err := writeClusterScaleStatusTextBlock(w, rows, useColor, maxCol1, maxSync, hasReady); err != nil {
			return err
		}
		if i < len(blocks)-1 {
			if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		}
	}
	if len(blocks) > 0 {
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writeClusterScaleStatusTextBlock(
	w io.Writer,
	rows []scaleStatusTextRow,
	useColor htypes.Color,
	maxCol1, maxSync int,
	hasReady bool,
) error {
	for _, r := range rows {
		syncLabel := formatScaleStatusStateForText(r.state)
		if bool(useColor) && r.isRootWorkload {
			pad := maxCol1 - len(r.col1)
			if pad < 0 {
				pad = 0
			}
			if _, err := fmt.Fprintf(w, "%s%s%s%s",
				colors.BoldWhite(),
				r.col1,
				colors.Reset.String(),
				strings.Repeat(" ", pad),
			); err != nil {
				return err
			}
		} else if bool(useColor) && r.refFlowTag != "" {
			// "    " + tag + " " + rest — tag only in light blue (visible width must match r.col1)
			rest := string(r.col1)
			if len(rest) >= 6 && rest[:4] == "    " {
				rest = rest[4:]
				if strings.HasPrefix(rest, r.refFlowTag+" ") {
					rest = rest[len(r.refFlowTag)+1:]
				}
			}
			prefix := "    "
			tagPart := r.refFlowTag + " "
			pad := maxCol1 - len(prefix) - len(tagPart) - len(rest)
			if pad < 0 {
				pad = 0
			}
			if _, err := fmt.Fprintf(w, "%s%s%s%s%s%s",
				prefix,
				colors.LightBlue.String(),
				r.refFlowTag,
				colors.Reset.String(),
				" ",
				rest,
			); err != nil {
				return err
			}
			if _, err := fmt.Fprint(w, strings.Repeat(" ", pad)); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "%-*s", maxCol1, r.col1); err != nil {
			return err
		}
		if useColor {
			pad := maxSync - len(syncLabel)
			if pad < 0 {
				pad = 0
			}
			if _, err := fmt.Fprintf(w, "  %s%s%s%s",
				scaleStatusStateColor(r.state).String(),
				syncLabel,
				colors.Reset.String(),
				strings.Repeat(" ", pad),
			); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintf(w, "  %-*s", maxSync, syncLabel); err != nil {
			return err
		}
		if hasReady {
			if r.ready != "" {
				if useColor {
					if _, err := fmt.Fprintf(w, "  %s%s%s\n",
						scaleReadyLabelColor(r.ready).String(),
						r.ready,
						colors.Reset.String(),
					); err != nil {
						return err
					}
				} else if _, err := fmt.Fprintf(w, "  %s\n", r.ready); err != nil {
					return err
				}
			} else if _, err := fmt.Fprintln(w); err != nil {
				return err
			}
		} else if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
		if r.ready == ClusterScaleReadyNotReady && len(r.readyMessages) > 0 {
			for _, msg := range r.readyMessages {
				if useColor {
					if _, err := fmt.Fprintf(w, "  %s- %s%s\n",
						colors.Red.String(),
						msg,
						colors.Reset.String(),
					); err != nil {
						return err
					}
				} else if _, err := fmt.Fprintf(w, "  - %s\n", msg); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func liveMatchesScaledDownState(target ScaleTarget, live *unstructured.Unstructured) bool {
	if live == nil {
		return false
	}
	switch {
	case target.IsCustomWorkload:
		for _, path := range target.ReplicaPaths {
			if currentCustomReplicaValue(live.Object, path) != 0 {
				return false
			}
		}
		return true
	case target.IsJob:
		suspended, _ := values.Lookup(live.Object, "spec", "suspend").(bool)
		return suspended
	case target.IsDaemonSet:
		currentNS := nodeSelectorFromObject(live)
		return len(currentNS) == 1 && currentNS[hydra.AnnotationHydraScaleDisabled] == "true"
	default:
		currentReplicas := int64(1)
		if r := values.Lookup(live.Object, "spec", "replicas"); r != nil {
			currentReplicas, _ = toInt64(r)
		}
		return currentReplicas == 0
	}
}

func liveMatchesTemplateScaleState(target ScaleTarget, live *unstructured.Unstructured) bool {
	if live == nil {
		return false
	}
	switch {
	case target.IsCustomWorkload:
		for path, expected := range target.OriginalReplicas {
			if currentCustomReplicaValue(live.Object, path) != expected {
				return false
			}
		}
		return true
	case target.IsJob:
		suspended, _ := values.Lookup(live.Object, "spec", "suspend").(bool)
		return !suspended
	case target.IsDaemonSet:
		current := nodeSelectorFromObject(live)
		currentExists := nodeSelectorFieldExists(live)
		targetExists := target.NodeSelector != nil
		return currentExists == targetExists && nodeSelectorEqual(current, target.NodeSelector)
	default:
		replicas, ok := toInt64(values.Lookup(live.Object, "spec", "replicas"))
		if !ok || replicas != target.Replicas {
			return false
		}
		return true
	}
}

func entityIsV1SecretOrConfigMap(e entity.Entity) bool {
	gvk, err := e.GVKString()
	if err != nil {
		return false
	}
	return gvk == htypes.KubernetesGvkV1Secret || gvk == htypes.KubernetesGvkV1ConfigMap
}

// upgradeMissingJobToOKIfDepsSatisfied turns a **missing** batch Job into sync state **ok** when it has at least
// one out-like dependency (RefRole produces or downstream) and every dependency row is up+ready.
func upgradeMissingJobToOKIfDepsSatisfied(target ScaleTarget, state string, deps []ClusterScaleWorkloadDependencyStatus) string {
	if state != ClusterScaleWorkloadStateMissing || !target.IsJob || len(deps) == 0 {
		return state
	}
	outLike := false
	for _, d := range deps {
		if d.RefRole == ScaleDependencyRefRoleProduces || d.RefRole == ScaleDependencyRefRoleDownstream {
			outLike = true
			break
		}
	}
	if !outLike {
		return state
	}
	for _, d := range deps {
		if !workloadRowSyncAndReadyOK(d.State, d.Ready) {
			return state
		}
	}
	return ClusterScaleWorkloadStateOK
}

func classifyWorkloadScaleSyncState(target ScaleTarget, e entity.Entity, templateKey, liveKey htypes.EntityKeyUnstructured) string {
	live, ok := e.Unstructured(liveKey)
	if !ok {
		if target.IsJob {
			return ClusterScaleWorkloadStateMissing
		}
		return ClusterScaleWorkloadStateOutOfSync
	}
	livePtr := live
	if liveMatchesTemplateScaleState(target, &livePtr) {
		return ClusterScaleWorkloadStateUp
	}
	if liveMatchesScaledDownState(target, &livePtr) {
		return ClusterScaleWorkloadStateDown
	}
	return ClusterScaleWorkloadStateOutOfSync
}

func classifyDependencySyncState(
	targets []ScaleTarget,
	depID htypes.Id,
	depEntity entity.Entity,
	templateKey, liveKey htypes.EntityKeyUnstructured,
) string {
	depTarget, tok := findScaleTargetByID(targets, depID)
	if !tok {
		// Not a scale target (e.g. Pod): no template replica sync — show as up when the object exists in the cluster.
		if _, ok := depEntity.Unstructured(liveKey); ok {
			return ClusterScaleWorkloadStateUp
		}
		if entityIsV1SecretOrConfigMap(depEntity) {
			return ClusterScaleWorkloadStateMissing
		}
		return ClusterScaleWorkloadStateDown
	}
	return classifyWorkloadScaleSyncState(depTarget, depEntity, templateKey, liveKey)
}

type workloadDepEdgeKey struct {
	from htypes.Id
	to   htypes.Id
}

// collectPodEntityIds returns entity ids whose GVK is core v1 Pod (refs may point at standalone pods).
func collectPodEntityIds(em entity.EntityMap) map[htypes.Id]bool {
	out := make(map[htypes.Id]bool)
	for id, e := range em {
		gvk, err := e.GVKString()
		if err != nil {
			continue
		}
		if gvk == htypes.KubernetesGvkV1Pod {
			out[id] = true
		}
	}
	return out
}

// directWorkloadDependenciesFromRefs returns direct dependency edges derived from refs
// (after [ResolveTransitiveWorkloadDeps]), with optional:startup merged per edge (required wins).
// Edges are kept when From is a scale target and To is a scale target or a Pod listed in podIds.
func directWorkloadDependenciesFromRefs(refs []htypes.Ref, workloadIds, podIds map[htypes.Id]bool) map[htypes.Id][]struct {
	to       htypes.Id
	optional bool
} {
	optionalByEdge := make(map[workloadDepEdgeKey]bool)
	for _, ref := range refs {
		from := htypes.Id(ref.From)
		to := htypes.Id(ref.To)
		if ref.Reverse {
			from, to = to, from
		}
		if from == to {
			continue
		}
		if !workloadIds[from] || (!workloadIds[to] && !podIds[to]) {
			continue
		}
		e := workloadDepEdgeKey{from: from, to: to}
		opt := ref.HasTag(htypes.RefTagOptionalStartup)
		if prev, ok := optionalByEdge[e]; !ok {
			optionalByEdge[e] = opt
		} else if prev && !opt {
			optionalByEdge[e] = false
		}
	}

	out := make(map[htypes.Id][]struct {
		to       htypes.Id
		optional bool
	})
	for e, optional := range optionalByEdge {
		out[e.from] = append(out[e.from], struct {
			to       htypes.Id
			optional bool
		}{to: e.to, optional: optional})
	}
	for from := range out {
		sort.Slice(out[from], func(i, j int) bool {
			return out[from][i].to < out[from][j].to
		})
	}
	return out
}

// ComputeClusterScaleWorkloadStatusReport classifies each scale target in merged template+live entities.
// refsSync drives workload→workload sync dependencies (ResolveTransitiveWorkloadDeps + direct edges).
// refsFull drives transitive outgoing reach for ready-oriented rows (same hop cap as inspect when refsFull is the cluster-tree graph).
func ComputeClusterScaleWorkloadStatusReport(
	scaleEntities entity.Entities,
	refsSync []htypes.Ref,
	refsFull []htypes.Ref,
	templateKey htypes.EntityKeyUnstructured,
	liveKey htypes.EntityKeyUnstructured,
	eval *ReadyEvaluator,
	customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup,
) (ClusterScaleStatusReport, error) {
	if refsFull == nil {
		refsFull = refsSync
	}
	targets, err := CollectScaleTargets(scaleEntities, templateKey, customWorkloads...)
	if err != nil {
		return ClusterScaleStatusReport{}, err
	}
	if len(targets) == 0 {
		return ClusterScaleStatusReport{Workloads: nil}, nil
	}

	workloadIds := make(map[htypes.Id]bool, len(targets))
	for _, t := range targets {
		workloadIds[t.Id] = true
	}
	podIds := collectPodEntityIds(scaleEntities.EntityMap)

	enrichedRefs := ResolveTransitiveWorkloadDeps(refsSync, workloadIds)
	depByFrom := directWorkloadDependenciesFromRefs(enrichedRefs, workloadIds, podIds)

	sort.Slice(targets, func(i, j int) bool {
		return targets[i].Id < targets[j].Id
	})

	report := ClusterScaleStatusReport{
		Workloads: make([]ClusterScaleWorkloadStatus, 0, len(targets)),
	}

	for _, target := range targets {
		e, ok := scaleEntities.EntityMap[target.Id]
		if !ok {
			return ClusterScaleStatusReport{}, fmt.Errorf("scale status: missing entity for workload %s", target.Id)
		}
		state := classifyWorkloadScaleSyncState(target, e, templateKey, liveKey)

		var rootReady string
		var rootReadyMessages []string
		if eval != nil && state != ClusterScaleWorkloadStateMissing {
			if matched, rs, rmsgs, _ := eval.ReadyState(e, liveKey); matched {
				rootReady = rs
				rootReadyMessages = rmsgs
			}
		}

		optionalByTo := map[htypes.Id]bool{}
		for _, d := range depByFrom[target.Id] {
			if prev, exists := optionalByTo[d.to]; !exists {
				optionalByTo[d.to] = d.optional
			} else if prev && !d.optional {
				optionalByTo[d.to] = false
			}
		}

		depSeen := map[htypes.Id]bool{}
		var depOrder []htypes.Id
		addDep := func(id htypes.Id) {
			if depSeen[id] {
				return
			}
			depSeen[id] = true
			depOrder = append(depOrder, id)
		}

		for _, d := range depByFrom[target.Id] {
			addDep(d.to)
		}
		for _, id := range TransitiveOutgoingRefReach(refsFull, target.Id) {
			depEnt, dok := scaleEntities.EntityMap[id]
			if !dok {
				continue
			}
			if podIds[id] {
				addDep(id)
				continue
			}
			if eval != nil && eval.RuleMatched(depEnt, liveKey) {
				addDep(id)
			}
		}
		depStatuses := make([]ClusterScaleWorkloadDependencyStatus, 0, len(depOrder))
		for _, depID := range depOrder {
			depEntity, eok := scaleEntities.EntityMap[depID]
			if !eok {
				return ClusterScaleStatusReport{}, fmt.Errorf("scale status: missing entity for dependency %s", depID)
			}
			depState := classifyDependencySyncState(targets, depID, depEntity, templateKey, liveKey)
			opt := optionalByTo[depID]
			var depReady string
			var depReadyMessages []string
			if eval != nil && depState != ClusterScaleWorkloadStateMissing {
				if matched, rs, dmsgs, _ := eval.ReadyState(depEntity, liveKey); matched {
					depReady = rs
					depReadyMessages = dmsgs
				}
			}
			depGVK, gvkErr := depEntity.GVKString()
			if gvkErr != nil {
				depGVK = ""
			}
			depStatuses = append(depStatuses, ClusterScaleWorkloadDependencyStatus{
				WorkloadId:    depID,
				Optional:      opt,
				RefRole:       scaleDependencyRefRole(target.Id, depID, refsFull, depGVK),
				State:         depState,
				Ready:         depReady,
				ReadyMessages: depReadyMessages,
			})
		}
		slices.SortFunc(depStatuses, func(a, b ClusterScaleWorkloadDependencyStatus) int {
			if c := refRoleSortRank(a.RefRole) - refRoleSortRank(b.RefRole); c != 0 {
				return c
			}
			return strings.Compare(string(a.WorkloadId), string(b.WorkloadId))
		})

		state = upgradeMissingJobToOKIfDepsSatisfied(target, state, depStatuses)

		report.Workloads = append(report.Workloads, ClusterScaleWorkloadStatus{
			AppId:         primaryAppIdFromEntity(e),
			WorkloadId:    target.Id,
			State:         state,
			Ready:         rootReady,
			ReadyMessages: rootReadyMessages,
			Dependencies:  depStatuses,
		})
	}

	sortScaleStatusWorkloadsByAppThenWorkload(report.Workloads)
	return report, nil
}

func findScaleTargetByID(targets []ScaleTarget, id htypes.Id) (ScaleTarget, bool) {
	for _, t := range targets {
		if t.Id == id {
			return t, true
		}
	}
	return ScaleTarget{}, false
}
