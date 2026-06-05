package action

import (
	"fmt"
	"strings"

	"github.com/pmezard/go-difflib/difflib"

	"hydra-gitops.org/hydra/hydra-go/base/colors"
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/cli/flags"
	"hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/commands"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/util/sets"
)

type ClusterDiffFlags struct {
	flags.ClusterRESTClientFlags
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.CrdModeFlag
	flags.KubernetesVersionFlag
	flags.DiffModeFlag
	flags.DiffUnifiedContextFlag
	flags.ExcludeAppFlag
	flags.PredicatesFlag
	flags.NoCacheFlag
	AppIdPatterns []types.AppIdPattern
}

func (f *ClusterDiffFlags) Flags() flags.Flags {
	return f
}

// ClusterDiff shows the diff between rendered templates and the live cluster state
// for the selected app(s). In server mode (default), templates are sent through
// a server-side apply dry-run so that API-server defaults are filled in before
// comparison. In raw mode, templates are compared 1:1 against the cluster state.
func ClusterDiff(f ClusterDiffFlags) error {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedYes)

	appIds, err := commands.ResolveAppIdsFromConfig(l, f.HydraContext, config, f.AppIdPatterns, f.ExcludeAppPatterns, f.HelmNetworkMode, false)
	if err != nil {
		return err
	}

	if len(appIds) == 0 {
		return log.CreateError(errors.ErrNoAppsSpecified, "no apps specified for diff")
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

	allClusterAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return err
	}

	skipRootApps := types.SkipRootApps(cluster.ClusterName != types.InCluster)
	renderedEntities, _, _, err := commands.RenderCluster(cluster, appIds, f.KubernetesVersion, f.CrdMode, skipRootApps, nil)
	if err != nil {
		return err
	}

	var hydraConfigCatalog entity.Entities
	if !appIds.Equal(allClusterAppIds) {
		hydraConfigCatalog, err = commands.RenderClusterSelectedApps(
			cluster, f.HelmNetworkMode, f.KubernetesVersion, allClusterAppIds, types.KeyTemplateEntity)
		if err != nil {
			return err
		}
	}

	diffIgnoreEntries, err := hydra.HydraDiffIgnoreRuleEntries(cluster, appIds, f.HelmNetworkMode, renderedEntities)
	if err != nil {
		return err
	}
	templatePatchEntries, err := hydra.HydraTemplatePatchRuleEntries(cluster, appIds, f.HelmNetworkMode, renderedEntities, hydraConfigCatalog)
	if err != nil {
		return err
	}
	var templatePatchOwnerByNs map[types.Namespace]types.AppId
	if len(templatePatchEntries) > 0 {
		templatePatchOwnerByNs, err = commands.BuildTemplatePatchOwnerByNamespace(
			cluster, f.HelmNetworkMode, renderedEntities, hydraConfigCatalog, types.KeyTemplateEntity)
		if err != nil {
			return err
		}
	}
	templatePatchPipe, err := commands.NewTemplatePatchPipelineWithNamespaceOwners(templatePatchEntries, templatePatchOwnerByNs)
	if err != nil {
		return err
	}
	renderedEntities, err = commands.ApplyTemplatePatchesToEntities(templatePatchPipe, renderedEntities, types.KeyTemplateEntity)
	if err != nil {
		return err
	}

	if f.DiffMode == types.DiffModeServer {
		l.Info(logIdAction, "running server-side apply dry-run on rendered templates to enrich them with API-server defaults")
		renderedEntities, _, err = commands.ServerSideDryRunApplyEntities(cluster, renderedEntities, types.KeyTemplateEntity, types.KeyTemplateEntity,
			commands.ServerSideDryRunApplyOptions{
				FailOnAnyPatchError: true,
				APIWarningSource:    k8s.KubernetesAPIWarningSourceServerSideDiff,
			}, false)
		if err != nil {
			return err
		}
	} else {
		l.Info(logIdAction, "using raw template comparison (no server-side dry-run)")
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
	clusterEntities = invModel.ClusterEntities()

	diffIgnorePipeline, err := commands.NewDiffIgnorePipeline(diffIgnoreEntries)
	if err != nil {
		return err
	}

	merged, err := renderedEntities.Merge(
		clusterEntities,
		types.KeyTemplateEntity,
		types.KeyClusterEntity)
	if err != nil {
		return err
	}

	compareResult, err := merged.Compare(types.KeyTemplateEntity, types.KeyClusterEntity)
	if err != nil {
		return err
	}

	leftOnly := compareResult.LeftOnly.Items
	both := compareResult.Both.Items

	var resourcePredicate cel.Predicate
	if len(f.Predicates) > 0 {
		env, err := cel.NewEnv()
		if err != nil {
			return err
		}
		resourcePredicate, err = env.CompilePredicateAt(`hydra gitops diff --predicate`, f.Predicates...)
		if err != nil {
			return err
		}
		leftOnly, err = filterDiffEntitiesByPredicate(leftOnly, resourcePredicate)
		if err != nil {
			return err
		}
		both, err = filterDiffEntitiesByPredicate(both, resourcePredicate)
		if err != nil {
			return err
		}
	}

	orphans, err := filterManagedOrphans(l, compareResult.RightOnly, appIds)
	if err != nil {
		return err
	}

	orphanItems := orphans.Items
	if resourcePredicate != nil {
		orphanItems, err = filterDiffEntitiesByPredicate(orphanItems, resourcePredicate)
		if err != nil {
			return err
		}
	}

	contextLines := flags.UnifiedDiffContextLines(f.Before, f.After, f.Both)
	var sb strings.Builder

	// LeftOnly: only in templates -> new resources (all lines as additions)
	for _, e := range leftOnly {
		if diffIgnorePipeline != nil {
			skipDiff, err := diffIgnorePipeline.IgnoreLeftOnlyWhenClusterMissing(e)
			if err != nil {
				return err
			}
			if skipDiff {
				id, idErr := e.Id()
				if idErr != nil {
					return idErr
				}
				sb.WriteString(fmt.Sprintf("diff ignored (resource absent in cluster): %s\n", id))
				continue
			}
		}
		localYaml, yErr := entityYamlForDiff(e, types.KeyTemplateEntity, diffIgnorePipeline)
		if yErr != nil {
			return yErr
		}
		d, err := entityDiff(e, "", localYaml, contextLines)
		if err != nil {
			return err
		}
		sb.WriteString(d)
	}

	// Both: in templates and cluster -> potentially modified
	for _, e := range both {
		localYaml, yErr := entityYamlForDiff(e, types.KeyTemplateEntity, diffIgnorePipeline)
		if yErr != nil {
			return yErr
		}
		clusterYaml, yErr := entityYamlForDiff(e, types.KeyClusterEntity, diffIgnorePipeline)
		if yErr != nil {
			return yErr
		}
		if localYaml == clusterYaml {
			continue
		}
		d, err := entityDiff(e, clusterYaml, localYaml, contextLines)
		if err != nil {
			return err
		}
		sb.WriteString(d)
	}

	for _, e := range orphanItems {
		clusterYaml, yErr := entityYamlForDiff(e, types.KeyClusterEntity, diffIgnorePipeline)
		if yErr != nil {
			return yErr
		}
		d, err := entityDiff(e, clusterYaml, "", contextLines)
		if err != nil {
			return err
		}
		sb.WriteString(d)
	}

	fullDiff := sb.String()
	if fullDiff == "" {
		l.Info(logIdAction, "no differences found")
		return nil
	}

	if f.Color {
		fullDiff = colors.ColorDiff(fullDiff)
	}

	fmt.Print(fullDiff)
	return nil
}

// entityYamlForDiff converts an entity's unstructured object to a clean YAML string after optional
// global.hydra.diff.ignore normalization (same pipeline as apply).
func entityYamlForDiff(e entity.Entity, key types.EntityKeyUnstructured, pipeline *commands.DiffIgnorePipeline) (string, error) {
	u, ok := e.Unstructured(key)
	if !ok {
		return "", nil
	}
	copy := *u.DeepCopy()
	if pipeline != nil {
		if err := pipeline.ApplyToUnstructured(e, &copy); err != nil {
			return "", err
		}
	}
	yamlStr, err := yaml.PrintObject(types.KeepServerFieldsNo, nil, &copy)
	if err != nil {
		return "", err
	}
	return string(yamlStr), nil
}

// entityDiff produces a unified diff between oldYaml and newYaml for the given entity.
// Either oldYaml or newYaml (but not both) may be empty, representing creation or deletion.
// contextLines is passed to go-difflib as the symmetric context around each hunk.
func entityDiff(e entity.Entity, oldYaml, newYaml string, contextLines int) (string, error) {
	if oldYaml == newYaml {
		return "", nil
	}

	id, err := e.Id()
	if err != nil {
		return "", err
	}

	oldName := "a/" + string(id)
	newName := "b/" + string(id)
	if oldYaml == "" {
		oldName = "/dev/null"
	}
	if newYaml == "" {
		newName = "/dev/null"
	}

	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldYaml),
		B:        difflib.SplitLines(newYaml),
		FromFile: oldName,
		ToFile:   newName,
		Context:  contextLines,
	}

	text, err := difflib.GetUnifiedDiffString(diff)
	if err != nil {
		return "", err
	}

	if text == "" {
		return "", nil
	}

	return text, nil
}

// filterDiffEntitiesByPredicate keeps entities for which the CEL predicate evaluates to true.
// Uses MissingKeysReject like hydra local find.
func filterDiffEntitiesByPredicate(items []entity.Entity, pred cel.Predicate) ([]entity.Entity, error) {
	if pred == nil || len(items) == 0 {
		return items, nil
	}
	ents, err := entity.NewEntities(items)
	if err != nil {
		return nil, err
	}
	_, matched, err := ents.Select(func(e entity.Entity) (bool, error) {
		return pred.EvalBool(e, types.MissingKeysReject)
	})
	if err != nil {
		return nil, err
	}
	return matched.Items, nil
}

// filterManagedOrphans filters cluster-only entities to those managed by the given apps
// using ArgoCD tracking IDs. Resources with ownerReferences are excluded because they
// are controller-managed and would never appear in rendered templates.
func filterManagedOrphans(
	l log.Logger,
	candidates entity.Entities,
	appIds sets.Set[types.AppId],
) (entity.Entities, error) {
	if candidates.Len() == 0 {
		return candidates, nil
	}

	candidates, err := candidates.UnselectAll()
	if err != nil {
		return entity.Entities{}, err
	}

	candidates, err = commands.MarkAsSelectedArgoCdManagedResources(l, candidates, types.KeyClusterEntity, appIds)
	if err != nil {
		return entity.Entities{}, err
	}

	selected, err := candidates.Selected()
	if err != nil {
		return entity.Entities{}, err
	}

	// exclude resources with ownerReferences — these are controller-managed
	filtered := make([]entity.Entity, 0, selected.Len())
	for _, e := range selected.Items {
		owners := e.OwnerUids(types.KeyClusterEntity)
		if owners != nil && owners.Len() > 0 {
			continue
		}
		filtered = append(filtered, e)
	}

	return entity.NewEntities(filtered)
}
