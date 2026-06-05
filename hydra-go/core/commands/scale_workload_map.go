package commands

import (
	"cmp"
	"fmt"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func scaleMapFromHydraValues(hv *types.HydraValues) map[types.GVKString]types.HydraScaleGroup {
	if hv == nil || len(hv.Scale) == 0 {
		return nil
	}
	out := make(map[types.GVKString]types.HydraScaleGroup, len(hv.Scale))
	for _, g := range hv.Scale {
		if g.GVK == "" {
			continue
		}
		out[types.GVKString(g.GVK)] = g
	}
	return out
}

// ScaleWorkloadMap loads the scale workload map from global.hydra.scale in Hydra values.
func ScaleWorkloadMap(h hydra.Hydra, networkMode types.HelmNetworkMode) (map[types.GVKString]types.HydraScaleGroup, error) {
	hv, err := hydra.HydraValues(h, networkMode)
	if err != nil {
		return nil, err
	}
	m := scaleMapFromHydraValues(hv)
	if m == nil {
		return make(map[types.GVKString]types.HydraScaleGroup), nil
	}
	return m, nil
}

// extractScaleFromMergedMap reads global.hydra.scale from a merged hydra fragment (Helm + ConfigMap docs).
func extractScaleFromMergedMap(merged types.ValuesMap) map[string]types.HydraScaleGroup {
	if merged == nil {
		return nil
	}
	scaleVal, ok := merged["scale"]
	if !ok || scaleVal == nil {
		return nil
	}
	ys, err := yaml.ToYaml(scaleVal)
	if err != nil {
		return nil
	}
	m, err := yaml.FromYaml[map[string]types.HydraScaleGroup](ys)
	if err != nil {
		return nil
	}
	return m
}

func scaleGVKMapFromNamedGroups(named map[string]types.HydraScaleGroup) map[types.GVKString]types.HydraScaleGroup {
	if len(named) == 0 {
		return nil
	}
	out := make(map[types.GVKString]types.HydraScaleGroup, len(named))
	for _, g := range named {
		if g.GVK == "" {
			continue
		}
		out[types.GVKString(g.GVK)] = g
	}
	return out
}

func mergeScaleGVKMap(dst map[types.GVKString]types.HydraScaleGroup, src map[types.GVKString]types.HydraScaleGroup) {
	for k, v := range src {
		dst[k] = v
	}
}

// MergedScaleWorkloadMap builds the effective global.hydra.scale map for cluster scale / apply when a set
// of apps is selected. It merges, in order (later overrides same GVK): (1) cluster-level values.yaml
// global.hydra.scale via [ScaleWorkloadMap] on the cluster, (2) for each selected app id (sorted),
// Helm global.hydra for that app merged with Hydra ConfigMap documents from rendered manifests (same
// merge model as ref-parsers), and (3) app-independent global Hydra ConfigMap documents from the render.
func MergedScaleWorkloadMap(
	cluster *hydra.Cluster,
	appIds sets.Set[types.AppId],
	networkMode types.HelmNetworkMode,
	rendered entity.Entities,
) (map[types.GVKString]types.HydraScaleGroup, error) {
	out := make(map[types.GVKString]types.HydraScaleGroup)

	base, err := ScaleWorkloadMap(cluster, networkMode)
	if err != nil {
		return nil, err
	}
	mergeScaleGVKMap(out, base)

	perApp, global, err := hydra.PartitionHydraConfigDocumentsByApp(rendered, types.KeyTemplateEntity, appIds)
	if err != nil {
		return nil, err
	}

	appIdSlice := make([]types.AppId, 0, appIds.Len())
	for id := range appIds {
		appIdSlice = append(appIdSlice, id)
	}
	slices.SortFunc(appIdSlice, func(a, b types.AppId) int {
		return cmp.Compare(string(a), string(b))
	})

	for _, appId := range appIdSlice {
		h, err := cluster.WithApp(appId)
		if err != nil {
			return nil, err
		}
		hv, err := hydra.HydraValues(h, networkMode)
		if err != nil {
			return nil, err
		}
		helmMap, err := hydra.HelmHydraMapFromValues(hv)
		if err != nil {
			return nil, err
		}
		docs := hydra.HydraConfigMapDocumentsForApp(perApp, global, appIds, appId)
		merged := hydra.MergeHelmHydraWithConfigMapDocuments(helmMap, docs)
		named := extractScaleFromMergedMap(merged)
		mergeScaleGVKMap(out, scaleGVKMapFromNamedGroups(named))
	}

	return out, nil
}

// shouldSkipOperatorStatefulSet checks whether a StatefulSet is managed by an operator CR
// that appears in the scale map. It checks the provided unstructured first (typically the
// template view) and falls back to the live cluster view (KeyClusterEntity) for
// ownerReferences, which are only set at runtime by the Kubernetes API server.
func shouldSkipOperatorStatefulSet(
	entities entity.Entities,
	item entity.Entity,
	u unstructured.Unstructured,
	key types.EntityKeyUnstructured,
	scaleMap map[types.GVKString]types.HydraScaleGroup,
) bool {
	if ShouldSkipOperatorManagedStatefulSet(entities, u, scaleMap) {
		return true
	}
	if key == types.KeyClusterEntity {
		return false
	}
	liveU, ok := item.Unstructured(types.KeyClusterEntity)
	if !ok {
		return false
	}
	return ShouldSkipOperatorManagedStatefulSet(entities, liveU, scaleMap)
}

// ShouldSkipOperatorManagedStatefulSet returns true when this StatefulSet is owned by a
// CR that is present in entities and listed in the scale map, so scaling should target the
// CR instead of the StatefulSet.
func ShouldSkipOperatorManagedStatefulSet(
	entities entity.Entities,
	sts unstructured.Unstructured,
	scaleMap map[types.GVKString]types.HydraScaleGroup,
) bool {
	if scaleMap == nil {
		return false
	}
	ns := types.Namespace(sts.GetNamespace())
	for _, owner := range sts.GetOwnerReferences() {
		if owner.Controller == nil || !*owner.Controller {
			continue
		}
		gvkK8s := schema.FromAPIVersionAndKind(owner.APIVersion, owner.Kind)
		gvkStr := types.NewGVKFromK8s(gvkK8s).GVKString()
		if _, ok := scaleMap[gvkStr]; !ok {
			continue
		}
		id, err := idFromOwnerReference(owner, ns)
		if err != nil {
			continue
		}
		if _, exists := entities.EntityMap[id]; exists {
			return true
		}
	}
	return false
}

// idFromOwnerReference builds a Hydra entity Id from a Kubernetes owner reference.
func idFromOwnerReference(owner metav1.OwnerReference, ns types.Namespace) (types.Id, error) {
	gv, err := schema.ParseGroupVersion(owner.APIVersion)
	if err != nil {
		return "", err
	}
	if gv.Group == "" {
		return types.Id(fmt.Sprintf("%s/%s/%s/%s", gv.Version, owner.Kind, ns, owner.Name)), nil
	}
	return types.Id(fmt.Sprintf("%s/%s/%s/%s/%s", gv.Group, gv.Version, owner.Kind, ns, owner.Name)), nil
}
