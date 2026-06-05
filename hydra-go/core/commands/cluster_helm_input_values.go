package commands

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/util/sets"
)

// PrepareClusterHelmMergedHydraMaps returns the effective global.hydra subtree per app: Helm-derived hydra
// merged with Hydra ConfigMap data.hydra (partitioned against fullRender), then ownerNamespaces inference.
func PrepareClusterHelmMergedHydraMaps(
	cluster *hydra.Cluster,
	renderAppIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	fullRender entity.Entities,
	fullClusterRender entity.Entities,
) (map[types.AppId]types.ValuesMap, error) {
	mergedMaps, err := hydra.HydraAppMergedGlobalHydraMaps(cluster, renderAppIds, networkMode, fullRender)
	if err != nil {
		return nil, err
	}
	out := make(map[types.AppId]types.ValuesMap, len(mergedMaps))
	for appId, m := range mergedMaps {
		out[appId] = MergeInferredOwnerNamespacesIntoHydraMap(m, appId, fullClusterRender)
	}
	return out, nil
}

// ClusterHelmInputValuesMap returns Helm chart input values for h: base LoadValuesMap with global.hydra
// replaced by mergedHydra (from [PrepareClusterHelmMergedHydraMaps]), then root-only global.hydra.cluster
// injection matching [RootApp.template] when h is a root app.
func ClusterHelmInputValuesMap(
	h hydra.HydraApp,
	networkMode types.HelmNetworkMode,
	mergedHydra types.ValuesMap,
) (types.ValuesMap, error) {
	vals, err := h.LoadValuesMap(networkMode)
	if err != nil {
		return nil, err
	}

	ys, err := yaml.ToYaml(vals)
	if err != nil {
		return nil, err
	}
	out, err := yaml.FromYaml[types.ValuesMap](ys)
	if err != nil {
		return nil, err
	}

	globalRaw, ok := out["global"]
	if !ok || globalRaw == nil {
		out["global"] = types.ValuesMap{"hydra": mergedHydra}
	} else {
		gys, err := yaml.ToYaml(globalRaw)
		if err != nil {
			return nil, err
		}
		global, err := yaml.FromYaml[types.ValuesMap](gys)
		if err != nil {
			return nil, err
		}
		global["hydra"] = mergedHydra
		out["global"] = global
	}

	if ra := h.AsRootApp(); ra != nil {
		out = values.MergeValues(out, types.ValuesMap{
			"global": types.ValuesMap{
				"hydra": types.ValuesMap{
					"cluster": string(ra.ClusterName),
				},
			},
		})
	}

	return out, nil
}

// ClusterHelmInstallValuesMap returns the ValuesMap that must be passed to helm.Template / Install.Run:
// raw cluster and chart user values with global.hydra replaced by mergedHydra (cluster ConfigMaps merge),
// plus root-only global.hydra.cluster injection. This differs from [ClusterHelmInputValuesMap], which
// reflects post-ToRenderValues merged values for inspection.
func ClusterHelmInstallValuesMap(
	h hydra.HydraApp,
	networkMode types.HelmNetworkMode,
	mergedHydra types.ValuesMap,
) (types.ValuesMap, error) {
	var base types.ValuesMap
	var err error
	switch {
	case h.AsRootApp() != nil:
		base, err = h.AsRootApp().Cluster.LoadValuesMap(networkMode)
	case h.AsChildApp() != nil:
		base, err = h.AsChildApp().MergedChildValuesForHelmInstall(networkMode)
	default:
		return nil, log.CreateError(errors.ErrInvalidHydraStructure, "cluster helm install values require root or child app")
	}
	if err != nil {
		return nil, err
	}

	ys, err := yaml.ToYaml(base)
	if err != nil {
		return nil, err
	}
	out, err := yaml.FromYaml[types.ValuesMap](ys)
	if err != nil {
		return nil, err
	}

	globalRaw, ok := out["global"]
	if !ok || globalRaw == nil {
		out["global"] = types.ValuesMap{"hydra": mergedHydra}
	} else {
		gys, err := yaml.ToYaml(globalRaw)
		if err != nil {
			return nil, err
		}
		global, err := yaml.FromYaml[types.ValuesMap](gys)
		if err != nil {
			return nil, err
		}
		global["hydra"] = mergedHydra
		out["global"] = global
	}

	if ra := h.AsRootApp(); ra != nil {
		out = values.MergeValues(out, types.ValuesMap{
			"global": types.ValuesMap{
				"hydra": types.ValuesMap{
					"cluster": string(ra.ClusterName),
				},
			},
		})
	}

	return out, nil
}
