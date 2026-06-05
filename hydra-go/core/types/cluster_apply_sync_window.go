package types

import (
	"fmt"
	"strings"
)

// ClusterApplySyncWindow selects how ArgoCD AppProject syncWindows are adjusted during
// `hydra gitops apply` for resources in the workload apply set.
type ClusterApplySyncWindow string

const (
	ClusterApplySyncWindowDefault       ClusterApplySyncWindow = "default"
	ClusterApplySyncWindowManual        ClusterApplySyncWindow = "manual"
	ClusterApplySyncWindowAuto          ClusterApplySyncWindow = "auto"
	ClusterApplySyncWindowPrevent       ClusterApplySyncWindow = "prevent"
	ClusterApplySyncWindowKeepOrManual  ClusterApplySyncWindow = "keep-or-manual"
	ClusterApplySyncWindowKeepOrAuto    ClusterApplySyncWindow = "keep-or-auto"
	ClusterApplySyncWindowKeepOrPrevent ClusterApplySyncWindow = "keep-or-prevent"
	ClusterApplySyncWindowKeepOrDefault ClusterApplySyncWindow = "keep-or-default"
)

var clusterApplySyncWindowValues = map[string]ClusterApplySyncWindow{
	"default":         ClusterApplySyncWindowDefault,
	"manual":          ClusterApplySyncWindowManual,
	"auto":            ClusterApplySyncWindowAuto,
	"prevent":         ClusterApplySyncWindowPrevent,
	"keep-or-manual":  ClusterApplySyncWindowKeepOrManual,
	"keep-or-auto":    ClusterApplySyncWindowKeepOrAuto,
	"keep-or-prevent": ClusterApplySyncWindowKeepOrPrevent,
	"keep-or-default": ClusterApplySyncWindowKeepOrDefault,
}

// legacyClusterApplySyncWindowAliases maps deprecated CLI spellings to canonical modes.
var legacyClusterApplySyncWindowAliases = map[string]ClusterApplySyncWindow{
	"deny":         ClusterApplySyncWindowManual,
	"keep-or-deny": ClusterApplySyncWindowKeepOrManual,
}

// ParseClusterApplySyncWindow parses and validates a --sync value.
func ParseClusterApplySyncWindow(s string) (ClusterApplySyncWindow, error) {
	key := strings.TrimSpace(strings.ToLower(s))
	if key == "" {
		return "", fmt.Errorf("sync mode must not be empty")
	}
	if m, ok := clusterApplySyncWindowValues[key]; ok {
		return m, nil
	}
	if m, ok := legacyClusterApplySyncWindowAliases[key]; ok {
		return m, nil
	}
	return "", fmt.Errorf("invalid --sync %q: want default|manual|auto|prevent|keep-or-manual|keep-or-auto|keep-or-prevent|keep-or-default", s)
}

// ClusterSyncKindManual returns ArgoCD sync window kind and manualSync for modes that match
// `hydra gitops sync` (auto / manual-as-yellow / prevent).
func (m ClusterApplySyncWindow) ClusterSyncKindManual() (kind string, manualSync bool, ok bool) {
	switch m {
	case ClusterApplySyncWindowAuto:
		return "allow", true, true
	case ClusterApplySyncWindowManual:
		return "deny", true, true
	case ClusterApplySyncWindowPrevent:
		return "deny", false, true
	default:
		return "", false, false
	}
}

// KeepOrNewKindManual returns kind and manualSync for newly created AppProjects in keep-or-* modes.
// useTemplate is true for keep-or-default (leave template syncWindows unchanged).
func (m ClusterApplySyncWindow) KeepOrNewKindManual() (kind string, manualSync bool, useTemplate bool) {
	switch m {
	case ClusterApplySyncWindowKeepOrManual:
		return "deny", true, false
	case ClusterApplySyncWindowKeepOrAuto:
		return "allow", true, false
	case ClusterApplySyncWindowKeepOrPrevent:
		return "deny", false, false
	case ClusterApplySyncWindowKeepOrDefault:
		return "", false, true
	default:
		return "", false, false
	}
}
