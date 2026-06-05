package commands

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/util/sets"
)

// RenderClusterSelectedAppsOption configures [RenderClusterSelectedApps].
type RenderClusterSelectedAppsOption func(*renderClusterSelectedAppsConfig)

type renderClusterSelectedAppsConfig struct {
	skipFoundDefinitionsInfoLog bool
	// definitionsProgress enables footer progress: one step per app while templating Helm manifests.
	definitionsProgress bool
	// scopeCatalog merges ScopeInfoMapFromCrds from this full-cluster (or multi-app) render into the
	// per-batch merged scope so single-app renders still resolve GVK scope (e.g. Strimzi KafkaTopic)
	// when CRD manifests live in other apps. See WithScopeCatalog.
	scopeCatalog entity.Entities
}

func applyRenderClusterSelectedAppsOptions(opts []RenderClusterSelectedAppsOption) renderClusterSelectedAppsConfig {
	var cfg renderClusterSelectedAppsConfig
	for _, o := range opts {
		o(&cfg)
	}
	return cfg
}

// WithSkipFoundDefinitionsInfoLog suppresses the INFO log "found definitions of …" when
// RenderClusterSelectedApps is invoked repeatedly in one workflow (e.g. cluster uninstall).
func WithSkipFoundDefinitionsInfoLog() RenderClusterSelectedAppsOption {
	return func(c *renderClusterSelectedAppsConfig) {
		c.skipFoundDefinitionsInfoLog = true
	}
}

// WithScopeCatalog merges CRD-derived scope from a broader render (typically all apps on the cluster)
// into applyScopeInfoMap resolution. Required so per-app standalone renders match full-cluster ids
// when Helm omits metadata.namespace and the CRD is declared in another app.
func WithScopeCatalog(catalog entity.Entities) RenderClusterSelectedAppsOption {
	return func(c *renderClusterSelectedAppsConfig) {
		c.scopeCatalog = catalog
	}
}

// WithDefinitionsProgress reports per-app progress while rendering Helm templates for the selected apps.
func WithDefinitionsProgress(enabled bool) RenderClusterSelectedAppsOption {
	return func(c *renderClusterSelectedAppsConfig) {
		c.definitionsProgress = enabled
	}
}

func RenderClusterSelectedApps(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	appIds sets.Set[types.AppId],
	key types.EntityKeyUnstructured,
	opts ...RenderClusterSelectedAppsOption,
) (entity.Entities, error) {
	cfg := applyRenderClusterSelectedAppsOptions(opts)
	logger := cluster.L()
	logger.DebugLog(logIdCommands, "rendering {count} apps", log.Int("count", len(appIds)))

	clusterPath := cluster.ClusterPath()
	gitRoot, _ := findGitRoot(clusterPath)

	entries := []entity.Entity{}

	appList := appIds.UnsortedList()
	slices.SortFunc(appList, func(a, b types.AppId) int { return cmp.Compare(string(a), string(b)) })
	totalApps := len(appList)

	var defBar log.Progress
	var defDetailTask log.ProgressTask
	if cfg.definitionsProgress && totalApps > 0 {
		var err error
		defBar, err = logger.NewProgress("helm templates · apps", totalApps)
		if err != nil {
			return entity.Entities{}, err
		}
		defDetailTask = defBar.NewTask("")
		defer func() {
			if defBar != nil {
				_ = defBar.Close()
				logger.Info(logIdCommands, fmt.Sprintf("helm templates · apps: rendered %d app(s)", totalApps))
			}
		}()
	}

	for i, appId := range appList {
		if defBar != nil && defDetailTask != nil {
			defDetailTask.SetDetail(string(appId))
			defBar.Advance(i+1, totalApps)
		}
		h, err := cluster.WithApp(appId)
		if err != nil {
			return entity.Entities{}, err
		}

		kubernetesVersionOrFallback, err := hydra.KubernetesVersionOrFallback(h, kubernetesVersion, networkMode)
		if err != nil {
			return entity.Entities{}, err
		}

		t, err := h.Template(networkMode, kubernetesVersionOrFallback)
		if err != nil {
			return entity.Entities{}, err
		}

		items, err := renderedEntityItemsFromAppManifest(logger, clusterPath, gitRoot, h, t, networkMode, key)
		if err != nil {
			return entity.Entities{}, err
		}
		entries = append(entries, items...)
	}

	if !cfg.skipFoundDefinitionsInfoLog {
		logger.Info(logIdCommands, "found definitions of {resources} resources stored in {apps} apps", log.Int("apps", len(appIds)), log.Int("resources", len(entries)))
	}

	if err := ValidateNoCrossAppDuplicateTemplateResourceIds(entries); err != nil {
		return entity.Entities{}, err
	}

	rendered, err := entity.NewEntities(entries)
	if err != nil {
		return entity.Entities{}, err
	}

	templatePatchEntries, err := hydra.HydraTemplatePatchRuleEntries(cluster, appIds, networkMode, rendered, cfg.scopeCatalog)
	if err != nil {
		return entity.Entities{}, err
	}
	if len(templatePatchEntries) > 0 {
		templatePatchPipe, err := NewTemplatePatchPipelineWithNamespaceOwners(templatePatchEntries, nil)
		if err != nil {
			return entity.Entities{}, err
		}
		rendered, err = ApplyTemplatePatchesToEntitiesBeforeScope(templatePatchPipe, rendered, key)
		if err != nil {
			return entity.Entities{}, err
		}
	}

	templatesScopeInfoMap, err := rendered.ScopeInfoMapFromCrds(key)
	if err != nil {
		return entity.Entities{}, err
	}
	mergedScopeInfoMap, err := MergeScopeInfoMaps(DefaultScopeInfoMap(), templatesScopeInfoMap)
	if err != nil {
		return entity.Entities{}, err
	}
	if cfg.scopeCatalog.Len() > 0 {
		catalogScope, err := cfg.scopeCatalog.ScopeInfoMapFromCrds(key)
		if err != nil {
			return entity.Entities{}, err
		}
		mergedScopeInfoMap, err = MergeScopeInfoMaps(mergedScopeInfoMap, catalogScope)
		if err != nil {
			return entity.Entities{}, err
		}
	}
	// Same namespace/resource normalization as RenderCluster (ApplyScopeInfoMaps), but keep GVKs that are
	// not yet in DefaultScopeInfoMap or rendered CRDs until cluster scope is merged in RenderCluster.
	rendered, err = applyScopeInfoMap(types.CrdModeKeepUnknown, rendered, mergedScopeInfoMap, key)
	if err != nil {
		return entity.Entities{}, err
	}

	rendered, err = AssignSingularTemplateAppIdMetadata(rendered)
	if err != nil {
		return entity.Entities{}, err
	}

	return rendered, nil
}

// renderedEntityItemsFromAppManifest parses a rendered Helm manifest into template-scoped entities
// for one app (namespace, app ID, repo paths, same as a single iteration of RenderClusterSelectedApps).
func renderedEntityItemsFromAppManifest(
	logger log.Logger,
	clusterPath string,
	gitRoot string,
	h hydra.HydraApp,
	manifest types.YamlString,
	networkMode types.HelmNetworkMode,
	key types.EntityKeyUnstructured,
) ([]entity.Entity, error) {
	appId := h.AppId()

	appNamespace, err := h.Namespace(networkMode)
	if err != nil {
		return nil, err
	}

	e, err := entity.NewEntitiesFromYaml(logger, manifest, key)
	if err != nil {
		return nil, err
	}

	e, err = e.WithAppNamespace(types.AppNamespace(appNamespace))
	if err != nil {
		return nil, err
	}

	entries := make([]entity.Entity, 0, len(e.Items))
	for _, item := range e.Items {
		modified, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			b, _ = b.AddAppId(appId)
			return b
		})
		if modErr != nil {
			return nil, modErr
		}
		modified, modErr = enrichEntityPaths(modified, clusterPath, gitRoot)
		if modErr != nil {
			return nil, modErr
		}
		entries = append(entries, modified)
	}

	return entries, nil
}

// TemplateRenderedEntities builds template-scoped entities for a single Hydra app from an already-rendered manifest.
// It applies the same pipeline as RenderClusterSelectedApps (app namespace, app ID, paths, CRD scope) so CEL
// predicates behave like hydra local find.
func TemplateRenderedEntities(
	l log.Logger,
	h hydra.HydraApp,
	manifest types.YamlString,
	networkMode types.HelmNetworkMode,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	app := h.AsApp()
	if app == nil {
		return entity.Entities{}, log.CreateError(errors.ErrInvalidHydraStructure,
			"template resource filtering requires a Hydra app (root or child app)")
	}

	var clusterPath string
	switch {
	case app.AsRootApp() != nil:
		clusterPath = app.AsRootApp().ClusterPath()
	case app.AsChildApp() != nil:
		clusterPath = app.AsChildApp().ClusterPath()
	default:
		return entity.Entities{}, log.CreateError(errors.ErrInvalidHydraStructure,
			"template resource filtering requires a Hydra app (root or child app)")
	}

	gitRoot, _ := findGitRoot(clusterPath)

	items, err := renderedEntityItemsFromAppManifest(l, clusterPath, gitRoot, app, manifest, networkMode, key)
	if err != nil {
		return entity.Entities{}, err
	}

	if err := ValidateNoCrossAppDuplicateTemplateResourceIds(items); err != nil {
		return entity.Entities{}, err
	}

	rendered, err := entity.NewEntities(items)
	if err != nil {
		return entity.Entities{}, err
	}

	templatesScopeInfoMap, err := rendered.ScopeInfoMapFromCrds(key)
	if err != nil {
		return entity.Entities{}, err
	}
	mergedScopeInfoMap, err := MergeScopeInfoMaps(DefaultScopeInfoMap(), templatesScopeInfoMap)
	if err != nil {
		return entity.Entities{}, err
	}
	rendered, err = applyScopeInfoMap(types.CrdModeKeepUnknown, rendered, mergedScopeInfoMap, key)
	if err != nil {
		return entity.Entities{}, err
	}

	rendered, err = AssignSingularTemplateAppIdMetadata(rendered)
	if err != nil {
		return entity.Entities{}, err
	}

	return rendered, nil
}

func RenderClusterAllApps(
	cluster *hydra.Cluster,
	networkMode types.HelmNetworkMode,
	kubernetesVersion types.KubernetesVersion,
	key types.EntityKeyUnstructured,
	skipRootApps types.SkipRootApps,
	renderOpts ...RenderClusterSelectedAppsOption,
) (entity.Entities, sets.Set[types.AppId], error) {
	l := cluster.L()
	l.DebugLog(logIdCommands, "rendering CRDs for apps of cluster {cluster}", log.String("cluster", string(cluster.ClusterName)))

	appIds, err := cluster.AppIds(networkMode)
	if err != nil {
		return entity.Entities{}, nil, err
	}

	renderAppIds := appIds
	if skipRootApps {
		renderAppIds = sets.New[types.AppId]()
		for appId := range appIds {
			if appId.IsRootApp() {
				l.DebugLog(logIdCommands, "skipping root app {appId}", log.String("appId", string(appId)))
			} else {
				renderAppIds.Insert(appId)
			}
		}
	}

	for appId := range renderAppIds {
		l.DebugLog(logIdCommands, "found app id {appId}", log.String("appId", string(appId)))
	}

	rendered, err := RenderClusterSelectedApps(cluster, networkMode, kubernetesVersion, renderAppIds, key, renderOpts...)
	if err != nil {
		return entity.Entities{}, nil, err
	}

	return rendered, appIds, nil
}

func RenderCluster(
	cluster *hydra.Cluster,
	appIds sets.Set[types.AppId],
	kubernetesVersion types.KubernetesVersion,
	crdMode types.CrdMode,
	skipRootApps types.SkipRootApps,
	selectedAppsOpts []RenderClusterSelectedAppsOption,
	additionalScopeInfo ...types.ScopeInfoMap,
) (entity.Entities, sets.Set[types.Namespace], sets.Set[types.AppId], error) {
	cluster.ResetPreferredVersionsCache()

	// contains all rendered resources from all apps defined in the cluster
	allRenderedEntities, allAppIds, err := RenderClusterAllApps(cluster, types.HelmNetworkModeOnline, kubernetesVersion, types.KeyTemplateEntity, skipRootApps, selectedAppsOpts...)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	l := cluster.L()
	missingApps := appIds.Clone()
	missingApps.Delete(allAppIds.UnsortedList()...)
	if len(missingApps) > 0 {
		for appId := range missingApps {
			l.Error(logIdCommands, "App {appId} not found in cluster {cluster}",
				log.String("appId", string(appId)),
				log.String("cluster", string(cluster.ClusterName)))
		}
		return entity.Entities{}, nil, nil, log.CreateError(errors.ErrAppNotFound, "some apps were not found")
	}

	// find namespaces that are used exclusively by the selected apps
	namespaces, err := ExclusiveNamespaces(l, allRenderedEntities, appIds)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	namespaceEntities, err := CreateNamespaceEntities(namespaces, types.KeyTemplateEntity)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	// contains only the rendered resources from the apps selected for uninstallation
	_, renderedEntities, err := allRenderedEntities.SelectByAppIds(appIds)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	namespaceEntities, err = WithoutDuplicateSyntheticKubernetesDefaults(l, renderedEntities, namespaceEntities)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	renderedEntities, err = renderedEntities.Append(namespaceEntities)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	// contains scope infos extracted from api server, including default resources
	clusterScopeInfoMap, err := ScopeInfoMapFromCluster(cluster, types.KeyClusterEntity, crdMode)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	// CRD-derived scope from all cluster-rendered apps so selected manifests can resolve custom types
	// whose CRD manifests live in non-selected apps.
	crdScopeMap, err := ClusterApplyCrdScopeMap(allRenderedEntities, clusterScopeInfoMap, types.KeyTemplateEntity)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	scopeInfoMaps := append([]types.ScopeInfoMap{crdScopeMap}, additionalScopeInfo...)
	renderedEntities, err = ApplyScopeInfoMaps(crdMode, renderedEntities, types.KeyTemplateEntity, scopeInfoMaps...)
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	renderedEntities, err = NormalizeApiVersions(l, renderedEntities, types.KeyTemplateEntity, cluster, func() (types.ScopeInfoMap, error) {
		return clusterScopeInfoMap, nil
	})
	if err != nil {
		return entity.Entities{}, nil, nil, err
	}

	return renderedEntities, namespaces, allAppIds, nil
}

func findGitRoot(startPath string) (string, error) {
	dir, err := filepath.Abs(startPath)
	if err != nil {
		return "", err
	}

	for {
		if info, err := os.Stat(filepath.Join(dir, ".git")); err == nil && info.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no git root found starting from %s", startPath)
		}
		dir = parent
	}
}

// enrichEntityPaths computes RepoPath and AbsPath from the entity's TemplatePath and AppId,
// combined with the cluster path and git root.
func enrichEntityPaths(e entity.Entity, clusterPath string, gitRoot string) (entity.Entity, error) {
	templatePath, err := e.TemplatePath()
	if err != nil {
		return e, nil
	}

	appIds, err := e.AppIds()
	if err != nil || len(appIds) == 0 {
		return e, nil
	}
	appId := appIds[0]

	var absPath string
	rootAppName, err := appId.RootAppName()
	if err != nil {
		return e, nil
	}
	// Helm subchart paths often repeat the root app directory (e.g. "argocd/templates/...").
	// Joining clusterPath + rootAppName + that path would yield ".../argocd/argocd/templates/..."
	// and break SOPS file decryption (bootstrap falls back to re-marshalled YAML → MAC mismatch).
	tp := string(templatePath)
	if stripped, ok := strings.CutPrefix(tp, string(rootAppName)+"/"); ok {
		tp = stripped
	}
	absPath = filepath.Join(clusterPath, string(rootAppName), tp)

	return e.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
		b = b.WithAbsPath(types.AbsPath(absPath))
		if gitRoot != "" {
			if rel, relErr := filepath.Rel(gitRoot, absPath); relErr == nil {
				b = b.WithRepoPath(types.RepoPath(rel))
			}
		}
		return b
	})
}
