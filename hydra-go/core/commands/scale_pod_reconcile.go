package commands

import (
	"context"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	hcel "hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

const refreshAllPodsExpression types.CelPredicate = "gvk == 'v1/Pod'"

func RefreshEntitiesByPredicate(
	entities entity.Entities,
	predicate types.CelPredicate,
	key types.EntityKeyUnstructured,
	load func(types.CelPredicate, types.EntityKeyUnstructured) (entity.Entities, error),
) (entity.Entities, error) {
	env, err := hcel.NewEnv()
	if err != nil {
		return entity.Entities{}, err
	}
	compiled, err := env.CompilePredicate(predicate)
	if err != nil {
		return entity.Entities{}, err
	}

	stripped, err := entity.StripUnstructuredKeyWhere(entities, key, func(item entity.Entity) (bool, error) {
		return compiled.EvalBool(item, types.MissingKeysAccept)
	})
	if err != nil {
		return entity.Entities{}, err
	}

	loaded, err := load(predicate, key)
	if err != nil {
		return entity.Entities{}, err
	}
	return stripped.Merge(loaded, key)
}

func ListAllPods(
	ctx context.Context,
	dynamicClient dynamic.Interface,
) ([]unstructured.Unstructured, error) {
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	list, err := dynamicClient.Resource(podGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return list.Items, nil
}

func RefreshAllPods(
	ctx context.Context,
	_ log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	celExpression string,
	liveKey types.EntityKeyUnstructured,
) (entity.Entities, error) {
	return RefreshEntitiesByPredicate(
		entities,
		types.CelPredicate(celExpression),
		liveKey,
		func(_ types.CelPredicate, key types.EntityKeyUnstructured) (entity.Entities, error) {
			items, err := ListAllPods(ctx, dynamicClient)
			if err != nil {
				return entity.Entities{}, err
			}
			return entity.EntitiesFromListedUnstructured(items, key)
		},
	)
}

func collectTemplatePodIds(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
) (sets.Set[types.Id], error) {
	result := sets.New[types.Id]()
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}
		if gvk != types.KubernetesGvkV1Pod {
			continue
		}
		if _, ok := item.Unstructured(templateKey); !ok {
			continue
		}
		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		result.Insert(id)
	}
	return result, nil
}

func collectLiveTemplateUids(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
) sets.Set[types.Uid] {
	result := sets.New[types.Uid]()
	for _, item := range entities.Items {
		if _, ok := item.Unstructured(templateKey); !ok {
			continue
		}
		uid, ok := item.Uid(liveKey)
		if !ok || uid == "" {
			continue
		}
		result.Insert(uid)
	}
	return result
}

func podStillPresent(u unstructured.Unstructured) bool {
	if u.GetDeletionTimestamp() != nil {
		return true
	}
	phase, found, err := unstructured.NestedString(u.Object, "status", "phase")
	if err != nil || !found {
		return true
	}
	switch phase {
	case "Succeeded", "Failed":
		return false
	default:
		return true
	}
}

func podIsTerminating(u unstructured.Unstructured) bool {
	return u.GetDeletionTimestamp() != nil
}

// scaleDownPodsNeedReconcile returns true when the refreshed entity set contains at least one live Pod
// that would be considered by the scale-down pod reconcile loop (template-direct or app-associated, still present).
func scaleDownPodsNeedReconcile(
	refreshed entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	refs []types.Ref,
) (bool, error) {
	templatePodIds, err := collectTemplatePodIds(refreshed, templateKey)
	if err != nil {
		return false, err
	}
	liveTemplateUids := collectLiveTemplateUids(refreshed, templateKey, liveKey)
	ownerUidMap := refreshed.UidMap(liveKey)
	for _, item := range refreshed.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return false, err
		}
		if gvk != types.KubernetesGvkV1Pod {
			continue
		}
		livePod, ok := item.Unstructured(liveKey)
		if !ok || !podStillPresent(livePod) {
			continue
		}
		id, err := item.Id()
		if err != nil {
			return false, err
		}
		if templatePodIds.Has(id) {
			return true, nil
		}
		if podOwnedByLiveTemplateOrRefBackedClusterWorkload(item, refreshed, refs, templateKey, liveKey, liveTemplateUids, ownerUidMap) {
			return true, nil
		}
	}
	return false, nil
}

// entityOwnedByLiveTemplateEntity returns true when the live object is linked to a rendered
// template entity via ownerReferences (directly or transitively).
func entityOwnedByLiveTemplateEntity(
	item entity.Entity,
	liveKey types.EntityKeyUnstructured,
	liveTemplateUids sets.Set[types.Uid],
	ownerUidMap map[types.Uid]entity.Entity,
) bool {
	u, ok := item.Unstructured(liveKey)
	if !ok {
		return false
	}

	seen := sets.New[types.Uid]()
	var walk func(metav1.OwnerReference) bool
	walk = func(ref metav1.OwnerReference) bool {
		uid := types.Uid(ref.UID)
		if uid == "" || seen.Has(uid) {
			return false
		}
		seen.Insert(uid)
		if liveTemplateUids.Has(uid) {
			return true
		}

		ownerEntity, ok := ownerUidMap[uid]
		if !ok {
			return false
		}
		ownerObj, ok := ownerEntity.Unstructured(liveKey)
		if !ok {
			return false
		}
		for _, ownerRef := range ownerObj.GetOwnerReferences() {
			if walk(ownerRef) {
				return true
			}
		}
		return false
	}

	for _, ownerRef := range u.GetOwnerReferences() {
		if walk(ownerRef) {
			return true
		}
	}
	return false
}

// podOwnedByLiveTemplateOrRefBackedClusterWorkload reports whether a live Pod is app-associated for
// scale-down pod reconciliation: the ownerReference walk reaches a template+live UID, or some owner in
// the transitive chain is a built-in workload whose id is linked from a template-anchored entity via
// Hydra refs (clusterOnlyWorkloadLinkedByTemplateRef — e.g. Zalando Postgres pod → StatefulSet → postgresql CR).
func podOwnedByLiveTemplateOrRefBackedClusterWorkload(
	pod entity.Entity,
	entities entity.Entities,
	refs []types.Ref,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	liveTemplateUids sets.Set[types.Uid],
	ownerUidMap map[types.Uid]entity.Entity,
) bool {
	u, ok := pod.Unstructured(liveKey)
	if !ok {
		return false
	}
	seen := sets.New[types.Uid]()
	var walk func([]metav1.OwnerReference) bool
	walk = func(orefs []metav1.OwnerReference) bool {
		for _, or := range orefs {
			uid := types.Uid(or.UID)
			if uid == "" || seen.Has(uid) {
				continue
			}
			seen.Insert(uid)
			if liveTemplateUids.Has(uid) {
				return true
			}
			ownerEnt, ok := ownerUidMap[uid]
			if !ok {
				continue
			}
			id, err := ownerEnt.Id()
			if err != nil {
				continue
			}
			if clusterOnlyWorkloadLinkedByTemplateRef(id, entities, refs, templateKey, liveKey, liveTemplateUids) {
				return true
			}
			ownerObj, ok := ownerEnt.Unstructured(liveKey)
			if !ok {
				continue
			}
			if walk(ownerObj.GetOwnerReferences()) {
				return true
			}
		}
		return false
	}
	return walk(u.GetOwnerReferences())
}

func deleteLivePod(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	item entity.Entity,
	liveKey types.EntityKeyUnstructured,
	dryRun types.DryRun,
	message string,
	forceDelete bool,
) error {
	u, ok := item.Unstructured(liveKey)
	if !ok {
		return nil
	}
	name := u.GetName()
	namespace := u.GetNamespace()
	if dryRun {
		l.Info(logIdCommands, "[dry-run] would "+message+" {pod} in namespace {ns}",
			log.String("pod", name), log.String("ns", namespace))
		return nil
	}
	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	opts := metav1.DeleteOptions{}
	if forceDelete {
		g := int64(0)
		opts.GracePeriodSeconds = &g
	}
	if err := dynamicClient.Resource(podGVR).Namespace(namespace).Delete(ctx, name, opts); err != nil {
		if errors.IsNotFound(err) {
			l.DebugLog(logIdCommands, "pod {pod} in namespace {ns} not found during scale-down delete (already gone)",
				log.String("pod", name), log.String("ns", namespace))
			return nil
		}
		return err
	}
	if forceDelete {
		l.Info(logIdCommands, "pod force-deleted (--force-scale-down): {pod} in namespace {ns}",
			log.String("pod", name), log.String("ns", namespace))
	} else {
		l.Info(logIdCommands, "pod deleted: {pod} in namespace {ns}",
			log.String("pod", name), log.String("ns", namespace))
	}
	return nil
}

func ReconcileScaleDownPods(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	refs []types.Ref,
	clusterMutated bool,
	forceScaleDown types.ForceScaleDown,
	dryRun types.DryRun,
	listPods func(context.Context) ([]unstructured.Unstructured, error),
) (entity.Entities, error) {
	var listed []unstructured.Unstructured
	var err error
	if listPods != nil {
		listed, err = listPods(ctx)
	} else {
		listed, err = ListAllPods(ctx, dynamicClient)
	}
	if err != nil {
		return entity.Entities{}, err
	}

	refreshed, err := RefreshEntitiesByPredicate(
		entities,
		refreshAllPodsExpression,
		liveKey,
		func(_ types.CelPredicate, key types.EntityKeyUnstructured) (entity.Entities, error) {
			return entity.EntitiesFromListedUnstructured(listed, key)
		},
	)
	if err != nil {
		return entity.Entities{}, err
	}

	if !clusterMutated {
		need, needErr := scaleDownPodsNeedReconcile(refreshed, templateKey, liveKey, refs)
		if needErr != nil {
			return entity.Entities{}, needErr
		}
		if !need {
			return entities, nil
		}
	}

	templatePodIds, err := collectTemplatePodIds(refreshed, templateKey)
	if err != nil {
		return entity.Entities{}, err
	}
	liveTemplateUids := collectLiveTemplateUids(refreshed, templateKey, liveKey)
	ownerUidMap := refreshed.UidMap(liveKey)

	appAssocWarnCount := 0
	for _, item := range refreshed.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}
		if gvk != types.KubernetesGvkV1Pod {
			continue
		}

		livePod, ok := item.Unstructured(liveKey)
		if !ok || !podStillPresent(livePod) {
			continue
		}

		id, err := item.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		name, _ := item.Name()
		namespace, _ := item.Namespace()

		if templatePodIds.Has(id) {
			if err := deleteLivePod(ctx, l, dynamicClient, item, liveKey, dryRun, "delete pod", bool(forceScaleDown)); err != nil {
				return entity.Entities{}, err
			}
			continue
		}

		if !podOwnedByLiveTemplateOrRefBackedClusterWorkload(item, refreshed, refs, templateKey, liveKey, liveTemplateUids, ownerUidMap) {
			continue
		}

		if !forceScaleDown {
			if podIsTerminating(livePod) {
				l.Warn(logIdCommands, "app-associated pod {pod} in namespace {ns} still terminating after scale down (re-run with --force-scale-down to force-delete)",
					log.String("pod", string(name)), log.String("ns", string(namespace)))
			} else {
				l.Warn(logIdCommands, "app-associated pod {pod} in namespace {ns} still present after scale down (re-run with --force-scale-down to delete)",
					log.String("pod", string(name)), log.String("ns", string(namespace)))
			}
			appAssocWarnCount++
			continue
		}

		if err := deleteLivePod(ctx, l, dynamicClient, item, liveKey, dryRun, "force-delete app-associated pod", true); err != nil {
			return entity.Entities{}, err
		}
	}

	if appAssocWarnCount > 0 {
		l.Info(logIdCommands, "hint: use --force-scale-down to delete remaining app-associated pods")
	}

	return refreshed, nil
}

func ReconcileScaleUpPods(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	clusterMutated bool,
	dryRun types.DryRun,
	listPods func(context.Context) ([]unstructured.Unstructured, error),
) (entity.Entities, error) {
	if !clusterMutated {
		return entities, nil
	}

	var refreshed entity.Entities
	var err error
	if listPods == nil {
		refreshed, err = RefreshAllPods(ctx, l, dynamicClient, entities, string(refreshAllPodsExpression), liveKey)
	} else {
		listed, listErr := listPods(ctx)
		if listErr != nil {
			return entity.Entities{}, listErr
		}
		refreshed, err = RefreshEntitiesByPredicate(
			entities,
			refreshAllPodsExpression,
			liveKey,
			func(_ types.CelPredicate, key types.EntityKeyUnstructured) (entity.Entities, error) {
				return entity.EntitiesFromListedUnstructured(listed, key)
			},
		)
	}
	if err != nil {
		return entity.Entities{}, err
	}

	podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
	for _, item := range refreshed.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}
		if gvk != types.KubernetesGvkV1Pod {
			continue
		}
		if _, ok := item.Unstructured(templateKey); !ok {
			continue
		}
		if _, ok := item.Unstructured(liveKey); ok {
			continue
		}

		templatePod, err := item.UnstructuredOrError(templateKey)
		if err != nil {
			return entity.Entities{}, err
		}
		if dryRun {
			l.Info(logIdCommands, "[dry-run] would create pod {pod} in namespace {ns} from template",
				log.String("pod", templatePod.GetName()), log.String("ns", templatePod.GetNamespace()))
			continue
		}
		toCreate := templatePod.DeepCopy()
		toCreate.SetResourceVersion("")
		if _, err := dynamicClient.Resource(podGVR).Namespace(templatePod.GetNamespace()).
			Create(ctx, toCreate, metav1.CreateOptions{}); err != nil {
			return entity.Entities{}, err
		}
	}

	return refreshed, nil
}
