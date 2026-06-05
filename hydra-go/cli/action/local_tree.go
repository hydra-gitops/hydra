package action

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/cli/tui"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// LocalTreeFlags configures hydra local inspect.
type LocalTreeFlags struct {
	flags.ContextFlag
	flags.HelmNetworkModeFlag
	flags.NoCacheFlag
	Cluster    types.ClusterName
	ResourceId types.Id
}

var _ flags.WithContextFlag = (*LocalTreeFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*LocalTreeFlags)(nil)

func (f *LocalTreeFlags) Flags() flags.Flags {
	return f
}

// LocalTree runs the interactive reference inspect TUI using locally rendered templates only.
func LocalTree(f LocalTreeFlags) error {
	if f.ResourceId != "" {
		if _, _, _, _, _, err := f.ResourceId.Components(); err != nil {
			return log.CreateError(errors.ErrHydraConfigError, "invalid resource id",
				log.String("id", string(f.ResourceId)),
				log.Err(err))
		}
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

	graph, err := commands.LoadLocalTreeGraph(cluster, f.HelmNetworkMode)
	if err != nil {
		return err
	}

	implicitPicker := f.ResourceId == ""
	for {
		if f.ResourceId == "" {
			ids := graph.CandidateIds()
			if len(ids) == 0 {
				l.Info(logIdAction, "no resource ids found for local inspect")
				return nil
			}
			picked, err := tui.RunIDPicker("Select resource (local templates)", ids, nil)
			if err != nil {
				return err
			}
			if picked == "" {
				return nil
			}
			f.ResourceId = picked
		}

		if err := graph.EnsureStartId(f.ResourceId); err != nil {
			return err
		}

		back, err := tui.RunRefTree(graph.Refs, f.ResourceId, implicitPicker, false)
		if err != nil {
			return err
		}
		if !back {
			return nil
		}
		f.ResourceId = ""
	}
}
