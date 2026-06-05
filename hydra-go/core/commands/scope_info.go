package commands

import (
	"fmt"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func DefaultScopeInfoMap() types.ScopeInfoMap {
	return types.DefaultScopeInfoMap()
}

func MergeScopeInfoMaps(
	maps ...types.ScopeInfoMap,
) (types.ScopeInfoMap, error) {
	merged := types.ScopeInfoMap{}
	for _, m := range maps {
		for k, v := range m {
			if _, exists := merged[k]; exists {
				continue
			}
			merged[k] = v
		}
	}
	return merged, nil
}

func ScopeInfoMapFromCluster(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	crdMode types.CrdMode,
) (types.ScopeInfoMap, error) {
	if cluster.Config().KubernetesConnectionAllowed() == types.KubernetesConnectionAllowedNo {
		return types.ScopeInfoMap{}, nil
	}

	cluster.L().DebugLog(logIdCommands, "detecting resource scopes of cluster '{cluster}'",
		log.String("cluster", string(cluster.ClusterName)))

	// VisitResources lists GVKs in parallel when EffectiveClusterWorkerParallelism(0) > 1; guard map writes.
	var scopeMu sync.Mutex
	scopeInfoMap := types.ScopeInfoMap{}
	handlers := &VisitorHandlers{
		HandleClusterResource: func(e entity.Entity, r *metav1.APIResource) (bool, error) {
			gvk, err := e.GVKString()
			if err != nil {
				return false, err
			}
			group, err := e.Group()
			if err != nil {
				return false, err
			}
			version, err := e.Version()
			if err != nil {
				return false, err
			}
			kind, err := e.Kind()
			if err != nil {
				return false, err
			}
			resource, err := e.Resource()
			if err != nil {
				return false, err
			}
			scopeMu.Lock()
			scopeInfoMap[gvk] = types.ScopeInfo{
				Group:      group,
				Version:    version,
				Kind:       kind,
				Namespaced: false,
				Resource:   resource,
			}
			scopeMu.Unlock()
			return false, nil
		},
		HandleNamespacedResource: func(e entity.Entity, r *metav1.APIResource) (bool, error) {
			gvk, err := e.GVKString()
			if err != nil {
				return false, err
			}
			group, err := e.Group()
			if err != nil {
				return false, err
			}
			version, err := e.Version()
			if err != nil {
				return false, err
			}
			kind, err := e.Kind()
			if err != nil {
				return false, err
			}
			resource, err := e.Resource()
			if err != nil {
				return false, err
			}
			scopeMu.Lock()
			scopeInfoMap[gvk] = types.ScopeInfo{
				Group:      group,
				Version:    version,
				Kind:       kind,
				Namespaced: true,
				Resource:   resource,
			}
			scopeMu.Unlock()
			return false, nil
		},
	}

	_, err := VisitResources(cluster, key, handlers, crdMode == types.CrdModeSilent, false, 0)
	if err != nil {
		return nil, err
	}

	return scopeInfoMap, nil
}

func ApplyScopeInfoMaps(
	crdMode types.CrdMode,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	scopeInfoMaps ...types.ScopeInfoMap,
) (entity.Entities, error) {
	mergedScopeInfoMap, err := MergeScopeInfoMaps(scopeInfoMaps...)
	if err != nil {
		return entity.Entities{}, err
	}
	return applyScopeInfoMap(crdMode, entities, mergedScopeInfoMap, key)
}

func NormalizeApiVersions(
	l log.Logger,
	entities entity.Entities,
	key types.EntityKeyUnstructured,
	cluster *hydra.Cluster,
	computeScope func() (types.ScopeInfoMap, error),
) (entity.Entities, error) {
	if cluster == nil {
		return entities, nil
	}
	preferredVersions, err := cluster.PreferredVersions(computeScope)
	if err != nil {
		return entity.Entities{}, err
	}
	if len(preferredVersions) == 0 {
		return entities, nil
	}

	updated := make([]entity.Entity, 0, entities.Len())
	for _, e := range entities.Items {
		group, groupErr := e.Group()
		version, versionErr := e.Version()
		kind, kindErr := e.Kind()
		if groupErr != nil || versionErr != nil || kindErr != nil {
			updated = append(updated, e)
			continue
		}

		gkKey := types.NewGroupKindKey(group, kind)
		preferredVersion, found := preferredVersions[gkKey]
		if !found || preferredVersion == version {
			updated = append(updated, e)
			continue
		}

		gvkStr, gvkErr := e.GVKString()
		if gvkErr == nil && gvkStr != "" {
			dedupKey := hydra.ApiVersionNormalizationDedupKey(string(gvkStr), preferredVersion)
			if !cluster.HasLoggedApiVersionNormalization(dedupKey) {
				l.Warn(logIdCommands, "normalizing API version of {gvk} from {oldVersion} to {preferredVersion}",
					log.String("gvk", string(gvkStr)),
					log.String("oldVersion", string(version)),
					log.String("preferredVersion", string(preferredVersion)))
				cluster.RememberLoggedApiVersionNormalization(dedupKey)
			}
		}

		b := e.ToBuilder().WithVersion(preferredVersion)

		if u, ok := e.Unstructured(key); ok {
			apiVersionStr, err := b.ApiVersionString()
			if err == nil {
				u.SetAPIVersion(apiVersionStr)
				b = b.WithUnstructured(key, u)
			}
		}

		rebuilt, err := b.Build()
		if err != nil {
			return entity.Entities{}, err
		}
		updated = append(updated, rebuilt)
	}

	return entity.NewEntities(updated)
}

// NormalizePerAppTemplateEntities runs [NormalizeApiVersions] on each app's standalone render so
// template resource IDs use the cluster's preferred API versions. This aligns the ref-ownership
// template id map with [ListClusterAll] identities (same scope callback as cluster review source
// filtering).
func NormalizePerAppTemplateEntities(
	l log.Logger,
	cluster *hydra.Cluster,
	perAppRendered map[types.AppId]entity.Entities,
	key types.EntityKeyUnstructured,
	computeScope func() (types.ScopeInfoMap, error),
) (map[types.AppId]entity.Entities, error) {
	if cluster == nil {
		return perAppRendered, nil
	}
	out := make(map[types.AppId]entity.Entities, len(perAppRendered))
	for appId, ents := range perAppRendered {
		normalized, err := NormalizeApiVersions(l, ents, key, cluster, computeScope)
		if err != nil {
			return nil, err
		}
		out[appId] = normalized
	}
	return out, nil
}

func ApplyScopeInfoMap(
	crdMode types.CrdMode,
	entities entity.Entities,
	scopeInfoMap types.ScopeInfoMap,
	key types.EntityKeyUnstructured,
	requiredGVKs ...sets.Set[types.GVKString],
) (entity.Entities, error) {
	return applyScopeInfoMap(crdMode, entities, scopeInfoMap, key, requiredGVKs...)
}

// applyScopeInfoMap applies scope metadata and app-namespace normalization for template entities.
// Internal CrdModes may keep unknown GVKs unchanged instead of dropping or rejecting them.
func applyScopeInfoMap(
	crdMode types.CrdMode,
	entities entity.Entities,
	scopeInfoMap types.ScopeInfoMap,
	key types.EntityKeyUnstructured,
	requiredGVKs ...sets.Set[types.GVKString],
) (entity.Entities, error) {
	l := log.Default()
	l.DebugLog(logIdCommands, "applying default namespace to rendered resources")

	resolved := []entity.Entity{}

	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}

		info, found := scopeInfoMap[gvk]
		if !found {
			if crdMode == types.CrdModeKeepUnknown {
				resolved = append(resolved, e)
				continue
			}
			if crdMode == types.CrdModeSilent {
				continue
			}
			if crdMode == types.CrdModeIgnoreOptional {
				if len(requiredGVKs) > 0 && requiredGVKs[0].Has(gvk) {
					return entity.Entities{}, log.CreateError(errors.ErrRequiredCrdMissing,
						"CRD for {gvk} is required by global.hydra.scale but not found in cluster",
						log.String("gvk", string(gvk)))
				}
				continue
			}
			if crdMode == types.CrdModeIgnore {
				l.Warn(logIdCommands, "skipping entity with unknown scope for {gvk}",
					log.String("gvk", string(gvk)))
				continue
			}
			return entity.Entities{}, log.CreateError(errors.ErrMissingScope, "could not determine scope for entity {gvk}",
				log.String("gvk", string(gvk)))
		}

		resource, err := e.Resource()
		if err == nil {
			if info.Resource != resource {
				if crdMode == types.CrdModeSilent {
					continue
				}
				if crdMode == types.CrdModeIgnoreOptional {
					if len(requiredGVKs) > 0 && requiredGVKs[0].Has(gvk) {
						return entity.Entities{}, log.CreateError(errors.ErrRequiredCrdMissing,
							"CRD for {gvk} is required by global.hydra.scale but not found in cluster",
							log.String("gvk", string(gvk)))
					}
					continue
				}
				if crdMode == types.CrdModeIgnore {
					l.Warn(logIdCommands, "skipping entity with unknown scope for {gvk}",
						log.String("gvk", string(gvk)))
					continue
				}
				return entity.Entities{}, log.CreateError(errors.ErrMissingScope, "could not determine scope for entity {gvk}",
					log.String("gvk", string(gvk)))
			}
		}

		b := e.ToBuilder()
		if err != nil {
			b = b.WithResource(info.Resource)
		}

		if info.Namespaced == types.NamespacedNo {
			b = b.WithNamespaced(types.NamespacedNo)
			if !b.HasNamespace() {
				appNamespace, err := b.AppNamespace()
				if err != nil {
					return entity.Entities{}, log.CreateError(errors.ErrNamespaceNotDefined, "could not determine app namespace for entity {gvk}",
						log.String("gvk", string(gvk)))
				}
				b = b.WithNamespace(types.Namespace(appNamespace))
			}
			ns, _ := b.Namespace()
			if u, ok := b.Unstructured(key); ok {
				if u.GetNamespace() == "" && ns != "" {
					u.SetNamespace(string(ns))
					b = b.WithUnstructured(key, u)
				}
			}
		} else {
			if namespace, ok := entityManifestNamespace(e, key); ok {
				return entity.Entities{}, log.CreateError(errors.ErrInvalidResourceScope,
					"cluster-scoped entity {gvk} must not set metadata.namespace {namespace}; remove metadata.namespace from {source}",
					log.String("gvk", string(gvk)),
					log.String("namespace", string(namespace)),
					log.String("source", entityTemplateSource(e)))
			}
			b = b.WithNamespaced(types.NamespacedYes)
		}
		rebuilt, err := b.Build()
		if err != nil {
			return entity.Entities{}, err
		}
		resolved = append(resolved, rebuilt)
	}

	return entity.NewEntities(resolved)
}

func entityManifestNamespace(e entity.Entity, key types.EntityKeyUnstructured) (types.Namespace, bool) {
	if u, ok := e.Unstructured(key); ok {
		if ns := u.GetNamespace(); ns != "" {
			return types.Namespace(ns), true
		}
		return "", false
	}
	if e.HasNamespace() {
		ns, err := e.Namespace()
		if err == nil && ns != "" {
			return ns, true
		}
	}
	return "", false
}

func entityTemplateSource(e entity.Entity) string {
	source := "(unknown template)"
	if path, err := e.TemplatePath(); err == nil && path != "" {
		source = string(path)
	}
	if index, err := e.TemplateIndex(); err == nil && index > 0 {
		source += fmt.Sprintf("#%d", index)
	}
	return source
}
