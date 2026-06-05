package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterValuesFlags configures hydra gitops values.
type ClusterValuesFlags struct {
	flags.ContextFlag
	flags.AppIdFlag
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.NoCacheFlag
	flags.KubernetesVersionFlag
}

var _ flags.WithContextFlag = (*ClusterValuesFlags)(nil)
var _ flags.WithAppIdFlag = (*ClusterValuesFlags)(nil)
var _ flags.WithColorFlag = (*ClusterValuesFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ClusterValuesFlags)(nil)
var _ flags.WithNoCacheFlag = (*ClusterValuesFlags)(nil)

func (f *ClusterValuesFlags) Flags() flags.Flags { return f }

// ClusterValues prints merged Helm values for one app, with global.hydra merged from Helm plus every Hydra
// ConfigMap data.hydra fragment on the cluster (same merge semantics as hydra local config, but using a
// full-cluster render catalog so other apps' chart-owned ConfigMaps participate). Uses the same
// [commands.ClusterHelmInputValuesMap] path as hydra gitops template for Helm chart input values.
func ClusterValues(f ClusterValuesFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	l.Info(logIdAction, "Rendering cluster values for App '{app}'", log.String("app", string(f.AppId)))

	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)

	h, err := hydra.ResolvePathWithAppId(l, f.HydraContext, f.AppId, config)
	if err != nil {
		return nil, "", err
	}

	cluster := clusterFromHydraApp(h)
	if cluster == nil {
		return nil, "", log.CreateError(errors.ErrInvalidHydraStructure, "hydra gitops values could not resolve cluster")
	}
	if err := commands.ValidateAppIdInCluster(cluster, f.AppId, f.HelmNetworkMode); err != nil {
		return nil, "", err
	}

	allAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return nil, "", err
	}

	skipRootApps := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	renderAppIds := allAppIds
	if skipRootApps {
		renderAppIds = sets.New[types.AppId]()
		for id := range allAppIds {
			if !id.IsRootApp() {
				renderAppIds.Insert(id)
			}
		}
	}

	renderOpts := []commands.RenderClusterSelectedAppsOption{commands.WithSkipFoundDefinitionsInfoLog()}

	fullClusterRender, _, err := commands.RenderClusterAllApps(
		cluster,
		f.HelmNetworkMode,
		f.KubernetesVersion,
		types.KeyTemplateEntity,
		skipRootApps,
		renderOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	fullRender, err := commands.RenderClusterSelectedApps(
		cluster,
		f.HelmNetworkMode,
		f.KubernetesVersion,
		renderAppIds,
		types.KeyTemplateEntity,
		renderOpts...,
	)
	if err != nil {
		return nil, "", err
	}

	mergedPerApp, err := commands.PrepareClusterHelmMergedHydraMaps(
		cluster, renderAppIds, f.HelmNetworkMode, fullRender, fullClusterRender)
	if err != nil {
		return nil, "", err
	}
	mh, ok := mergedPerApp[f.AppId]
	if !ok {
		return nil, "", log.CreateError(errors.ErrInternalError, "missing merged hydra for app {appId}",
			log.String("appId", string(f.AppId)))
	}

	out, err := commands.ClusterHelmInputValuesMap(h, f.HelmNetworkMode, mh)
	if err != nil {
		return nil, "", err
	}

	result, err := yq.ToYaml(f.Color, out)
	if err != nil {
		return nil, "", err
	}
	return h, result, nil
}
