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
	"hydra-gitops.org/hydra/hydra-go/core/yq"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ClusterTemplateFlags configures hydra gitops template.
type ClusterTemplateFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.CrdModeFlag
	flags.KubernetesVersionFlag
	flags.ExcludeAppFlag
	flags.PredicatesFlag
	flags.BootstrapFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterTemplateFlags) Flags() flags.Flags { return f }

func (f *ClusterTemplateFlags) WithBootstrapFlag() *flags.BootstrapFlag { return &f.BootstrapFlag }

var _ flags.WithContextFlag = (*ClusterTemplateFlags)(nil)
var _ flags.WithColorFlag = (*ClusterTemplateFlags)(nil)
var _ flags.WithPredicatesFlag = (*ClusterTemplateFlags)(nil)
var _ flags.WithBootstrapFlag = (*ClusterTemplateFlags)(nil)
var _ flags.WithCrdModeFlag = (*ClusterTemplateFlags)(nil)
var _ flags.WithNoCacheFlag = (*ClusterTemplateFlags)(nil)

// ClusterTemplate renders selected apps like hydra local template, but uses live API discovery to
// normalize preferred API versions and collects global.hydra.templatePatches (Helm + Hydra ConfigMaps)
// from every app on the cluster before applying them to each printed manifest set.
func ClusterTemplate(f ClusterTemplateFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}
	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for cluster template")
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

	cluster.ResetPreferredVersionsCache()

	allAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return err
	}

	skipRootApps := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	renderAppIds := allAppIds
	if skipRootApps {
		renderAppIds = sets.New[types.AppId]()
		for id := range allAppIds {
			if !id.IsRootApp() {
				renderAppIds.Insert(id)
			}
		}
	}

	var renderOpts []commands.RenderClusterSelectedAppsOption
	if log.TerminalProgressUI() {
		renderOpts = append(renderOpts, commands.WithDefinitionsProgress(true))
	}
	renderOpts = append(renderOpts, commands.WithSkipFoundDefinitionsInfoLog())

	fullClusterRender, _, err := commands.RenderClusterAllApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, types.KeyTemplateEntity, skipRootApps, renderOpts...)
	if err != nil {
		return err
	}

	catalogRender, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, renderAppIds, types.KeyTemplateEntity, renderOpts...)
	if err != nil {
		return err
	}

	mergedPerApp, err := commands.PrepareClusterHelmMergedHydraMaps(
		cluster, renderAppIds, f.HelmNetworkMode, catalogRender, fullClusterRender)
	if err != nil {
		return err
	}

	cluster.SetHelmInputValuesForApp(func(id types.AppId) (types.ValuesMap, error) {
		ha, err := cluster.WithApp(id)
		if err != nil {
			return nil, err
		}
		mh, ok := mergedPerApp[id]
		if !ok {
			return nil, log.CreateError(errors.ErrInternalError, "missing merged hydra for app {appId}",
				log.String("appId", string(id)))
		}
		return commands.ClusterHelmInstallValuesMap(ha, f.HelmNetworkMode, mh)
	})
	defer cluster.ClearHelmInputValuesForApp()

	fullRender, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, renderAppIds, types.KeyTemplateEntity, renderOpts...)
	if err != nil {
		return err
	}

	clusterScopeInfoMap, err := commands.ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, f.CrdMode)
	if err != nil {
		return err
	}

	mergedCrdScope, err := commands.ClusterApplyCrdScopeMap(fullRender, clusterScopeInfoMap, types.KeyTemplateEntity)
	if err != nil {
		return err
	}

	templatePatchEntries, err := hydra.HydraTemplatePatchRuleEntries(cluster, renderAppIds, f.HelmNetworkMode, fullRender, fullClusterRender)
	if err != nil {
		return err
	}
	var templatePatchOwnerByNs map[types.Namespace]types.AppId
	if len(templatePatchEntries) > 0 {
		templatePatchOwnerByNs, err = commands.BuildTemplatePatchOwnerByNamespace(
			cluster, f.HelmNetworkMode, fullRender, fullClusterRender, types.KeyTemplateEntity)
		if err != nil {
			return err
		}
	}
	templatePatchPipe, err := commands.NewTemplatePatchPipelineWithNamespaceOwners(templatePatchEntries, templatePatchOwnerByNs)
	if err != nil {
		return err
	}

	var cloneYaml types.YamlString
	rules, err := hydra.HydraAppCloneRules(cluster, allAppIds, f.HelmNetworkMode, fullRender)
	if err != nil {
		return err
	}
	if len(rules) > 0 {
		withClones, bootCount, err := commands.MaterializeHydraClonesForApply(
			l, cluster, allAppIds, fullRender, types.KeyTemplateEntity, f.Bootstrap, f.HelmNetworkMode, nil)
		if err != nil {
			return err
		}
		if err := commands.ValidateBootstrapTemplateClones(f.Bootstrap, rules, bootCount); err != nil {
			return err
		}
		added, err := commands.DiffEntities(fullRender, withClones)
		if err != nil {
			return err
		}
		added, err = commands.ApplyScopeInfoMaps(f.CrdMode, added, types.KeyTemplateEntity, mergedCrdScope)
		if err != nil {
			return err
		}
		added, err = commands.NormalizeApiVersions(l, added, types.KeyTemplateEntity, cluster, func() (types.ScopeInfoMap, error) {
			return clusterScopeInfoMap, nil
		})
		if err != nil {
			return err
		}
		added, err = commands.ApplyTemplatePatchesToEntities(templatePatchPipe, added, types.KeyTemplateEntity)
		if err != nil {
			return err
		}
		cloneYaml, err = added.ToYaml(types.KeyTemplateEntity)
		if err != nil {
			return err
		}
	} else if f.Bootstrap == types.BootstrapYes {
		if err := commands.ValidateBootstrapTemplateClones(f.Bootstrap, nil, 0); err != nil {
			return err
		}
	}

	for appId := range appIds {
		if err := commands.ValidateAppIdInCluster(cluster, appId, f.HelmNetworkMode); err != nil {
			return err
		}
		out, err := clusterTemplateRenderOneApp(
			l, cluster, appId, fullRender, clusterScopeInfoMap, mergedCrdScope, f.CrdMode)
		if err != nil {
			return err
		}
		out, err = commands.ApplyTemplatePatchesToEntities(templatePatchPipe, out, types.KeyTemplateEntity)
		if err != nil {
			return err
		}
		out, err = out.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
		if err != nil {
			return err
		}
		outStr, err := clusterTemplateApplyPredicates(f.Predicates, out)
		if err != nil {
			return err
		}
		colored, err := yq.YamlStringColored(f.Color, types.YamlString(outStr))
		if err != nil {
			return err
		}
		fmt.Println(string(colored))
		l.Info(logIdAction, "cluster template output for AppId '{appId}'", log.String("appId", string(appId)))
	}

	if len(cloneYaml) > 0 {
		coloredClone, err := yq.YamlStringColored(f.Color, cloneYaml)
		if err != nil {
			return err
		}
		fmt.Println("---")
		fmt.Println(string(coloredClone))
	}

	return nil
}

func clusterTemplateRenderOneApp(
	l log.Logger,
	cluster *hydra.Cluster,
	appId types.AppId,
	fullRender entity.Entities,
	clusterScopeInfoMap types.ScopeInfoMap,
	mergedCrdScope types.ScopeInfoMap,
	crdMode types.CrdMode,
) (entity.Entities, error) {
	namespaces, err := commands.ExclusiveNamespaces(l, fullRender, sets.New(appId))
	if err != nil {
		return entity.Entities{}, err
	}
	namespaceEntities, err := commands.CreateNamespaceEntities(namespaces, types.KeyTemplateEntity)
	if err != nil {
		return entity.Entities{}, err
	}

	_, renderedEntities, err := fullRender.SelectByAppIds(sets.New(appId))
	if err != nil {
		return entity.Entities{}, err
	}

	namespaceEntities, err = commands.WithoutDuplicateSyntheticKubernetesDefaults(l, renderedEntities, namespaceEntities)
	if err != nil {
		return entity.Entities{}, err
	}

	renderedEntities, err = renderedEntities.Append(namespaceEntities)
	if err != nil {
		return entity.Entities{}, err
	}

	renderedEntities, err = commands.ApplyScopeInfoMaps(crdMode, renderedEntities, types.KeyTemplateEntity, mergedCrdScope)
	if err != nil {
		return entity.Entities{}, err
	}

	renderedEntities, err = commands.NormalizeApiVersions(l, renderedEntities, types.KeyTemplateEntity, cluster, func() (types.ScopeInfoMap, error) {
		return clusterScopeInfoMap, nil
	})
	if err != nil {
		return entity.Entities{}, err
	}

	return renderedEntities, nil
}

func clusterTemplateApplyPredicates(predicates []types.CelPredicate, entities entity.Entities) (string, error) {
	if len(predicates) == 0 {
		yamlOut, err := entities.ToYaml(types.KeyTemplateEntity)
		if err != nil {
			return "", err
		}
		return string(yamlOut), nil
	}
	env, err := cel.NewEnv()
	if err != nil {
		return "", err
	}
	predicate, err := env.CompilePredicateAt(`hydra gitops template --predicate`, predicates...)
	if err != nil {
		return "", err
	}
	_, matched, err := entities.Select(func(e entity.Entity) (bool, error) {
		return predicate.EvalBool(e, types.MissingKeysReject)
	})
	if err != nil {
		return "", err
	}
	matched, err = matched.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	if err != nil {
		return "", err
	}
	filteredYaml, err := matched.ToYaml(types.KeyTemplateEntity)
	if err != nil {
		return "", err
	}
	return string(filteredYaml), nil
}
