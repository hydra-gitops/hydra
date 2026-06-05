package action

import (
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

// TemplateFlags holds flags specific to the template command
type TemplateFlags struct {
	flags.HelmNetworkModeFlag
	flags.ContextFlag
	flags.ColorFlag
	flags.KubernetesVersionFlag
	flags.ExcludeAppFlag
	flags.PredicatesFlag
	flags.BootstrapFlag
	flags.NoCacheFlag
	AppId types.AppId
}

var _ flags.WithContextFlag = (*TemplateFlags)(nil)
var _ flags.WithColorFlag = (*TemplateFlags)(nil)
var _ flags.WithHelmNetworkModeFlag = (*TemplateFlags)(nil)
var _ flags.WithKubernetesVersionFlag = (*TemplateFlags)(nil)
var _ flags.WithPredicatesFlag = (*TemplateFlags)(nil)
var _ flags.WithBootstrapFlag = (*TemplateFlags)(nil)
var _ flags.WithNoCacheFlag = (*TemplateFlags)(nil)

func (f *TemplateFlags) Flags() flags.Flags {
	return f
}

func (f *TemplateFlags) WithBootstrapFlag() *flags.BootstrapFlag {
	return &f.BootstrapFlag
}

func (f *TemplateFlags) WithNoCacheFlag() *flags.NoCacheFlag {
	return &f.NoCacheFlag
}

// Template renders and returns the templates for the given app.
func Template(f TemplateFlags) (hydra.Hydra, string, error) {
	h, err := resolveHydraForLocalTemplateRender(f)
	if err != nil {
		return nil, "", err
	}
	out, err := templateRenderedColoredYAMLFromResolved(h, f)
	if err != nil {
		return nil, "", err
	}
	return h, out, nil
}

func resolveHydraForLocalTemplateRender(f TemplateFlags) (hydra.Hydra, error) {
	l := log.Default()
	config := flags.NewConfigFromFlags(&f, types.KubernetesConnectionAllowedNo)
	return resolveHydraAppForLocalPrint(l, f.HydraContext, f.AppId, config, f.HelmNetworkMode)
}

// resolveHydraAppForLocalPrint resolves and validates a Hydra app for local template-style commands.
func resolveHydraAppForLocalPrint(
	l log.Logger,
	hydraContext types.HydraContext,
	appId types.AppId,
	config types.Config,
	networkMode types.HelmNetworkMode,
) (hydra.Hydra, error) {
	l.Info(logIdAction, "Rendering templates for AppId '{appId}'", log.String("appId", string(appId)))

	h, err := hydra.ResolvePathWithAppId(l, hydraContext, appId, config)
	if err != nil {
		return nil, err
	}

	cluster := clusterFromHydraApp(h)
	if cluster == nil {
		return nil, log.CreateError(errors.ErrInvalidHydraStructure, "could not resolve cluster for local Helm render")
	}
	if err := commands.ValidateAppIdInCluster(cluster, appId, networkMode); err != nil {
		return nil, err
	}

	if h.AsApp() == nil {
		return nil, log.CreateError(errors.ErrInvalidHydraStructure,
			"local Helm render requires a root or child app")
	}

	return h, nil
}

// TemplateSortedRenderedEntities returns rendered template entities for f.AppId using the same
// pipeline as hydra local template (scope catalog, template patches, then optional CEL filters).
func TemplateSortedRenderedEntities(f TemplateFlags) (entity.Entities, error) {
	h, err := resolveHydraForLocalTemplateRender(f)
	if err != nil {
		return entity.Entities{}, err
	}
	return templateSortedEntitiesFromResolved(h, f)
}

// templateSortedEntitiesFromResolved runs the Hydra template entity pipeline for f.AppId
// after [resolveHydraForLocalTemplateRender] produced h.
func templateSortedEntitiesFromResolved(h hydra.Hydra, f TemplateFlags) (entity.Entities, error) {
	l := log.Default()
	app := h.AsApp()
	if app == nil {
		return entity.Entities{}, log.CreateError(errors.ErrInvalidHydraStructure,
			"local Helm render requires a root or child app")
	}
	cluster := clusterFromHydraApp(app)
	if cluster == nil {
		return entity.Entities{}, log.CreateError(errors.ErrInvalidHydraStructure, "could not resolve cluster for local Helm render")
	}

	allAppIds, err := cluster.AppIds(f.HelmNetworkMode)
	if err != nil {
		return entity.Entities{}, err
	}
	scopeCatalog, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, allAppIds, types.KeyTemplateEntity,
		commands.WithSkipFoundDefinitionsInfoLog())
	if err != nil {
		return entity.Entities{}, err
	}

	partitionRender, err := commands.RenderClusterSelectedApps(
		cluster, f.HelmNetworkMode, f.KubernetesVersion, sets.New(f.AppId), types.KeyTemplateEntity,
		commands.WithSkipFoundDefinitionsInfoLog(),
		commands.WithScopeCatalog(scopeCatalog))
	if err != nil {
		return entity.Entities{}, err
	}
	entities, err := commands.ApplyTemplatePatchesUsingPartitionRender(
		cluster, sets.New(f.AppId), f.HelmNetworkMode, partitionRender, scopeCatalog, partitionRender, types.KeyTemplateEntity)
	if err != nil {
		return entity.Entities{}, err
	}

	l.DebugLog(logIdAction, "Finished rendering templates for AppId '{appId}'", log.String("appId", string(f.AppId)))

	if len(f.Predicates) > 0 {
		env, err := cel.NewEnv()
		if err != nil {
			return entity.Entities{}, err
		}

		predicate, err := env.CompilePredicateAt(`hydra template --predicate`, f.Predicates...)
		if err != nil {
			return entity.Entities{}, err
		}

		_, matched, err := entities.Select(func(e entity.Entity) (bool, error) {
			return predicate.EvalBool(e, types.MissingKeysReject)
		})
		if err != nil {
			return entity.Entities{}, err
		}

		return matched.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
	}

	return entities.Sort(entity.NewIdFieldOrder(types.DirectionAscending))
}

// templateRenderedColoredYAMLFromResolved runs the Hydra template entity pipeline for f.AppId
// after [resolveHydraForLocalTemplateRender] produced h.
func templateRenderedColoredYAMLFromResolved(h hydra.Hydra, f TemplateFlags) (string, error) {
	sorted, err := templateSortedEntitiesFromResolved(h, f)
	if err != nil {
		return "", err
	}

	fullYaml, err := sorted.ToYaml(types.KeyTemplateEntity)
	if err != nil {
		return "", err
	}

	colored, err := yq.YamlStringColored(f.Color, types.YamlString(string(fullYaml)))
	if err != nil {
		return "", err
	}

	return colored, nil
}
