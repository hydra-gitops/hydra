package action

import (
	"fmt"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
)

// LocalRefsFlags configures hydra local refs.
type LocalRefsFlags struct {
	flags.ContextFlag
	flags.ColorFlag
	flags.HelmNetworkModeFlag
	flags.NoCacheFlag
	Cluster    types.ClusterName
	ResourceId types.Id
}

var _ flags.WithContextFlag = (*LocalRefsFlags)(nil)
var _ flags.WithColorFlag = (*LocalRefsFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*LocalRefsFlags)(nil)

func (f *LocalRefsFlags) Flags() flags.Flags {
	return f
}

// LocalRefs prints transitive reference reachability for ResourceId on the given cluster as YAML.
func LocalRefs(f LocalRefsFlags) error {
	if _, _, _, _, _, err := f.ResourceId.Components(); err != nil {
		return log.CreateError(errors.ErrHydraConfigError, "invalid resource id",
			log.String("id", string(f.ResourceId)),
			log.Err(err))
	}

	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)
	l := log.Default()

	context, err := hydra.NewContext(l, types.ContextPath(f.HydraContext), config)
	if err != nil {
		return err
	}

	cluster, err := hydra.NewCluster(context, f.Cluster, hydra.RESTClientLimits{})
	if err != nil {
		return err
	}

	rows, err := commands.ClusterRefsTransitive(cluster, f.HelmNetworkMode, f.ResourceId)
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
