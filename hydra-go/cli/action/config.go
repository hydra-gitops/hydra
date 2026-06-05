package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
)

// ConfigFlags holds flags for hydra local config (Helm global.hydra only — no Hydra ConfigMaps).
type ConfigFlags struct {
	flags.ContextFlag
	flags.AppIdFlag
	flags.ColorFlag
	flags.NoCacheFlag
}

var _ flags.WithContextFlag = (*ConfigFlags)(nil)
var _ flags.WithColorFlag = (*ConfigFlags)(nil)

func (f *ConfigFlags) Flags() flags.Flags {
	return f
}

// Config renders effective Hydra configuration from Helm values only (`global.hydra`). Hydra ConfigMap
// overlays apply to hydra gitops commands, not hydra local config (see hydra gitops values).
func Config(f ConfigFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	l.Info(logIdAction, "Rendering Hydra config for App Id '{appId}'", log.String("appId", string(f.AppId)))

	h, err := hydra.ResolvePathWithAppId(l, f.HydraContext, f.AppId, flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo))
	if err != nil {
		return nil, "", err
	}

	app := h.AsApp()
	if app == nil {
		return nil, "", log.CreateError(errors.ErrInvalidHydraStructure, "hydra local config requires a root or child app")
	}

	cluster := clusterFromHydraApp(app)
	if cluster == nil {
		return nil, "", log.CreateError(errors.ErrInvalidHydraStructure, "hydra local config could not resolve cluster")
	}

	if err := commands.ValidateAppIdInCluster(cluster, f.AppId, types.HelmNetworkModeOffline); err != nil {
		return nil, "", err
	}

	networkMode := types.HelmNetworkModeOffline

	hydraValues, err := hydra.HydraValues(h, networkMode)
	if err != nil {
		return nil, "", err
	}

	helmHydra, err := hydra.HelmHydraMapFromValues(hydraValues)
	if err != nil {
		return nil, "", err
	}

	var merged types.ValuesMap
	if helmHydra != nil {
		merged = helmHydra
	}

	skipRoot := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	fullClusterRender, _, err := commands.RenderClusterAllApps(cluster, networkMode, "", types.KeyTemplateEntity, skipRoot,
		commands.WithSkipFoundDefinitionsInfoLog())
	if err != nil {
		return nil, "", err
	}
	merged = commands.MergeInferredOwnerNamespacesIntoHydraMap(merged, f.AppId, fullClusterRender)

	out := map[string]any{
		"global": map[string]any{
			"hydra": merged,
		},
	}

	result, err := yq.ToYaml(f.Color, out)
	if err != nil {
		return nil, "", err
	}

	return h, result, err
}

func clusterFromHydraApp(h hydra.HydraApp) *hydra.Cluster {
	if ra := h.AsRootApp(); ra != nil {
		return ra.Cluster
	}
	if ca := h.AsChildApp(); ca != nil {
		return ca.Cluster
	}
	return nil
}
