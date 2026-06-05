package hydra

import (
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HydraConfigScopeRule is one step in data.hydra scope (Hydra ConfigMaps only).
type HydraConfigScopeRule struct {
	Mode   string   // include or exclude
	Values []string // AppId globs (same semantics as CLI)
}

// ExtractAndRemoveScope parses scope from the hydra YAML map, removes the key for merging into global.hydra,
// and returns validation errors for malformed entries.
func ExtractAndRemoveScope(doc map[string]any, cmID types.Id) ([]HydraConfigScopeRule, error) {
	raw, ok := doc["scope"]
	if !ok || raw == nil {
		return nil, nil
	}
	delete(doc, "scope")

	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("ConfigMap %s data.hydra scope: must be a YAML list", cmID)
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("ConfigMap %s data.hydra scope: list must not be empty", cmID)
	}

	rules := make([]HydraConfigScopeRule, 0, len(list))
	for i, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("ConfigMap %s data.hydra scope[%d]: expected a mapping", cmID, i)
		}
		modeVal, _ := m["mode"].(string)
		mode := strings.ToLower(strings.TrimSpace(modeVal))
		if mode != "include" && mode != "exclude" {
			return nil, fmt.Errorf("ConfigMap %s data.hydra scope[%d].mode must be include or exclude", cmID, i)
		}
		vraw, ok := m["values"]
		if !ok {
			return nil, fmt.Errorf("ConfigMap %s data.hydra scope[%d].values is required", cmID, i)
		}
		valsAny, ok := vraw.([]any)
		if !ok || len(valsAny) == 0 {
			return nil, fmt.Errorf("ConfigMap %s data.hydra scope[%d].values must be a non-empty list", cmID, i)
		}
		vals := make([]string, 0, len(valsAny))
		for j, va := range valsAny {
			s, ok := va.(string)
			if !ok || strings.TrimSpace(s) == "" {
				return nil, fmt.Errorf("ConfigMap %s data.hydra scope[%d].values[%d]: non-empty string required", cmID, i, j)
			}
			vals = append(vals, strings.TrimSpace(s))
		}
		rules = append(rules, HydraConfigScopeRule{Mode: mode, Values: vals})
	}
	return rules, nil
}

// EvaluateHydraScope returns whether targetApp is included after applying rules in order over universe U.
// Empty rules means all apps in universe match.
func EvaluateHydraScope(rules []HydraConfigScopeRule, universe sets.Set[types.AppId], target types.AppId) bool {
	if len(rules) == 0 {
		return true
	}
	S := sets.New[types.AppId]()
	for a := range universe {
		S.Insert(a)
	}
	appList := universe.UnsortedList()

	for _, rule := range rules {
		match := sets.New[types.AppId]()
		for _, aid := range appList {
			for _, pat := range rule.Values {
				if types.MatchAppIdGlob(pat, string(aid)) {
					match.Insert(aid)
					break
				}
			}
		}
		switch rule.Mode {
		case "include":
			next := sets.New[types.AppId]()
			for a := range S {
				if match.Has(a) {
					next.Insert(a)
				}
			}
			S = next
		case "exclude":
			for a := range match {
				S.Delete(a)
			}
		}
	}
	return S.Has(target)
}

// ScopeMatchesAnySelected returns true if CM scope allows at least one app in selectedAppIds (for live inventory CM parsing).
func ScopeMatchesAnySelected(rules []HydraConfigScopeRule, universe, selectedAppIds sets.Set[types.AppId]) bool {
	if len(rules) == 0 {
		return true
	}
	if selectedAppIds.Len() == 0 {
		return false
	}
	for _, aid := range selectedAppIds.UnsortedList() {
		if EvaluateHydraScope(rules, universe, aid) {
			return true
		}
	}
	return false
}
