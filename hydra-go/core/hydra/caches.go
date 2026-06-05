package hydra

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/helm"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type valuesCacheKey struct {
	appId       types.AppId
	networkMode types.HelmNetworkMode
}

func (k valuesCacheKey) String() string {
	return fmt.Sprintf("%s:%s", k.appId, k.networkMode.String())
}

func newValuesCacheKey(appId types.AppId, networkMode types.HelmNetworkMode) valuesCacheKey {
	return valuesCacheKey{
		appId:       appId,
		networkMode: networkMode,
	}
}

type templateCacheKey struct {
	appId                       types.AppId
	networkMode                 types.HelmNetworkMode
	kubernetesVersionOrFallback types.KubernetesVersionOrFallback
	skipCrds                    bool
	// valuesDigest is SHA256(hex) over YAML of the ValuesMap passed to helm.Template (see
	// helmChartTemplateValuesDigest). Required so cache invalidates when merged Helm user
	// values change; appId+k8sVersion alone is not enough.
	valuesDigest string
}

func (k templateCacheKey) String() string {
	return fmt.Sprintf("%s:%s:%s:%v:%s", k.appId, k.networkMode.String(), k.kubernetesVersionOrFallback, k.skipCrds, k.valuesDigest)
}

func newTemplateCacheKey(
	appId types.AppId,
	networkMode types.HelmNetworkMode,
	kubernetesVersionOrFallback types.KubernetesVersionOrFallback,
	skipCrds bool,
	valuesDigest string,
) templateCacheKey {
	return templateCacheKey{
		appId:                       appId,
		networkMode:                 networkMode,
		kubernetesVersionOrFallback: kubernetesVersionOrFallback,
		skipCrds:                    skipCrds,
		valuesDigest:                valuesDigest,
	}
}

type valuesCache = cache.Cache[valuesCacheKey, types.ValuesMap]
type templateCache = cache.Cache[templateCacheKey, types.YamlString]

type hydraCaches struct {
	l                        log.Logger
	chartCache               *helm.ChartCache
	valuesCache              *valuesCache
	templatesCache           *templateCache
	helmTemplateCacheEnabled bool
}

func newHydraCaches(l log.Logger, helmTemplateCacheEnabled bool) *hydraCaches {
	cc := helm.NewChartCache(l)
	cc.SetDisabled(!helmTemplateCacheEnabled)
	return &hydraCaches{
		l:                        l,
		chartCache:               cc,
		valuesCache:              cache.NewCache(l, "values", false, onValuesStored),
		templatesCache:           cache.NewCache(l, "templates", false, onTemplateStored),
		helmTemplateCacheEnabled: helmTemplateCacheEnabled,
	}
}

func (hc *hydraCaches) getOrLoadValues(key valuesCacheKey, load func() (types.ValuesMap, error)) (types.ValuesMap, error) {
	if !hc.helmTemplateCacheEnabled {
		return load()
	}
	return hc.valuesCache.GetOrLoad(key, load)
}

func (hc *hydraCaches) getOrLoadTemplates(key templateCacheKey, load func() (types.YamlString, error)) (types.YamlString, error) {
	if !hc.helmTemplateCacheEnabled {
		return load()
	}
	return hc.templatesCache.GetOrLoad(key, load)
}

func onValuesStored(c *cache.Cache[valuesCacheKey, types.ValuesMap], key valuesCacheKey, value types.ValuesMap, err error) {
	if key.networkMode == types.HelmNetworkModeOffline || err != nil {
		return
	}
	key.networkMode = types.HelmNetworkModeOffline
	c.StoreIfAbsent(key, value, err)
}

func onTemplateStored(c *cache.Cache[templateCacheKey, types.YamlString], key templateCacheKey, value types.YamlString, err error) {
	if key.networkMode == types.HelmNetworkModeOffline || err != nil {
		return
	}
	key.networkMode = types.HelmNetworkModeOffline
	c.StoreIfAbsent(key, value, err)
}
