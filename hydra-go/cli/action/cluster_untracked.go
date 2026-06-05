package action

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ClusterUntrackedFlags configures hydra gitops untracked.
type ClusterUntrackedFlags struct {
	flags.ClusterRESTClientFlags
	flags.ContextFlag
	flags.ClusterFlag
	flags.HelmNetworkModeFlag
	flags.ExcludeAppFlag
	flags.PredicatesFlag
	flags.NoCacheFlag
	flags.ClusterListParallelFlag
}

func (f *ClusterUntrackedFlags) Flags() flags.Flags {
	return f
}

func (f *ClusterUntrackedFlags) WithClusterListParallelFlag() *flags.ClusterListParallelFlag {
	return &f.ClusterListParallelFlag
}

var _ flags.WithClusterListParallelFlag = (*ClusterUntrackedFlags)(nil)

// ClusterUntracked prints Hydra resource ids for live objects that are not present in any
// merged enabled-app template and do not match enabled cluster-defaults presets (coredns,
// kubernetes, flannel, canal, kubermatic, syseleven, metakube, syseleven-node-problem-detector, quobyte, cloudinit, cinder, talos) or other Kubernetes standard exemptions used by cluster review.
// Live objects matched by enabled global.hydra ref-parser predicates tagged uninstall,
// uninstall-force, uninstall-safe, or backup are omitted (same predicate families as cluster uninstall).
// After that, untracked roots referenced from live inventory ref edges (ClusterInventoryRefs) are
// dropped when the inventory root of the ref source is not untracked, repeating until stable so
// runtime targets reached via any ref-parser edge (for example PVC→PV) are omitted generically.
// Child objects (ownerReference UID present elsewhere in the live inventory) are omitted;
// only roots are evaluated, except when every owner UID is missing or empty (then the object
// is still treated as a root).
func ClusterUntracked(f ClusterUntrackedFlags) (hydra.Hydra, string, error) {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	if f.Parallel < 0 {
		return nil, "", fmt.Errorf("--parallel must not be negative")
	}
	if f.Parallel > 64 {
		return nil, "", fmt.Errorf("--parallel must be at most 64")
	}

	clusterName := f.Cluster
	appIds, err := commands.ResolveAppIdsInClusterWithExcludes(
		l, f.HydraContext, config, clusterName, f.ExcludeAppPatterns, f.HelmNetworkMode,
		f.ToRESTClientLimits())
	if err != nil {
		return nil, "", err
	}
	if len(appIds) == 0 {
		return nil, "", log.CreateError(errors.ErrNoAppsSpecified, "no apps left for cluster untracked after excludes")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		ClusterName:  clusterName,
	})
	if err != nil {
		return nil, "", err
	}
	showProgress := log.TerminalProgressUI()

	renderAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return nil, "", err
	}
	scopeInfo, err := commands.ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, types.CrdModeSilent)
	if err != nil {
		return nil, "", err
	}
	renderedAllApps, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, "", renderAppIds, types.KeyTemplateEntity,
		commands.WithDefinitionsProgress(showProgress))
	if err != nil {
		return nil, "", err
	}
	renderedAllApps, err = commands.NormalizeApiVersions(cluster.L(), renderedAllApps, types.KeyTemplateEntity, cluster, func() (types.ScopeInfoMap, error) {
		return scopeInfo, nil
	})
	if err != nil {
		return nil, "", err
	}
	liveEntities, err := commands.ListClusterAll(cluster, types.KeyClusterEntity, showProgress, f.Parallel)
	if err != nil {
		return nil, "", err
	}
	perAppRendered, err := commands.PartitionTemplateEntitiesByPrimaryApp(renderedAllApps)
	if err != nil {
		return nil, "", err
	}
	for appId := range perAppRendered {
		if !appIds.Has(appId) {
			delete(perAppRendered, appId)
		}
	}
	var selectedRenderedItems []entity.Entity
	for _, ents := range perAppRendered {
		selectedRenderedItems = append(selectedRenderedItems, ents.Items...)
	}
	selectedRenderedAllApps, err := entity.NewEntities(selectedRenderedItems)
	if err != nil {
		return nil, "", err
	}

	invModel, err := commands.BuildResourceModel(commands.ResourceModelInput{
		Cluster:                cluster,
		NetworkMode:            f.HelmNetworkMode,
		Bootstrap:              types.BootstrapNo,
		TemplateEntities:       &selectedRenderedAllApps,
		ClusterEntities:        &liveEntities,
		PerAppTemplateEntities: perAppRendered,
		AppIds:                 appIds,
		PredicateAppIds:        appIds,
		ScopeInfo:              scopeInfo,
		Parallel:               f.Parallel,
	}, showProgress)
	if err != nil {
		return nil, "", err
	}

	l.Info(logIdAction, "loading inventory for untracked calculation on '{cluster}'",
		log.String("cluster", string(cluster.ClusterName)))
	log.FlushProgressForStdout()

	untracked, err := invModel.UntrackedRootClusterEntities()
	if err != nil {
		return nil, "", err
	}

	if len(f.Predicates) > 0 {
		env, err := cel.NewEnv()
		if err != nil {
			return nil, "", err
		}
		predicate, err := env.CompilePredicateAt(`hydra gitops untracked --predicate`, f.Predicates...)
		if err != nil {
			return nil, "", err
		}
		_, matched, err := untracked.Select(func(e entity.Entity) (bool, error) {
			return predicate.EvalBool(e, types.MissingKeysReject)
		})
		if err != nil {
			return nil, "", err
		}
		untracked = matched
	}

	untracked, err = untracked.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	if err != nil {
		return nil, "", err
	}

	l.Info(logIdAction, "untracked entity count for '{cluster}': {count}",
		log.String("cluster", string(cluster.ClusterName)),
		log.Int("count", untracked.Len()))

	for _, e := range untracked.Items {
		id, err := e.Id()
		if err != nil {
			return nil, "", err
		}
		fmt.Println(string(id))
	}

	return cluster, "", nil
}
