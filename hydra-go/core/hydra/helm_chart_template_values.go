package hydra

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
)

// helmChartValuesForTemplate returns the ValuesMap passed to helm.Template for this app.
// Values must be the same raw user-supplied shape Helm's install action coalesces once
// ([helm.CoalescedValuesMapBeforeRender] / child [ChildApp.MergedChildValuesForHelmInstall]),
// not the post-ToRenderValues output of [HydraApp.LoadValuesMap].
// When a cluster-wide hook is active, the hook provides values (merged cluster ConfigMaps + same shape).
func helmChartValuesForTemplate(h HydraApp, networkMode types.HelmNetworkMode) (types.ValuesMap, error) {
	// ChildApp embeds *RootApp; [HydraApp.AsRootApp] is non-nil for children — check child before root
	// (same ordering as [ChartDirectoryForHydraApp]).
	ca := h.AsChildApp()
	if ca != nil {
		var vals types.ValuesMap
		var err error
		if ca.Cluster != nil {
			if v, hook, hookErr := ca.Cluster.helmChartInputValues(ca.AppId()); hook {
				vals, err = v, hookErr
				if err != nil {
					return nil, err
				}
				replicateParentGlobalIntoChildDependencyValues(vals, string(ca.ChildAppName))
				return vals, nil
			}
		}
		vals, err = ca.MergedChildValuesForHelmInstall(networkMode)
		if err != nil {
			return nil, err
		}
		replicateParentGlobalIntoChildDependencyValues(vals, string(ca.ChildAppName))
		return vals, nil
	}

	if ra := h.AsRootApp(); ra != nil {
		if ra.Cluster != nil {
			if v, ok, err := ra.Cluster.helmChartInputValues(ra.AppId()); ok {
				return v, err
			}
		}
		vals, err := ra.Cluster.LoadValuesMap(networkMode)
		if err != nil {
			return nil, err
		}
		return values.MergeValues(vals, types.ValuesMap{
			"global": types.ValuesMap{
				"hydra": types.ValuesMap{
					"cluster": string(ra.ClusterName),
				},
			},
		}), nil
	}

	return nil, log.CreateError(errors.ErrInvalidHydraStructure, "helmChartValuesForTemplate requires a root or child app")
}

// helmChartTemplateValuesDigest returns a hex-encoded SHA256 over the YAML of the ValuesMap passed
// to helm.Template, so in-memory template caches invalidate when merged Helm values change.
func helmChartTemplateValuesDigest(h HydraApp, networkMode types.HelmNetworkMode) (string, error) {
	vals, err := helmChartValuesForTemplate(h, networkMode)
	if err != nil {
		return "", err
	}
	// Deep-clone before yaml encoding: yaml.v3 Encode mutates nested maps in the ValuesMap and can
	// tear keys (e.g. dependency subtree) off structures shared with cached umbrella/coalesce maps.
	b, err := json.Marshal(vals)
	if err != nil {
		return "", err
	}
	var clone types.ValuesMap
	if err := json.Unmarshal(b, &clone); err != nil {
		return "", err
	}
	ys, err := yaml.ToYaml(clone)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(ys))
	return hex.EncodeToString(sum[:]), nil
}
