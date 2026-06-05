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
	"hydra-gitops.org/hydra/hydra-go/core/yq"
)

// ClusterDumpFlags contains configuration options for list operations.
type ClusterDumpFlags struct {
	flags.ClusterRESTClientFlags
	flags.KeepServerFieldsFlag
	flags.ContextFlag
	flags.ClusterFlag
	flags.ColorFlag
	flags.PredicatesFlag
	flags.NoCacheFlag
}

func (f *ClusterDumpFlags) Flags() flags.Flags {
	return f
}

// ClusterDump prints live cluster resources as a multi-document YAML stream.
func ClusterDump(f ClusterDumpFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	l.Info(logIdAction, "dumping cluster '{cluster}'", log.String("cluster", string(f.Cluster)))

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

	predicate, err := env.CompilePredicateAt(`hydra gitops dump --predicate`, f.Predicates...)
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
	_, entities, err := invModel.ClusterEntities().Select(func(e entity.Entity) (bool, error) {
		return predicate.EvalBool(e, types.MissingKeysError)
	})
	if err != nil {
		return nil, "", err
	}

	entities, err = entities.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	if err != nil {
		return nil, "", err
	}

	l.DebugLog(logIdAction, "found {count} resources in cluster {cluster}",
		log.Int("count", entities.Len()),
		log.String("cluster", string(cluster.ClusterName)))

	first := true
	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return nil, "", err
		}

		l.Info(logIdAction, "dumping '{entity}'", log.String("entity", string(id)))

		comment := string(id)

		clusterEntity, err := e.UnstructuredOrError(types.KeyClusterEntity)
		if err != nil {
			return nil, "", err
		}

		printed, err := yq.PrintObject(
			f.Color,
			f.KeepServerFields,
			&comment,
			&clusterEntity,
		)
		if err != nil {
			return nil, "", err
		}

		if first {
			first = false
		} else {
			fmt.Println("---")
		}

		fmt.Println(printed)
	}

	return cluster, "", nil
}
