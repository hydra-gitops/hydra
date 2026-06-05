package commands

import (
	"context"
	"encoding/json"
	"time"

	herrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

func isClusterOnlyWorkloadGVK(gvk types.GVKString) bool {
	switch gvk {
	case types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1ReplicaSet,
		types.KubernetesGvkAppsV1StatefulSet,
		types.KubernetesGvkAppsV1DaemonSet:
		return true
	default:
		return false
	}
}

// isBuiltinTemplateParentWorkloadGVK identifies workload controllers rendered from app templates whose
// scale-down is handled by the main template workload path; their child ReplicaSets must not be
// scaled as cluster-only workloads.
func isBuiltinTemplateParentWorkloadGVK(gvk types.GVKString) bool {
	switch gvk {
	case types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1StatefulSet,
		types.KubernetesGvkAppsV1DaemonSet:
		return true
	default:
		return false
	}
}

// entityOwnedByLiveTemplateBuiltinWorkload returns true when some ownerReference chain reaches a
// live object whose UID is in liveTemplateUids and whose kind is Deployment, StatefulSet, or
// DaemonSet (app-managed workload). ReplicaSets under a templated Deployment match; a StatefulSet
// owned only by a template CR does not.
func entityOwnedByLiveTemplateBuiltinWorkload(
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
			ownerEntity, ok := ownerUidMap[uid]
			if !ok {
				return false
			}
			gvk, err := ownerEntity.GVKString()
			if err != nil {
				return false
			}
			return isBuiltinTemplateParentWorkloadGVK(gvk)
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

// clusterOnlyWorkloadExemptFromBuiltinExpectedIDs reports true when the workload id is in exempt
// (Kubernetes builtin + cluster-defaults preset audit ids) or is owned by such a resource
// (e.g. ReplicaSet under apps/v1/Deployment/.../coredns listed in the coredns preset).
func clusterOnlyWorkloadExemptFromBuiltinExpectedIDs(
	item entity.Entity,
	liveKey types.EntityKeyUnstructured,
	id types.Id,
	exempt sets.Set[types.Id],
	ownerUidMap map[types.Uid]entity.Entity,
) bool {
	if exempt == nil || exempt.Len() == 0 {
		return false
	}
	if exempt.Has(id) {
		return true
	}
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
		ownerEntity, ok := ownerUidMap[uid]
		if !ok {
			return false
		}
		ownerID, err := ownerEntity.Id()
		if err == nil && exempt.Has(ownerID) {
			return true
		}
		ownerObj, ok := ownerEntity.Unstructured(liveKey)
		if !ok {
			return false
		}
		for _, next := range ownerObj.GetOwnerReferences() {
			if walk(next) {
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

func entityAnchoredInLiveTemplateSet(
	item entity.Entity,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	liveTemplateUids sets.Set[types.Uid],
) bool {
	if _, ok := item.Unstructured(templateKey); !ok {
		return false
	}
	liveU, ok := item.Unstructured(liveKey)
	if !ok {
		return false
	}
	uid := types.Uid(liveU.GetUID())
	return uid != "" && liveTemplateUids.Has(uid)
}

// clusterOnlyWorkloadLinkedByTemplateRef returns true when a Hydra ref connects a template-anchored
// entity (present in template and live with a UID in liveTemplateUids) to this workload id.
// Normalization matches scale dependency edges: when ref.Reverse is true, From/To are swapped.
func clusterOnlyWorkloadLinkedByTemplateRef(
	workloadId types.Id,
	entities entity.Entities,
	refs []types.Ref,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	liveTemplateUids sets.Set[types.Uid],
) bool {
	if len(refs) == 0 {
		return false
	}
	for _, ref := range refs {
		from, to := ref.From, ref.To
		if ref.Reverse {
			from, to = to, from
		}
		if to != workloadId {
			continue
		}
		fromEnt, ok := entities.EntityMap[from]
		if !ok {
			continue
		}
		if entityAnchoredInLiveTemplateSet(fromEnt, templateKey, liveKey, liveTemplateUids) {
			return true
		}
	}
	return false
}

func clusterOnlyWorkloadNeedsScaleDown(u unstructured.Unstructured, gvk types.GVKString) bool {
	if gvk == types.KubernetesGvkAppsV1DaemonSet {
		ns := nodeSelectorFromObject(&u)
		return len(ns) != 1 || ns[hydra.AnnotationHydraScaleDisabled] != "true"
	}
	replicas := values.Lookup(u.Object, "spec", "replicas")
	if replicas == nil {
		return true
	}
	v, ok := toInt64(replicas)
	return !ok || v != 0
}

func collectClusterOnlyWorkloadEntities(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	refs []types.Ref,
	exemptBuiltinAndPresetWorkloadIDs sets.Set[types.Id],
) ([]entity.Entity, error) {
	liveTemplateUids := collectLiveTemplateUids(entities, templateKey, liveKey)
	ownerUidMap := entities.UidMap(liveKey)

	var out []entity.Entity
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}
		if !isClusterOnlyWorkloadGVK(gvk) {
			continue
		}
		if _, ok := item.Unstructured(templateKey); ok {
			continue
		}
		liveU, ok := item.Unstructured(liveKey)
		if !ok {
			continue
		}
		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		linkedByOwner := entityOwnedByLiveTemplateEntity(item, liveKey, liveTemplateUids, ownerUidMap)
		linkedByRef := clusterOnlyWorkloadLinkedByTemplateRef(id, entities, refs, templateKey, liveKey, liveTemplateUids)
		if !linkedByOwner && !linkedByRef {
			continue
		}
		if entityOwnedByLiveTemplateBuiltinWorkload(item, liveKey, liveTemplateUids, ownerUidMap) {
			continue
		}
		if !clusterOnlyWorkloadNeedsScaleDown(liveU, gvk) {
			continue
		}
		if clusterOnlyWorkloadExemptFromBuiltinExpectedIDs(item, liveKey, id, exemptBuiltinAndPresetWorkloadIDs, ownerUidMap) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

// ClusterOnlyScaleDownWillMutate reports whether ScaleDownClusterOnlyWorkloads would apply patches.
func ClusterOnlyScaleDownWillMutate(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	refs []types.Ref,
	exemptBuiltinAndPresetWorkloadIDs sets.Set[types.Id],
) (bool, error) {
	items, err := collectClusterOnlyWorkloadEntities(entities, templateKey, liveKey, refs, exemptBuiltinAndPresetWorkloadIDs)
	if err != nil {
		return false, err
	}
	return len(items) > 0, nil
}

func patchClusterOnlyWorkloadDown(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	e entity.Entity,
	dryRun types.DryRun,
) error {
	gvk, err := e.GVKString()
	if err != nil {
		return err
	}
	id, err := e.Id()
	if err != nil {
		return err
	}
	name, err := e.Name()
	if err != nil {
		return err
	}
	ns, _ := e.Namespace()
	gvr, err := e.GVR()
	if err != nil {
		return err
	}

	drp := ""
	if dryRun {
		drp = "[dry-run] "
	}

	client := resourceClient(dynamicClient, ns, gvr)
	n := string(name)

	if gvk == types.KubernetesGvkAppsV1DaemonSet {
		l.Info(logIdCommands, drp+"disabling cluster-only daemonset {name}", log.String("name", n))
		if dryRun {
			return nil
		}
		patchData, mErr := json.Marshal(map[string]any{
			"spec": map[string]any{
				"template": map[string]any{
					"spec": map[string]any{
						"nodeSelector": map[string]string{hydra.AnnotationHydraScaleDisabled: "true"},
					},
				},
			},
		})
		if mErr != nil {
			return mErr
		}
		_, err = client.Patch(ctx, n, ktypes.MergePatchType, patchData, metav1.PatchOptions{})
	} else {
		l.Info(logIdCommands, drp+"scaling down cluster-only workload {id} to 0 replicas", log.String("id", string(id)))
		if dryRun {
			return nil
		}
		_, err = client.Patch(ctx, n, ktypes.MergePatchType,
			[]byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{})
	}
	if err != nil {
		if errors.IsNotFound(err) {
			l.DebugLog(logIdCommands, "{name} not found, skipping cluster-only scale", log.String("name", n))
			return nil
		}
		return err
	}
	return nil
}

func podsStillRunningForScaledWorkloads(
	pods []unstructured.Unstructured,
	scaledUids sets.Set[types.Uid],
) int {
	n := 0
	for _, p := range pods {
		if !podStillPresent(p) {
			continue
		}
		for _, ref := range p.GetOwnerReferences() {
			if scaledUids.Has(types.Uid(ref.UID)) {
				n++
				break
			}
		}
	}
	return n
}

func waitClusterOnlyPodsGone(
	ctx context.Context,
	l log.Logger,
	listPods func(context.Context) ([]unstructured.Unstructured, error),
	scaledUids sets.Set[types.Uid],
	timeout time.Duration,
) error {
	if scaledUids.Len() == 0 {
		return nil
	}
	poll := func() (int, error) {
		listed, err := listPods(ctx)
		if err != nil {
			return 0, err
		}
		return podsStillRunningForScaledWorkloads(listed, scaledUids), nil
	}
	if remaining, err := poll(); err != nil {
		return err
	} else if remaining == 0 {
		l.DebugLog(logIdCommands, "cluster-only workload pods scaled down")
		return nil
	}

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	deadline := time.After(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline:
			remaining, err := poll()
			if err != nil {
				return err
			}
			if remaining == 0 {
				return nil
			}
			l.Warn(logIdCommands, "cluster-only workload pods still running after {timeout} ({count} pods)",
				log.String("timeout", timeout.String()), log.Int("count", remaining))
			return log.CreateError(herrors.ErrClusterWorkloadWaitTimeout,
				"aborted: cluster-only workload pods did not terminate within {timeout}",
				log.String("timeout", timeout.String()))
		case <-ticker.C:
			remaining, err := poll()
			if err != nil {
				return err
			}
			if remaining == 0 {
				l.DebugLog(logIdCommands, "cluster-only workload pods scaled down")
				return nil
			}
			l.DebugLog(logIdCommands, "waiting for cluster-only workload pods to terminate ({count} remaining)",
				log.Int("count", remaining))
		}
	}
}

// ScaleDownClusterOnlyWorkloads scales built-in workloads that exist on the cluster but not in
// rendered templates (e.g. operator-created StatefulSets), when they are linked to the app via
// transitive ownerReferences or via a Hydra ref from a template-anchored entity to the workload
// (same From/To normalization as scale dependencies, including ref.Reverse). Runs after template
// workload scale-down and before pod reconciliation.
//
// When forceScaleDown is true, patches are applied and no post-patch wait runs.
// When false, waits up to clusterWorkloadTimeout for pods owned by the scaled workloads to exit.
func ScaleDownClusterOnlyWorkloads(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	refs []types.Ref,
	exemptBuiltinAndPresetWorkloadIDs sets.Set[types.Id],
	dryRun types.DryRun,
	forceScaleDown types.ForceScaleDown,
	clusterWorkloadTimeout time.Duration,
	listPods func(context.Context) ([]unstructured.Unstructured, error),
) (bool, error) {
	targets, err := collectClusterOnlyWorkloadEntities(entities, templateKey, liveKey, refs, exemptBuiltinAndPresetWorkloadIDs)
	if err != nil {
		return false, err
	}
	if len(targets) == 0 {
		return false, nil
	}

	l.Info(logIdCommands, "scaling down {count} cluster-only workloads", log.Int("count", len(targets)))

	scaledUids := sets.New[types.Uid]()
	mutated := false

	for _, e := range targets {
		liveU, ok := e.Unstructured(liveKey)
		if !ok {
			continue
		}
		uid := types.Uid(liveU.GetUID())
		if uid == "" {
			continue
		}

		if err := patchClusterOnlyWorkloadDown(ctx, l, dynamicClient, e, dryRun); err != nil {
			return false, err
		}
		if !dryRun {
			mutated = true
			scaledUids.Insert(uid)
		}
	}

	if bool(dryRun) || !mutated || bool(forceScaleDown) {
		return mutated, nil
	}

	if err := waitClusterOnlyPodsGone(ctx, l, listPods, scaledUids, clusterWorkloadTimeout); err != nil {
		return false, err
	}
	return true, nil
}
