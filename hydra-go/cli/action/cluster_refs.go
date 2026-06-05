package action

import (
	"fmt"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
)

// ClusterRefsFlags configures hydra gitops refs.
type ClusterRefsFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.ClusterFlag
	flags.BootstrapFlag
	flags.NoCacheFlag
	ResourceId types.Id
}

var _ flags.WithContextFlag = (*ClusterRefsFlags)(nil)
var _ flags.WithColorFlag = (*ClusterRefsFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ClusterRefsFlags)(nil)
var _ flags.WithClusterFlag = (*ClusterRefsFlags)(nil)
var _ flags.WithBootstrapFlag = (*ClusterRefsFlags)(nil)

func (f *ClusterRefsFlags) WithClusterFlag() *flags.ClusterFlag {
	return &f.ClusterFlag
}

func (f *ClusterRefsFlags) WithBootstrapFlag() *flags.BootstrapFlag {
	return &f.BootstrapFlag
}

func (f *ClusterRefsFlags) Flags() flags.Flags {
	return f
}

// ClusterRefs prints transitive reference reachability for ResourceId on the given cluster as YAML.
func ClusterRefs(f ClusterRefsFlags) error {
	if _, _, _, _, _, err := f.ResourceId.Components(); err != nil {
		return log.CreateError(errors.ErrHydraConfigError, "invalid resource id",
			log.String("id", string(f.ResourceId)),
			log.Err(err))
	}

	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		ClusterName:  f.Cluster,
	})
	if err != nil {
		return err
	}

	rows, err := commands.ClusterRefsTransitiveWithCluster(cluster, f.HelmNetworkMode, f.Bootstrap, f.ResourceId)
	if err != nil {
		return err
	}

	data, err := yq.ToYaml(f.Color, rows)
	if err != nil {
		return err
	}
	log.FlushProgressForStdout()
	_, err = fmt.Fprint(os.Stdout, data)
	return err
}
