package commands

import (
	"context"
	"fmt"
	"sort"
	"time"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ArgocdAppCondition struct {
	Type    string
	Message string
}

type ArgocdAppStatusEntry struct {
	Name         string
	Project      string
	SyncStatus   string
	WindowStatus string
	Conditions   []ArgocdAppCondition
	// OperationPhase is the effective ArgoCD operation phase (e.g. Running, Succeeded) for styling; empty if no operation line.
	OperationPhase string
	OperationLine  string // formatted line for status.operationState; empty if absent
}

func ListArgocdApplicationNames(h hydra.Hydra) ([]string, error) {
	dynamicClient, err := newDynamicClient(h)
	if err != nil {
		return nil, err
	}

	apps, err := dynamicClient.Resource(applicationGVR).Namespace("argocd").List(
		context.Background(), metav1.ListOptions{})
	if err != nil {
		return nil, log.CreateError(errors.ErrSyncWindowFailed,
			"failed to list Applications (argoproj.io/v1alpha1) in namespace 'argocd': {err}",
			log.Err(err))
	}

	names := make([]string, 0, len(apps.Items))
	for _, app := range apps.Items {
		names = append(names, app.GetName())
	}
	sort.Strings(names)
	return names, nil
}

func ResolveArgocdStatusSelection(includePatterns []string, excludePatterns []string, allVisibleApps []string) ([]string, error) {
	selected, err := resolveArgocdSelection(includePatterns, allVisibleApps, true)
	if err != nil {
		return nil, err
	}
	return excludeResolvedAppNames(selected, excludePatterns, allVisibleApps), nil
}

func ResolveArgocdSyncTargets(includePatterns []string, excludePatterns []string, allVisibleApps []string) ([]string, error) {
	selected, err := resolveArgocdSelection(includePatterns, allVisibleApps, false)
	if err != nil {
		return nil, err
	}
	return excludeResolvedAppNames(selected, excludePatterns, allVisibleApps), nil
}

func resolveArgocdSelection(includePatterns []string, allVisibleApps []string, implicitAll bool) ([]string, error) {
	if len(includePatterns) == 0 {
		if !implicitAll {
			return []string{}, nil
		}
		selected := append([]string(nil), allVisibleApps...)
		sort.Strings(selected)
		return deduplicateStrings(selected), nil
	}

	resolved, _, err := ResolvePatterns(includePatterns, allVisibleApps)
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func excludeResolvedAppNames(selected []string, excludePatterns []string, allVisibleApps []string) []string {
	if len(excludePatterns) == 0 {
		return deduplicateStrings(selected)
	}

	selectedSet := map[string]bool{}
	for _, name := range selected {
		selectedSet[name] = true
	}

	for _, pattern := range excludePatterns {
		if !htypes.IsGlobPattern(pattern) {
			delete(selectedSet, pattern)
			continue
		}
		for _, name := range allVisibleApps {
			if selectedSet[name] && htypes.MatchAppIdGlob(pattern, name) {
				delete(selectedSet, name)
			}
		}
	}

	result := make([]string, 0, len(selectedSet))
	for name := range selectedSet {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}

func deduplicateStrings(items []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		if seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func parseApplicationConditions(obj map[string]any) []ArgocdAppCondition {
	raw, found, _ := unstructuredNestedSlice(obj, "status", "conditions")
	if !found || len(raw) == 0 {
		return nil
	}
	out := make([]ArgocdAppCondition, 0, len(raw))
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typ, _ := m["type"].(string)
		msg, _ := m["message"].(string)
		if typ == "" && msg == "" {
			continue
		}
		out = append(out, ArgocdAppCondition{Type: typ, Message: msg})
	}
	return out
}

func parseRFC3339Time(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// formatArgocdOperationStateLine formats status.operationState (sync in progress or last run).
func formatArgocdOperationStateLine(phase, startedAt, finishedAt string) (line string, phaseForColor string) {
	started, startedOK := parseRFC3339Time(startedAt)
	finished, finishedOK := parseRFC3339Time(finishedAt)

	effectivePhase := phase
	if effectivePhase == "" && startedOK && !finishedOK {
		effectivePhase = "Running"
	}
	if effectivePhase == "" && !startedOK && !finishedOK {
		return "", ""
	}
	if effectivePhase == "" {
		effectivePhase = "unknown"
	}
	phaseForColor = effectivePhase

	switch effectivePhase {
	case "Running":
		if startedOK {
			d := time.Since(started).Round(time.Second)
			line = fmt.Sprintf("sync operation: %s (%s elapsed)", effectivePhase, d.String())
			return line, phaseForColor
		}
		line = fmt.Sprintf("sync operation: %s", effectivePhase)
		return line, phaseForColor
	default:
		if startedOK && finishedOK {
			d := finished.Sub(started).Round(time.Second)
			line = fmt.Sprintf("sync operation: %s (last run %s)", effectivePhase, d.String())
			return line, phaseForColor
		}
		line = fmt.Sprintf("sync operation: %s", effectivePhase)
		return line, phaseForColor
	}
}

func parseApplicationOperationState(obj map[string]any) (phaseForColor string, line string) {
	ph, _, _ := unstructuredNestedString(obj, "status", "operationState", "phase")
	startedStr, _, _ := unstructuredNestedString(obj, "status", "operationState", "startedAt")
	finishedStr, _, _ := unstructuredNestedString(obj, "status", "operationState", "finishedAt")
	if ph == "" && startedStr == "" && finishedStr == "" {
		return "", ""
	}
	line, phase := formatArgocdOperationStateLine(ph, startedStr, finishedStr)
	return phase, line
}

func BuildArgocdAppStatusEntries(
	apps []unstructured.Unstructured,
	syncWindowMap map[string]syncWindowState,
) ([]ArgocdAppStatusEntry, error) {
	entries := make([]ArgocdAppStatusEntry, 0, len(apps))
	for _, app := range apps {
		project, _, _ := unstructuredNestedString(app.Object, "spec", "project")
		syncStatus, found, _ := unstructuredNestedString(app.Object, "status", "sync", "status")
		if !found || syncStatus == "" {
			syncStatus = "Unknown"
		}

		windowStatus := "unknown"
		if state, ok := syncWindowMap[project]; ok {
			_, windowStatus = syncWindowColor(state.kind, state.manualSync)
		}

		conditions := parseApplicationConditions(app.Object)
		opPhase, opLine := parseApplicationOperationState(app.Object)

		entries = append(entries, ArgocdAppStatusEntry{
			Name:           app.GetName(),
			Project:        project,
			SyncStatus:     syncStatus,
			WindowStatus:   windowStatus,
			Conditions:     conditions,
			OperationPhase: opPhase,
			OperationLine:  opLine,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

func syncStatusColor(status string) colors.Color {
	switch status {
	case "Synced":
		return colors.Green
	case "OutOfSync":
		return colors.Red
	case "Syncing":
		return colors.Yellow
	default:
		return colors.Yellow
	}
}

func formatSyncStatus(status string) string {
	if status == "" {
		return "unknown"
	}
	return status
}

func applyStatusColor(color htypes.Color, c colors.Color, text string) string {
	if !color {
		return text
	}
	return c.String() + text + colors.Reset.String()
}
