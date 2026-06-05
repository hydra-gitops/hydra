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

// ValuesFlags holds flags specific to the values command
type ValuesFlags struct {
	flags.ContextFlag
	flags.AppIdFlag
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.NoCacheFlag
}

var _ flags.WithContextFlag = (*ValuesFlags)(nil)
var _ flags.WithColorFlag = (*ValuesFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ValuesFlags)(nil)

func (f *ValuesFlags) Flags() flags.Flags {
	return f
}

// Values renders and returns the values for the given context
func Values(f ValuesFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	l.Info(logIdAction, "Rendering values for App '{app}'", log.String("app", string(f.AppId)))

	h, err := hydra.ResolvePathWithAppId(l, f.HydraContext, f.AppId, flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo))
	if err != nil {
		return nil, "", err
	}

	cluster := clusterFromHydraApp(h)
	if cluster == nil {
		return nil, "", log.CreateError(errors.ErrInvalidHydraStructure, "hydra local values could not resolve cluster")
	}
	if err := commands.ValidateAppIdInCluster(cluster, f.AppId, f.HelmNetworkMode); err != nil {
		return nil, "", err
	}

	valuesMap, err := h.LoadValuesMap(f.HelmNetworkMode)
	if err != nil {
		return nil, "", err
	}

	result, err := yq.ToYaml(f.Color, valuesMap)
	if err != nil {
		return nil, "", err
	}

	return h, result, err
}
