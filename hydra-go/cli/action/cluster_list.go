package action

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterListFlags configures hydra gitops list.
type ClusterListFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ClusterFlag
	flags.PredicatesFlag
	flags.NoCacheFlag
	flags.ClusterListSkipOwnerRefsFlag
}

func (f *ClusterListFlags) Flags() flags.Flags {
	return f
}

func (f *ClusterListFlags) WithClusterListSkipOwnerRefsFlag() *flags.ClusterListSkipOwnerRefsFlag {
	return &f.ClusterListSkipOwnerRefsFlag
}

var _ flags.WithClusterListSkipOwnerRefsFlag = (*ClusterListFlags)(nil)

// ClusterList prints one Hydra resource id per line for resources visible in the cluster.
func ClusterList(f ClusterListFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	l.Info(logIdAction, "listing cluster entity ids for '{cluster}'", log.String("cluster", string(f.Cluster)))

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes),
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		ClusterName:  f.Cluster,
	})
	if err != nil {
		return nil, "", err
	}

	env, err := cel.NewEnv()
	if err != nil {
		return nil, "", err
	}

	predicate, err := env.CompilePredicateAt(`hydra gitops list --predicate`, f.Predicates...)
	if err != nil {
		return nil, "", err
	}

	clusterEntities, err := commands.ListClusterAll(cluster, types.KeyClusterEntity, false, 0)
	if err != nil {
		return nil, "", err
	}
	invModel, err := commands.BuildResourceModel(commands.ResourceModelInput{
		Cluster:         cluster,
		ClusterEntities: &clusterEntities,
		NetworkMode:     types.HelmNetworkModeOffline,
		Bootstrap:       types.BootstrapNo,
	}, false)
	if err != nil {
		return nil, "", err
	}

	var entities entity.Entities
	if f.SkipOwnerRefs {
		clusterEntities := invModel.ClusterEntities()
		fullUIDMap := clusterEntities.UidMap(types.KeyClusterEntity)
		var matched []entity.Entity
		for _, e := range clusterEntities.Items {
			ok, err := predicate.EvalBool(e, types.MissingKeysError)
			if err != nil {
				return nil, "", err
			}
			if !ok {
				continue
			}
			matched = append(matched, e)
		}
		matchedEnts, err := entity.NewEntities(matched)
		if err != nil {
			return nil, "", err
		}
		entities, err = matchedEnts.ClusterInventoryEntitiesExcludingOwnedChildren(types.KeyClusterEntity, fullUIDMap)
		if err != nil {
			return nil, "", err
		}
	} else {
		_, entities, err = invModel.ClusterEntities().Select(func(e entity.Entity) (bool, error) {
			return predicate.EvalBool(e, types.MissingKeysError)
		})
		if err != nil {
			return nil, "", err
		}
	}

	entities, err = entities.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	if err != nil {
		return nil, "", err
	}

	l.DebugLog(logIdAction, "found {count} resources in cluster {cluster}",
		log.Int("count", entities.Len()),
		log.String("cluster", string(cluster.ClusterName)))

	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return nil, "", err
		}
		fmt.Println(string(id))
	}

	return cluster, "", nil
}
