package entity

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type EntityMatchFunc func(Entity) (bool, error)

func entityHasAnyUnstructuredKey(e Entity) bool {
	for key := range types.EntityKeys() {
		uk, ok := key.(types.EntityKeyUnstructured)
		if !ok {
			continue
		}
		if _, found := e.Unstructured(uk); found {
			return true
		}
	}
	return false
}

func StripUnstructuredKeyWhere(
	entities Entities,
	stripKey types.EntityKeyUnstructured,
	match EntityMatchFunc,
) (Entities, error) {
	items := make([]Entity, 0, len(entities.Items))
	for _, e := range entities.Items {
		shouldStrip := true
		var err error
		if match != nil {
			shouldStrip, err = match(e)
			if err != nil {
				return Entities{}, err
			}
		}

		if shouldStrip {
			modified, err := e.Modify(func(b EntityBuilder) EntityBuilder {
				return b.WithoutUnstructured(stripKey)
			})
			if err != nil {
				return Entities{}, err
			}
			e = modified
		}

		if !entityHasAnyUnstructuredKey(e) {
			continue
		}
		items = append(items, e)
	}

	return NewEntities(items)
}

func StripUnstructuredKey(
	entities Entities,
	stripKey types.EntityKeyUnstructured,
) (Entities, error) {
	return StripUnstructuredKeyWhere(entities, stripKey, nil)
}

func resourceForLiveRefresh(gvk types.GVK) (types.Resource, error) {
	switch string(gvk.GVKString()) {
	case string(types.KubernetesGvkV1Pod):
		return types.Resource("pods"), nil
	case string(types.KubernetesGvkAppsV1Deployment):
		return types.Resource("deployments"), nil
	case string(types.KubernetesGvkAppsV1ReplicaSet):
		return types.Resource("replicasets"), nil
	case string(types.KubernetesGvkAppsV1StatefulSet):
		return types.Resource("statefulsets"), nil
	case string(types.KubernetesGvkAppsV1DaemonSet):
		return types.Resource("daemonsets"), nil
	case string(types.KubernetesGvkBatchV1Job):
		return types.Resource("jobs"), nil
	case string(types.KubernetesGvkV1ConfigMap):
		return types.Resource("configmaps"), nil
	case string(types.KubernetesGvkV1Secret):
		return types.Resource("secrets"), nil
	case string(types.KubernetesGvkV1Service):
		return types.Resource("services"), nil
	case string(types.KubernetesGvkV1ServiceAccount):
		return types.Resource("serviceaccounts"), nil
	case string(types.KubernetesGvkV1Namespace):
		return types.Resource("namespaces"), nil
	default:
		return "", fmt.Errorf("unsupported live refresh GVK %q", gvk.GVKString())
	}
}

func entityFromListedUnstructured(
	item unstructured.Unstructured,
	key types.EntityKeyUnstructured,
) (Entity, error) {
	gvk := types.NewGVKFromK8s(item.GroupVersionKind())
	resource, err := resourceForLiveRefresh(gvk)
	if err != nil {
		return Entity{}, err
	}

	builder := NewEntityBuilder().
		WithGVK(gvk).
		WithResource(resource).
		WithName(types.Name(item.GetName()))
	if item.GetNamespace() != "" {
		builder = builder.
			WithNamespace(types.Namespace(item.GetNamespace())).
			WithNamespaced(types.NamespacedNo)
	}

	return builder.WithUnstructured(key, item).Build()
}

func EntitiesFromListedUnstructured(
	items []unstructured.Unstructured,
	key types.EntityKeyUnstructured,
) (Entities, error) {
	result := make([]Entity, 0, len(items))
	for _, item := range items {
		e, err := entityFromListedUnstructured(item, key)
		if err != nil {
			return Entities{}, err
		}
		result = append(result, e)
	}
	return NewEntities(result)
}

func RefreshUnstructuredFromListed(
	entities Entities,
	stripKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	listed []unstructured.Unstructured,
) (Entities, error) {
	stripped, err := StripUnstructuredKey(entities, stripKey)
	if err != nil {
		return Entities{}, err
	}
	liveEntities, err := EntitiesFromListedUnstructured(listed, liveKey)
	if err != nil {
		return Entities{}, err
	}
	return stripped.Merge(liveEntities, liveKey)
}

func StripAndMergeClusterPods(
	entities Entities,
	_ string,
	listed []unstructured.Unstructured,
) (Entities, error) {
	return RefreshUnstructuredFromListed(entities, types.KeyClusterEntity, types.KeyClusterEntity, listed)
}
