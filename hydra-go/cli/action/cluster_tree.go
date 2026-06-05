package action

import (
	"context"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/cli/tui"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterTreeFlags configures hydra gitops inspect.
type ClusterTreeFlags struct {
	flags.ContextFlag
	flags.HelmNetworkModeFlag
	flags.ClusterFlag
	flags.BootstrapFlag
	flags.NoCacheFlag
	ResourceId types.Id
}

var _ flags.WithContextFlag = (*ClusterTreeFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ClusterTreeFlags)(nil)
var _ flags.WithClusterFlag = (*ClusterTreeFlags)(nil)
var _ flags.WithBootstrapFlag = (*ClusterTreeFlags)(nil)

func (f *ClusterTreeFlags) WithClusterFlag() *flags.ClusterFlag {
	return &f.ClusterFlag
}

func (f *ClusterTreeFlags) WithBootstrapFlag() *flags.BootstrapFlag {
	return &f.BootstrapFlag
}

func (f *ClusterTreeFlags) Flags() flags.Flags {
	return f
}

// ClusterTree runs the interactive reference inspect TUI using rendered templates plus live cluster data.
func ClusterTree(f ClusterTreeFlags) error {
	if f.ResourceId != "" {
		if _, _, _, _, _, err := f.ResourceId.Components(); err != nil {
			return log.CreateError(errors.ErrHydraConfigError, "invalid resource id",
				log.String("id", string(f.ResourceId)),
				log.Err(err))
		}
	}

	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)
	l := log.Default()

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       hydra.RESTClientLimits{},
		ClusterName:  f.Cluster,
	})
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	appIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return err
	}
	scopeInfo, err := commands.ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
	if err != nil {
		return err
	}
	templateEnts, err := commands.RenderClusterSelectedApps(cluster, f.HelmNetworkMode, "", appIds, types.KeyTemplateEntity)
	if err != nil {
		return err
	}
	templateEnts, err = commands.NormalizeApiVersions(cluster.L(), templateEnts, types.KeyTemplateEntity, cluster, func() (types.ScopeInfoMap, error) {
		return scopeInfo, nil
	})
	if err != nil {
		return err
	}
	clusterEnts, err := commands.ListClusterAll(cluster, types.KeyClusterEntity, false, 0)
	if err != nil {
		return err
	}
	invModel, err := commands.BuildResourceModel(commands.ResourceModelInput{
		Cluster:          cluster,
		NetworkMode:      f.HelmNetworkMode,
		Bootstrap:        f.Bootstrap,
		TemplateEntities: &templateEnts,
		ClusterEntities:  &clusterEnts,
		AppIds:           appIds,
		ScopeInfo:        scopeInfo,
		WatchCtx:         ctx,
	}, false)
	if err != nil {
		return err
	}
	defer invModel.Close()
	templateEnts = invModel.TemplateEntities()
	clusterEnts = invModel.ClusterEntities()

	loadGraph := func() (*commands.ClusterTreeGraph, error) {
		inspectGraph, err := commands.LoadInspectRefGraph(commands.InspectRefGraphParams{
			Inventory:                   invModel.InventoryForGraph(),
			Cluster:                     cluster,
			NetworkMode:                 f.HelmNetworkMode,
			Bootstrap:                   f.Bootstrap,
			IncludeTemplateRefs:         true,
			IncludeClusterRefs:          true,
			IncludeCloneMaterialization: true,
		})
		if err != nil {
			return nil, err
		}
		return &commands.ClusterTreeGraph{
			Refs:         inspectGraph.Refs,
			TemplateEnts: templateEnts,
			ClusterEnts:  clusterEnts,
		}, nil
	}

	graph, err := loadGraph()
	if err != nil {
		return err
	}

	implicitPicker := f.ResourceId == ""
	for {
		if f.ResourceId == "" {
			ids := graph.CandidateIds()
			if len(ids) == 0 {
				l.Info(logIdAction, "no resource ids found for cluster inspect")
				return nil
			}
			picked, err := tui.RunIDPicker("Select resource (templates + live cluster)", ids, graph.PickerRowStatuses(ids))
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

		back, err := tui.RunRefTree(graph.Refs, f.ResourceId, implicitPicker, true)
		if err != nil {
			return err
		}
		if !back {
			return nil
		}
		graph, err = loadGraph()
		if err != nil {
			return err
		}
		f.ResourceId = ""
	}
}
