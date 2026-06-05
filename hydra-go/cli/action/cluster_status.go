package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ClusterStatusFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterStatusFlags) Flags() flags.Flags {
	return f
}

var _ flags.WithContextFlag = (*ClusterStatusFlags)(nil)
var _ flags.WithColorFlag = (*ClusterStatusFlags)(nil)
var _ flags.WithExcludeAppFlag = (*ClusterStatusFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ClusterStatusFlags)(nil)

var clusterStatusResolveAppIdsFromConfig = commands.ResolveAppIdsFromConfig
var clusterStatusClusterForAppIds = func(config types.Config, hydraContext types.HydraContext, appIds sets.Set[types.AppId], limits hydra.RESTClientLimits) (*hydra.Cluster, error) {
	return commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: hydraContext,
		Limits:       limits,
		AppIds:       appIds,
	})
}
var clusterStatusRun = commands.ClusterStatus

func ClusterStatus(f ClusterStatusFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := clusterStatusResolveAppIdsFromConfig(
		l,
		f.HydraContext,
		config,
		f.AppIdPatterns,
		f.ExcludeAppPatterns,
		f.HelmNetworkMode,
		false,
	)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for status")
	}

	cluster, err := clusterStatusClusterForAppIds(config, f.HydraContext, appIds,
		f.ToRESTClientLimits())
	if err != nil {
		return err
	}

	return clusterStatusRun(cluster, f.Color, appIds, f.HelmNetworkMode)
}
