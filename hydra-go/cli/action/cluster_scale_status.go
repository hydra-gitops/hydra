package action

import (
	"fmt"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/references"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yq"
)

type ClusterScaleStatusFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.NoClusterFlag
	flags.KubernetesVersionFlag
	flags.BootstrapFlag
	flags.ExcludeAppFlag
	flags.NoCacheFlag
	YamlOutput bool
	// ShowAllHealthyApps (-A, --all) includes scale-target rows omitted by default (fully healthy blocks or missing Job with satisfied deps).
	ShowAllHealthyApps bool
	AppIdPatterns      []types.AppIdPattern
}

func (f *ClusterScaleStatusFlags) Flags() flags.Flags {
	return f
}

var _ flags.WithContextFlag = (*ClusterScaleStatusFlags)(nil)
var _ flags.WithColorFlag = (*ClusterScaleStatusFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*ClusterScaleStatusFlags)(nil)
var _ flags.WithNoClusterFlag = (*ClusterScaleStatusFlags)(nil)
var _ flags.WithKubernetesVersionFlag = (*ClusterScaleStatusFlags)(nil)
var _ flags.WithExcludeAppFlag = (*ClusterScaleStatusFlags)(nil)

func (f *ClusterScaleStatusFlags) WithBootstrapFlag() *flags.BootstrapFlag {
	return &f.BootstrapFlag
}

var _ flags.WithBootstrapFlag = (*ClusterScaleStatusFlags)(nil)

func ClusterScaleStatus(f ClusterScaleStatusFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for scale status")
	}

	if f.NoCluster {
		return fmt.Errorf("cluster scale status requires a live cluster (do not use --no-cluster)")
	}

	cluster, err := commands.ResolveCommandCluster(commands.ResolveCommandClusterOptions{
		Config:       config,
		HydraContext: f.HydraContext,
		Limits:       f.ToRESTClientLimits(),
		AppIds:       appIds,
	})
	if err != nil {
		return err
	}

	skipRootApps := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	renderedEntities, _, _, err := commands.RenderCluster(cluster, appIds, f.KubernetesVersion, types.CrdModeSilent, skipRootApps, nil)
	if err != nil {
		return err
	}

	parsers, err := hydra.HydraAppRefParsers(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return err
	}

	preferredVersions, err := cluster.PreferredVersions(nil)
	if err != nil {
		return err
	}

	scaleMap, err := commands.MergedScaleWorkloadMap(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return err
	}

	clusterEntities, err := commands.ListClusterAll(cluster, types.KeyClusterEntity, false, 0)
	if err != nil {
		return err
	}
	invModel, err := commands.BuildResourceModel(commands.ResourceModelInput{
		Cluster:         cluster,
		ClusterEntities: &clusterEntities,
		NetworkMode:     types.HelmNetworkModeOffline,
		Bootstrap:       types.BootstrapNo,
	}, false)
	if err != nil {
		return err
	}
	liveEntities := invModel.ClusterEntities()

	scaleEntities, err := renderedEntities.Merge(liveEntities, types.KeyClusterEntity)
	if err != nil {
		return err
	}

	ownerNamespaces, err := hydra.HydraAppNamespaceOwners(cluster, appIds, f.HelmNetworkMode)
	if err != nil {
		return err
	}
	refEntities, err := commands.AugmentClusterScaleEntitiesForRefs(scaleEntities, ownerNamespaces)
	if err != nil {
		return err
	}
	refs, err := references.Refs(l, refEntities, types.KeyTemplateEntity, nil, entity.Entities{}, entity.Entities{}, preferredVersions, parsers)
	if err != nil {
		return err
	}
	refs = references.AnnotateRefsWithSource(refs, types.RefSourceTemplate)

	tree, err := commands.LoadInspectRefGraph(commands.InspectRefGraphParams{
		Cluster:                     cluster,
		NetworkMode:                 f.HelmNetworkMode,
		Bootstrap:                   f.Bootstrap,
		ClusterInventory:            &liveEntities,
		IncludeTemplateRefs:         true,
		IncludeClusterRefs:          true,
		IncludeCloneMaterialization: true,
		SkipFoundDefinitionsInfoLog: true,
	})
	if err != nil {
		return err
	}
	var fullRefs []types.Ref
	if tree != nil {
		fullRefs = tree.Refs
	}

	readyEval, err := commands.ReadyEvaluatorFromHydra(cluster, f.HelmNetworkMode, scaleMap, scaleEntities, types.KeyClusterEntity)
	if err != nil {
		return err
	}

	report, err := commands.ComputeClusterScaleWorkloadStatusReport(
		scaleEntities,
		refs,
		fullRefs,
		types.KeyTemplateEntity,
		types.KeyClusterEntity,
		readyEval,
		scaleMap,
	)
	if err != nil {
		return err
	}

	fullReport := report
	report = commands.FilterClusterScaleStatusReportOmitFullyHealthyApps(report, f.ShowAllHealthyApps)

	log.FlushProgressForStdout()
	if f.YamlOutput {
		out, err := yq.ToYaml(f.Color, report)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprint(os.Stdout, out); err != nil {
			return err
		}
		return nil
	}

	if commands.ClusterScaleStatusAllTargetsOmittedAsHealthy(report, fullReport, f.ShowAllHealthyApps) {
		const msg = "All scale targets in the selection are up and ready (no issues found)."
		if bool(f.Color) {
			_, err := fmt.Fprintf(os.Stdout, "%s%s%s\n", colors.Green.String(), msg, colors.Reset.String())
			return err
		}
		_, err := fmt.Fprintln(os.Stdout, msg)
		return err
	}

	return commands.WriteClusterScaleStatusText(os.Stdout, report, f.Color)
}
