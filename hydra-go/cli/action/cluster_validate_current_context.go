package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterValidateCurrentContextFlags contains configuration options for check-context operations.
type ClusterValidateCurrentContextFlags struct {
	flags.ContextFlag
	flags.ClusterFlag
	flags.ColorFlag
	flags.NoCacheFlag
}

var _ flags.WithContextFlag = (*ClusterValidateCurrentContextFlags)(nil)
var _ flags.WithClusterFlag = (*ClusterValidateCurrentContextFlags)(nil)
var _ flags.WithColorFlag = (*ClusterValidateCurrentContextFlags)(nil)

func (f *ClusterValidateCurrentContextFlags) Flags() flags.Flags {
	return f
}

// ClusterValidateCurrentContext validates the current Kubernetes context
func ClusterValidateCurrentContext(f ClusterValidateCurrentContextFlags) (hydra.Hydra, string, error) {
	h, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo),
		HydraContext: f.HydraContext,
		Limits:       hydra.RESTClientLimits{},
		ClusterName:  f.Cluster,
	})
	if err != nil {
		return nil, "", err
	}

	cluster := h.AsCluster()
	if cluster == nil {
		return nil, "", log.CreateError(
			errors.ErrHydraContextProblem,
			"expected cluster resolution for validate-current-context")
	}

	_, err = hydra.BuildValidatedKubernetesConfigFlagsForCluster(cluster)
	if err != nil {
		return nil, "", err
	}

	return h, "Context validation successful", nil
}
