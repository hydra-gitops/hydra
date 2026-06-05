package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	herrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ktypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

// errScaleUpWaitContinue signals the scale-up wait loop to keep polling (workload ready but transitive ready gates pending).
var errScaleUpWaitContinue = errors.New("scale up wait continue")

var scaleUpWaitPollInterval = 2 * time.Second

var scaleUpProgressLogInterval = time.Minute

func newScaleUpProgressLogGate(interval time.Duration) func() bool {
	if interval <= 0 {
		return func() bool { return true }
	}

	var last time.Time
	return func() bool {
		now := time.Now()
		if last.IsZero() || now.Sub(last) >= interval {
			last = now
			return true
		}
		return false
	}
}

func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int64:
		return n, true
	case float64:
		return int64(n), true
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case float32:
		return int64(n), true
	default:
		return 0, false
	}
}

type ScaleDirection string

const (
	ScaleUp   ScaleDirection = "up"
	ScaleDown ScaleDirection = "down"
)

type ScaleTarget struct {
	Id               htypes.Id
	Name             htypes.Name
	Ns               htypes.Namespace
	GVR              htypes.GVR
	GVK              htypes.GVKString
	Replicas         int64
	IsDaemonSet      bool
	IsJob            bool
	NodeSelector     map[string]string
	IsCustomWorkload bool
	ReplicaPaths     []string
	OriginalReplicas map[string]int64
	StatusReadyPath  string
}

type startupExecutionPlan struct {
	combined         []PlanEntry
	requiredEntities entity.Entities
	optionalEntities entity.Entities
	requiredRefs     []htypes.Ref
	optionalRefs     []htypes.Ref
}

func isOwnedByWorkload(u unstructured.Unstructured) bool {
	workloadKinds := map[string]bool{
		"Deployment":  true,
		"StatefulSet": true,
		"DaemonSet":   true,
		"ReplicaSet":  true,
	}
	for _, owner := range u.GetOwnerReferences() {
		if workloadKinds[owner.Kind] {
			return true
		}
	}
	return false
}

func isOwnedByCronJob(u unstructured.Unstructured) bool {
	for _, owner := range u.GetOwnerReferences() {
		if owner.Kind == "CronJob" {
			return true
		}
	}
	return false
}

func CollectScaleTargets(entities entity.Entities, key htypes.EntityKeyUnstructured, customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup) ([]ScaleTarget, error) {
	var merged map[htypes.GVKString]htypes.HydraScaleGroup
	if len(customWorkloads) > 0 && customWorkloads[0] != nil {
		merged = customWorkloads[0]
	}

	workloadGVKs := []htypes.GVKString{
		htypes.KubernetesGvkAppsV1Deployment,
		htypes.KubernetesGvkAppsV1ReplicaSet,
		htypes.KubernetesGvkAppsV1StatefulSet,
		htypes.KubernetesGvkAppsV1DaemonSet,
	}

	var targets []ScaleTarget
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}

		if gvk == htypes.KubernetesGvkBatchV1Job {
			u, err := item.UnstructuredOrError(key)
			if err != nil {
				continue
			}
			if isOwnedByCronJob(u) {
				continue
			}

			id, err := item.Id()
			if err != nil {
				return nil, err
			}
			name, err := item.Name()
			if err != nil {
				return nil, err
			}
			ns, _ := item.Namespace()
			gvr, err := item.GVR()
			if err != nil {
				return nil, err
			}

			targets = append(targets, ScaleTarget{
				Id:    id,
				Name:  name,
				Ns:    ns,
				GVR:   gvr,
				GVK:   gvk,
				IsJob: true,
			})
			continue
		}

		if !slices.Contains(workloadGVKs, gvk) {
			continue
		}

		u, err := item.UnstructuredOrError(key)
		if err != nil {
			continue
		}

		if gvk == htypes.KubernetesGvkAppsV1StatefulSet {
			if shouldSkipOperatorStatefulSet(entities, item, u, key, merged) {
				continue
			}
		}

		if isOwnedByWorkload(u) {
			continue
		}

		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		name, err := item.Name()
		if err != nil {
			return nil, err
		}
		ns, _ := item.Namespace()
		gvr, err := item.GVR()
		if err != nil {
			return nil, err
		}

		target := ScaleTarget{
			Id:   id,
			Name: name,
			Ns:   ns,
			GVR:  gvr,
			GVK:  gvk,
		}

		if gvk == htypes.KubernetesGvkAppsV1DaemonSet {
			target.IsDaemonSet = true
			target.Replicas = 0

			nsRaw := values.Lookup(u.Object, "spec", "template", "spec", "nodeSelector")
			if nsRaw != nil {
				if nsMap, ok := nsRaw.(map[string]any); ok {
					nodeSelector := make(map[string]string, len(nsMap))
					for k, v := range nsMap {
						if vs, ok := v.(string); ok {
							nodeSelector[k] = vs
						}
					}
					target.NodeSelector = nodeSelector
				}
			}
		} else {
			replicas := values.Lookup(u.Object, "spec", "replicas")
			if replicas == nil {
				target.Replicas = 1
			} else {
				replicasInt64, ok := toInt64(replicas)
				if !ok {
					return nil, log.CreateError(herrors.ErrFailedToParseReplicas,
						"failed to parse replicas '{replicas}' for {entity}",
						log.String("entity", string(id)), log.Any("replicas", replicas))
				}
				target.Replicas = replicasInt64
			}
		}

		targets = append(targets, target)
	}

	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}

		scaleGroup, found := merged[gvk]
		if !found {
			continue
		}

		u, err := item.UnstructuredOrError(key)
		if err != nil {
			continue
		}

		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		name, err := item.Name()
		if err != nil {
			return nil, err
		}
		ns, _ := item.Namespace()
		gvr, err := item.GVR()
		if err != nil {
			return nil, err
		}

		originalReplicas := make(map[string]int64, len(scaleGroup.ReplicaPaths))
		for _, path := range scaleGroup.ReplicaPaths {
			parts := strings.Split(path, ".")
			val := values.Lookup(u.Object, parts...)
			if val == nil {
				originalReplicas[path] = 0
			} else {
				intVal, ok := toInt64(val)
				if !ok {
					originalReplicas[path] = 0
				} else {
					originalReplicas[path] = intVal
				}
			}
		}

		targets = append(targets, ScaleTarget{
			Id:               id,
			Name:             name,
			Ns:               ns,
			GVR:              gvr,
			GVK:              gvk,
			IsCustomWorkload: true,
			ReplicaPaths:     scaleGroup.ReplicaPaths,
			OriginalReplicas: originalReplicas,
			StatusReadyPath:  scaleGroup.StatusReadyPath,
		})
	}

	return targets, nil
}

func resourceClient(dynamicClient dynamic.Interface, ns htypes.Namespace, gvr htypes.GVR) dynamic.ResourceInterface {
	rc := dynamicClient.Resource(gvr.K8s())
	if ns == "" {
		return rc
	}
	return rc.Namespace(string(ns))
}

func filterWorkloadEntities(entities entity.Entities, targets []ScaleTarget) (entity.Entities, error) {
	targetIds := make(map[htypes.Id]bool, len(targets))
	for _, t := range targets {
		targetIds[t.Id] = true
	}
	var items []entity.Entity
	for _, e := range entities.Items {
		id, err := e.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if targetIds[id] {
			items = append(items, e)
		}
	}
	return entity.NewEntities(items)
}

func logScaleUpPlan(l log.Logger, plan []PlanEntry) {
	if len(plan) == 0 {
		return
	}
	var sb strings.Builder
	sb.WriteString("scale-up order:")
	for i, entry := range plan {
		sb.WriteString(fmt.Sprintf("\n  %d. %s", i+1, entry.Name))
		if len(entry.Dependencies) == 0 {
			sb.WriteString(" (no dependencies)")
		} else {
			sb.WriteString(fmt.Sprintf(" (after: %s)", strings.Join(entry.Dependencies, ", ")))
		}
	}
	l.Info(logIdCommands, sb.String())
}

func logWorkloadList(l log.Logger, title string, ids []string) {
	var sb strings.Builder
	sb.WriteString(title)
	for _, id := range ids {
		sb.WriteString("\n  * ")
		sb.WriteString(id)
	}
	l.Info(logIdCommands, sb.String())
}

func planEntryNames(plan []PlanEntry) []string {
	result := make([]string, 0, len(plan))
	for _, entry := range plan {
		result = append(result, entry.Name)
	}
	return result
}

func pendingScaleDownTargets(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	targets []ScaleTarget,
) ([]ScaleTarget, error) {
	pending := make([]ScaleTarget, 0, len(targets))
	for _, target := range targets {
		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.DebugLog(logIdCommands, "{name} not found, skipping", log.String("name", name))
				continue
			}
			return nil, err
		}

		switch {
		case target.IsCustomWorkload:
			needsScale := false
			for _, path := range target.ReplicaPaths {
				if currentCustomReplicaValue(obj.Object, path) != 0 {
					needsScale = true
					break
				}
			}
			if !needsScale {
				l.DebugLog(logIdCommands, "custom workload {name} already scaled down, skipping", log.String("name", name))
				continue
			}
		case target.IsJob:
			suspended, _ := values.Lookup(obj.Object, "spec", "suspend").(bool)
			if suspended {
				l.DebugLog(logIdCommands, "job {name} already suspended, skipping", log.String("name", name))
				continue
			}
		case target.IsDaemonSet:
			currentNS := nodeSelectorFromObject(obj)
			if currentNS[hydra.AnnotationHydraScaleDisabled] == "true" {
				l.DebugLog(logIdCommands, "daemonset {name} already disabled, skipping", log.String("name", name))
				continue
			}
		default:
			currentReplicas := int64(1)
			if r := values.Lookup(obj.Object, "spec", "replicas"); r != nil {
				currentReplicas, _ = toInt64(r)
			}
			if currentReplicas == 0 {
				l.DebugLog(logIdCommands, "{name} is already scaled down with 0 replicas, skipping",
					log.String("name", name))
				continue
			}
		}

		pending = append(pending, target)
	}
	return pending, nil
}

func pendingScaleUpTargets(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	targets []ScaleTarget,
) ([]ScaleTarget, error) {
	pending := make([]ScaleTarget, 0, len(targets))
	for _, target := range targets {
		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)
		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.DebugLog(logIdCommands, "{name} not found, skipping", log.String("name", name))
				continue
			}
			return nil, err
		}

		switch {
		case target.IsCustomWorkload:
			needsScale := false
			for path, expected := range target.OriginalReplicas {
				if currentCustomReplicaValue(obj.Object, path) != expected {
					needsScale = true
					break
				}
			}
			if !needsScale {
				l.DebugLog(logIdCommands, "custom workload {name} is already at target replicas, skipping",
					log.String("name", name))
				continue
			}
		case target.IsJob:
			suspended, _ := values.Lookup(obj.Object, "spec", "suspend").(bool)
			if !suspended {
				l.DebugLog(logIdCommands, "job {name} is already unsuspended, skipping", log.String("name", name))
				continue
			}
		case target.IsDaemonSet:
			currentNS := nodeSelectorFromObject(obj)
			currentNSExists := nodeSelectorFieldExists(obj)
			targetNSExists := target.NodeSelector != nil
			if currentNSExists == targetNSExists && nodeSelectorEqual(currentNS, target.NodeSelector) {
				l.DebugLog(logIdCommands, "daemonset {name} already has target nodeSelector, skipping",
					log.String("name", name))
				continue
			}
		default:
			currentReplicas := int64(1)
			if r := values.Lookup(obj.Object, "spec", "replicas"); r != nil {
				currentReplicas, _ = toInt64(r)
			}
			if currentReplicas == target.Replicas {
				l.DebugLog(logIdCommands, "{name} is already up and running with {replicas} replicas",
					log.String("name", name), log.Int64("replicas", target.Replicas))
				continue
			}
		}

		pending = append(pending, target)
	}
	return pending, nil
}

func workloadIdSet(entities entity.Entities) (map[htypes.Id]bool, error) {
	result := make(map[htypes.Id]bool, entities.Len())
	for _, item := range entities.Items {
		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		result[id] = true
	}
	return result, nil
}

func filterEntitiesByIds(entities entity.Entities, ids map[htypes.Id]bool) (entity.Entities, error) {
	if len(ids) == 0 {
		return entity.NewEntities(nil)
	}

	items := make([]entity.Entity, 0, len(ids))
	for _, item := range entities.Items {
		id, err := item.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		if ids[id] {
			items = append(items, item)
		}
	}
	return entity.NewEntities(items)
}

func buildStartupExecutionPlan(workloadEntities entity.Entities, refs []htypes.Ref) (startupExecutionPlan, error) {
	workloadIds, err := workloadIdSet(workloadEntities)
	if err != nil {
		return startupExecutionPlan{}, err
	}

	type edgeKey struct {
		from htypes.Id
		to   htypes.Id
	}

	edgeOptionality := make(map[edgeKey]bool)
	requiredParticipants := make(map[htypes.Id]bool)
	optionalParticipants := make(map[htypes.Id]bool)

	for _, ref := range refs {
		from := ref.From
		to := ref.To
		if ref.Reverse {
			from, to = to, from
		}
		if from == to {
			continue
		}

		fromWorkload := workloadIds[from]
		toWorkload := workloadIds[to]
		if !fromWorkload && !toWorkload {
			continue
		}

		if ref.HasTag(htypes.RefTagOptionalStartup) {
			if fromWorkload {
				optionalParticipants[from] = true
			}
			if toWorkload {
				optionalParticipants[to] = true
			}
		}

		if !fromWorkload || !toWorkload {
			continue
		}

		optional := ref.HasTag(htypes.RefTagOptionalStartup)
		edge := edgeKey{from: from, to: to}
		existingOptional, exists := edgeOptionality[edge]
		if !exists || (existingOptional && !optional) {
			edgeOptionality[edge] = optional
		}
		if !optional {
			requiredParticipants[from] = true
			requiredParticipants[to] = true
		}
	}

	optionalOnlyIds := make(map[htypes.Id]bool)
	requiredIds := make(map[htypes.Id]bool, len(workloadIds))
	for id := range workloadIds {
		if optionalParticipants[id] && !requiredParticipants[id] {
			optionalOnlyIds[id] = true
			continue
		}
		requiredIds[id] = true
	}

	requiredEntities, err := filterEntitiesByIds(workloadEntities, requiredIds)
	if err != nil {
		return startupExecutionPlan{}, err
	}
	optionalEntities, err := filterEntitiesByIds(workloadEntities, optionalOnlyIds)
	if err != nil {
		return startupExecutionPlan{}, err
	}

	requiredRefs := make([]htypes.Ref, 0, len(edgeOptionality))
	optionalRefs := make([]htypes.Ref, 0, len(edgeOptionality))
	for edge, optional := range edgeOptionality {
		ref := htypes.Ref{
			RefType:      htypes.RefTypeIndirect,
			EndpointType: htypes.RefEndpointTypeId,
			From:         edge.from,
			To:           edge.to,
		}
		if optional {
			ref.Tags = []string{htypes.RefTagOptionalStartup}
			if optionalOnlyIds[edge.from] && optionalOnlyIds[edge.to] {
				optionalRefs = append(optionalRefs, ref)
			}
			continue
		}
		if requiredIds[edge.from] && requiredIds[edge.to] {
			requiredRefs = append(requiredRefs, ref)
		}
	}

	combined := make([]PlanEntry, 0, workloadEntities.Len())
	if requiredEntities.Len() > 0 {
		graph, err := BuildDependencyGraph(requiredEntities, requiredRefs)
		if err != nil {
			return startupExecutionPlan{}, err
		}
		combined = append(combined, PlanTopologicalOrder(graph)...)
	}
	if optionalEntities.Len() > 0 {
		graph, err := BuildDependencyGraph(optionalEntities, optionalRefs)
		if err != nil {
			return startupExecutionPlan{}, err
		}
		combined = append(combined, PlanTopologicalOrder(graph)...)
	}

	return startupExecutionPlan{
		combined:         combined,
		requiredEntities: requiredEntities,
		optionalEntities: optionalEntities,
		requiredRefs:     requiredRefs,
		optionalRefs:     optionalRefs,
	}, nil
}

func LogStartupOrder(l log.Logger, entities entity.Entities, refs []htypes.Ref, key htypes.EntityKeyUnstructured, customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup) ([]PlanEntry, error) {
	targets, err := CollectScaleTargets(entities, key, customWorkloads...)
	if err != nil {
		return nil, err
	}
	if len(targets) == 0 {
		return nil, nil
	}

	workloadEntities, err := filterWorkloadEntities(entities, targets)
	if err != nil {
		return nil, err
	}

	workloadIds := make(map[htypes.Id]bool, len(targets))
	for _, t := range targets {
		workloadIds[t.Id] = true
	}
	enrichedRefs := ResolveTransitiveWorkloadDeps(refs, workloadIds)
	startupPlan, err := buildStartupExecutionPlan(workloadEntities, enrichedRefs)
	if err != nil {
		return nil, err
	}
	plan := startupPlan.combined
	logScaleUpPlan(l, plan)
	return plan, nil
}

func setNestedInt64(obj map[string]any, value int64, path ...string) bool {
	current := obj
	for i := 0; i < len(path)-1; i++ {
		next, ok := current[path[i]].(map[string]any)
		if !ok {
			return false
		}
		current = next
	}
	current[path[len(path)-1]] = value
	return true
}

func setNestedInt64Patch(obj map[string]any, value int64, path ...string) {
	current := obj
	for i := 0; i < len(path)-1; i++ {
		if _, ok := current[path[i]]; !ok {
			current[path[i]] = make(map[string]any)
		}
		current = current[path[i]].(map[string]any)
	}
	current[path[len(path)-1]] = value
}

func ZeroWorkloads(entities entity.Entities, key htypes.EntityKeyUnstructured, customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup) (entity.Entities, error) {
	var merged map[htypes.GVKString]htypes.HydraScaleGroup
	if len(customWorkloads) > 0 && customWorkloads[0] != nil {
		merged = customWorkloads[0]
	}

	workloadGVKs := []htypes.GVKString{
		htypes.KubernetesGvkAppsV1Deployment,
		htypes.KubernetesGvkAppsV1ReplicaSet,
		htypes.KubernetesGvkAppsV1StatefulSet,
		htypes.KubernetesGvkAppsV1DaemonSet,
	}

	var items []entity.Entity
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}

		if gvk == htypes.KubernetesGvkBatchV1Job {
			u, err := item.UnstructuredOrError(key)
			if err != nil {
				items = append(items, item)
				continue
			}
			modified := *u.DeepCopy()
			if specMap, ok := modified.Object["spec"].(map[string]any); ok {
				specMap["suspend"] = true
			} else {
				modified.Object["spec"] = map[string]any{"suspend": true}
			}
			modifiedEntity, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
				return b.WithUnstructured(key, modified)
			})
			if modErr != nil {
				return entity.Entities{}, modErr
			}
			items = append(items, modifiedEntity)
			continue
		}

		if !slices.Contains(workloadGVKs, gvk) {
			if scaleGroup, found := merged[gvk]; found {
				u, uErr := item.UnstructuredOrError(key)
				if uErr != nil {
					items = append(items, item)
					continue
				}
				modified := *u.DeepCopy()
				for _, path := range scaleGroup.ReplicaPaths {
					parts := strings.Split(path, ".")
					setNestedInt64(modified.Object, 0, parts...)
				}
				modifiedEntity, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
					return b.WithUnstructured(key, modified)
				})
				if modErr != nil {
					return entity.Entities{}, modErr
				}
				items = append(items, modifiedEntity)
				continue
			}
			items = append(items, item)
			continue
		}

		u, err := item.UnstructuredOrError(key)
		if err != nil {
			items = append(items, item)
			continue
		}

		if gvk == htypes.KubernetesGvkAppsV1StatefulSet {
			if shouldSkipOperatorStatefulSet(entities, item, u, key, merged) {
				items = append(items, item)
				continue
			}
		}

		modified := *u.DeepCopy()

		if gvk == htypes.KubernetesGvkAppsV1DaemonSet {
			if specMap, ok := modified.Object["spec"].(map[string]any); ok {
				if templateMap, ok := specMap["template"].(map[string]any); ok {
					if templateSpecMap, ok := templateMap["spec"].(map[string]any); ok {
						templateSpecMap["nodeSelector"] = map[string]any{hydra.AnnotationHydraScaleDisabled: "true"}
					}
				}
			}
		} else {
			if specMap, ok := modified.Object["spec"].(map[string]any); ok {
				specMap["replicas"] = int64(0)
			}
		}

		modifiedEntity, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(key, modified)
		})
		if modErr != nil {
			return entity.Entities{}, modErr
		}
		items = append(items, modifiedEntity)
	}

	return entity.NewEntities(items)
}

func nodeSelectorFromObject(obj *unstructured.Unstructured) map[string]string {
	nsRaw := values.Lookup(obj.Object, "spec", "template", "spec", "nodeSelector")
	if nsRaw == nil {
		return nil
	}
	nsMap, ok := nsRaw.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(nsMap))
	for k, v := range nsMap {
		if vs, ok := v.(string); ok {
			result[k] = vs
		}
	}
	return result
}

func nodeSelectorFieldExists(obj *unstructured.Unstructured) bool {
	_, found, err := unstructured.NestedFieldNoCopy(obj.Object, "spec", "template", "spec", "nodeSelector")
	return err == nil && found
}

func nodeSelectorEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func jobConditionStatusTrue(obj *unstructured.Unstructured, condType string) bool {
	condsRaw := values.Lookup(obj.Object, "status", "conditions")
	conds, ok := condsRaw.([]any)
	if !ok {
		return false
	}
	for _, raw := range conds {
		cm, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t, _ := cm["type"].(string)
		s, _ := cm["status"].(string)
		if t == condType && s == "True" {
			return true
		}
	}
	return false
}

func jobHasFailed(obj *unstructured.Unstructured) bool {
	return jobConditionStatusTrue(obj, "Failed")
}

func jobHasCompleteConditionTrue(obj *unstructured.Unstructured) bool {
	return jobConditionStatusTrue(obj, "Complete")
}

func jobDesiredCompletions(obj *unstructured.Unstructured) int64 {
	c := values.Lookup(obj.Object, "spec", "completions")
	if c == nil {
		return 1
	}
	v, ok := toInt64(c)
	if !ok || v < 1 {
		return 1
	}
	return v
}

func jobReachedSuccessCompletion(obj *unstructured.Unstructured) bool {
	if jobHasFailed(obj) {
		return false
	}
	if jobHasCompleteConditionTrue(obj) {
		return true
	}
	succeeded, ok := toInt64(values.Lookup(obj.Object, "status", "succeeded"))
	if !ok {
		succeeded = 0
	}
	return succeeded >= jobDesiredCompletions(obj)
}

func workloadReadyAtTarget(obj *unstructured.Unstructured, target ScaleTarget) bool {
	if target.IsJob {
		return jobReachedSuccessCompletion(obj)
	}
	if target.IsDaemonSet {
		desired, ok := toInt64(values.Lookup(obj.Object, "status", "desiredNumberScheduled"))
		if !ok {
			return false
		}
		if desired == 0 {
			return true
		}
		ready, _ := toInt64(values.Lookup(obj.Object, "status", "numberReady"))
		return ready >= desired
	}

	specReplicas, ok := toInt64(values.Lookup(obj.Object, "spec", "replicas"))
	if !ok || specReplicas == 0 {
		return false
	}
	readyReplicas, _ := toInt64(values.Lookup(obj.Object, "status", "readyReplicas"))
	return readyReplicas >= specReplicas
}

func ScaleUpWorkloads(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	refsSync []htypes.Ref,
	fullRefs []htypes.Ref,
	key htypes.EntityKeyUnstructured,
	liveKey htypes.EntityKeyUnstructured,
	dryRun htypes.DryRun,
	scaleTimeout time.Duration,
	eval *ReadyEvaluator,
	customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup,
) error {
	if fullRefs == nil {
		fullRefs = refsSync
	}
	targets, err := CollectScaleTargets(entities, key, customWorkloads...)
	if err != nil {
		return err
	}

	targetMap := make(map[htypes.Id]ScaleTarget, len(targets))
	for _, t := range targets {
		targetMap[t.Id] = t
	}

	workloadEntities, err := filterWorkloadEntities(entities, targets)
	if err != nil {
		return err
	}

	workloadIds := make(map[htypes.Id]bool, len(targets))
	for _, t := range targets {
		workloadIds[t.Id] = true
	}
	enrichedRefs := ResolveTransitiveWorkloadDeps(refsSync, workloadIds)
	pendingTargets, err := pendingScaleUpTargets(ctx, l, dynamicClient, targets)
	if err != nil {
		return err
	}
	if len(pendingTargets) == 0 {
		l.Info(logIdCommands, "nothing to scale up")
		return nil
	}

	pendingIds := make(map[htypes.Id]bool, len(pendingTargets))
	for _, target := range pendingTargets {
		pendingIds[target.Id] = true
	}
	pendingEntities, err := filterEntitiesByIds(workloadEntities, pendingIds)
	if err != nil {
		return err
	}
	startupPlan, err := buildStartupExecutionPlan(pendingEntities, enrichedRefs)
	if err != nil {
		return err
	}

	l.Info(logIdCommands, "starting scale up")
	logWorkloadList(l, fmt.Sprintf("scaling %d resources:", pendingEntities.Len()), planEntryNames(startupPlan.combined))

	drp := ""
	if dryRun {
		drp = "[dry-run] "
	}

	start := func(ctx context.Context, e entity.Entity) error {
		id, err := e.Id()
		if err != nil {
			return err
		}
		target, ok := targetMap[id]
		if !ok {
			return fmt.Errorf("scale target not found for %s", id)
		}

		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)

		if target.IsCustomWorkload {
			l.DebugLog(logIdCommands, drp+"scaling up custom workload {name}", log.String("name", name))
			if dryRun {
				return nil
			}
			patchObj := make(map[string]any)
			for path, val := range target.OriginalReplicas {
				parts := strings.Split(path, ".")
				setNestedInt64Patch(patchObj, val, parts...)
			}
			patchData, mErr := json.Marshal(patchObj)
			if mErr != nil {
				return mErr
			}
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, patchData, metav1.PatchOptions{})
		} else if target.IsJob {
			l.DebugLog(logIdCommands, drp+"unsuspending job {name}", log.String("name", name))
			if dryRun {
				return nil
			}
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, []byte(`{"spec":{"suspend":false}}`), metav1.PatchOptions{})
		} else if target.IsDaemonSet {
			currentNSExists := false
			if !dryRun {
				obj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						l.Warn(logIdCommands, "{name} not found, skipping", log.String("name", name))
						return nil
					}
					return gErr
				}
				currentNS := nodeSelectorFromObject(obj)
				currentNSExists = nodeSelectorFieldExists(obj)
				targetNSExists := target.NodeSelector != nil
				if currentNSExists == targetNSExists && nodeSelectorEqual(currentNS, target.NodeSelector) {
					l.DebugLog(logIdCommands, drp+"restoring {name}: nodeSelector already at target (skipped)", log.String("name", name))
					return nil
				}
			}

			if target.NodeSelector == nil {
				l.DebugLog(logIdCommands, drp+"restoring {name}: removing nodeSelector", log.String("name", name))
				if dryRun {
					return nil
				}
				patch := `[{"op":"remove","path":"/spec/template/spec/nodeSelector"}]`
				_, err = client.Patch(ctx, name, ktypes.JSONPatchType, []byte(patch), metav1.PatchOptions{})
			} else {
				l.DebugLog(logIdCommands, drp+"restoring {name}: setting nodeSelector", log.String("name", name))
				if dryRun {
					return nil
				}
				patchOp := "add"
				if currentNSExists {
					patchOp = "replace"
				}
				patchData, mErr := json.Marshal([]map[string]any{
					{
						"op":    patchOp,
						"path":  "/spec/template/spec/nodeSelector",
						"value": target.NodeSelector,
					},
				})
				if mErr != nil {
					return mErr
				}
				_, err = client.Patch(ctx, name, ktypes.JSONPatchType, patchData, metav1.PatchOptions{})
			}
		} else {
			var currentReplicas int64 = -1
			if !dryRun {
				obj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						l.Warn(logIdCommands, "{name} not found, skipping", log.String("name", name))
						return nil
					}
					return gErr
				}
				if r := values.Lookup(obj.Object, "spec", "replicas"); r != nil {
					currentReplicas, _ = toInt64(r)
				}
				if currentReplicas == target.Replicas {
					l.DebugLog(logIdCommands, drp+"{name} is already up and running with {replicas} replicas",
						log.String("name", name), log.Int64("replicas", target.Replicas))
					return nil
				}
			}

			if currentReplicas >= 0 {
				l.DebugLog(logIdCommands, drp+"scaling up {name} to {replicas} replicas (current: {current})",
					log.String("name", name), log.Int64("replicas", target.Replicas), log.Int64("current", currentReplicas))
			} else {
				l.DebugLog(logIdCommands, drp+"scaling up {name} to {replicas} replicas",
					log.String("name", name), log.Int64("replicas", target.Replicas))
			}
			if dryRun {
				return nil
			}
			patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, target.Replicas)
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, []byte(patch), metav1.PatchOptions{})
		}

		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.Warn(logIdCommands, "{name} not found, skipping", log.String("name", name))
				return nil
			}
			return err
		}
		return nil
	}

	waitReady := func(ctx context.Context, e entity.Entity) error {
		if dryRun {
			return nil
		}

		id, err := e.Id()
		if err != nil {
			return err
		}
		target, ok := targetMap[id]
		if !ok {
			return fmt.Errorf("scale target not found for %s", id)
		}

		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)
		var loggedTransitiveWait bool
		shouldLogTransitiveWait := newScaleUpProgressLogGate(scaleUpProgressLogInterval)

		tryTransitiveAndReturn := func() error {
			pending, err := transitiveReadyPendingNames(ctx, dynamicClient, entities, fullRefs, eval, liveKey, id)
			if err != nil {
				return err
			}
			if len(pending) > 0 {
				loggedTransitiveWait = true
				if shouldLogTransitiveWait() {
					l.Info(logIdCommands,
						"{name}: local readiness reached, waiting for transitive ready gates ({count} pending): {pending}",
						log.String("name", name),
						log.Int("count", len(pending)),
						log.String("pending", strings.Join(pending, ", ")))
				}
				return errScaleUpWaitContinue
			}
			if loggedTransitiveWait {
				l.Info(logIdCommands, "{name}: transitive ready gates satisfied", log.String("name", name))
			}
			return nil
		}

		if target.IsCustomWorkload {
			if !scaleUpCustomWorkloadNeedsWait(target, e, eval, liveKey) {
				return nil
			}
			l.Info(logIdCommands, "waiting for custom workload {name} to become ready", log.String("name", name))
			ticker := time.NewTicker(scaleUpWaitPollInterval)
			defer ticker.Stop()
			deadline := time.After(scaleTimeout)
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-deadline:
					return log.CreateError(herrors.ErrScaleUpTimeout,
						"aborted: workload {name} did not become ready within {timeout}. To retry, run: hydra gitops scale <params> up",
						log.String("name", name), log.String("timeout", scaleTimeout.String()))
				case <-ticker.C:
					obj, err := client.Get(ctx, name, metav1.GetOptions{})
					if err != nil {
						if k8serrors.IsNotFound(err) {
							return nil
						}
						return err
					}
					selfOk := true
					if target.StatusReadyPath != "" {
						selfOk = selfOk && customWorkloadLiveReadyAtPath(obj, target.StatusReadyPath)
					}
					if eval != nil && eval.RuleMatched(e, liveKey) {
						matched, rdy, _, rerr := eval.ReadyFromLiveObject(e, obj.Object, liveKey)
						selfOk = selfOk && rerr == nil && matched && rdy
					}
					if !selfOk {
						continue
					}
					if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
						continue
					} else if err != nil {
						return err
					}
					return nil
				}
			}
		}

		if !target.IsDaemonSet && !target.IsJob && target.Replicas == 0 {
			return nil
		}

		// Job completed / replicas ready can be true while transitive global.hydra.ready gates are still pending.
		// Log that local success only once per waitReady; otherwise the poll loop would spam identical lines every tick.
		var loggedScaleUpSelfTerminal bool
		shouldLogProgress := newScaleUpProgressLogGate(scaleUpProgressLogInterval)

		obj, err := client.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return nil
			}
			return err
		}
		if workloadReadyAtTarget(obj, target) {
			if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
				// fall through to polling
			} else if err != nil {
				return err
			} else {
				return nil
			}
		}

		if target.IsJob {
			l.Info(logIdCommands, "waiting for job {name} to complete", log.String("name", name))
		} else {
			l.Info(logIdCommands, "waiting for {name} to become ready", log.String("name", name))
		}

		ticker := time.NewTicker(scaleUpWaitPollInterval)
		defer ticker.Stop()
		deadline := time.After(scaleTimeout)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline:
				return log.CreateError(herrors.ErrScaleUpTimeout,
					"aborted: workload {name} did not become ready within {timeout}. To retry, run: hydra gitops scale <params> up",
					log.String("name", name), log.String("timeout", scaleTimeout.String()))
			case <-ticker.C:
				obj, err := client.Get(ctx, name, metav1.GetOptions{})
				if err != nil {
					if k8serrors.IsNotFound(err) {
						return nil
					}
					return err
				}

				if target.IsJob {
					if jobHasFailed(obj) {
						return log.CreateError(herrors.ErrInternalError,
							"aborted: job {name} failed",
							log.String("name", name))
					}
					if jobReachedSuccessCompletion(obj) {
						if !loggedScaleUpSelfTerminal {
							loggedScaleUpSelfTerminal = true
							l.Info(logIdCommands, "job {name} completed", log.String("name", name))
						}
						if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
							continue
						} else if err != nil {
							return err
						}
						return nil
					}
					if shouldLogProgress() {
						l.Info(logIdCommands, "job {name} not complete yet", log.String("name", name))
					}
					continue
				}

				if target.IsDaemonSet {
					desiredInt, _ := toInt64(values.Lookup(obj.Object, "status", "desiredNumberScheduled"))
					readyInt, _ := toInt64(values.Lookup(obj.Object, "status", "numberReady"))
					if desiredInt == 0 {
						if !loggedScaleUpSelfTerminal {
							loggedScaleUpSelfTerminal = true
							l.Info(logIdCommands, "{name}: desiredNumberScheduled is 0, nothing to wait for",
								log.String("name", name))
						}
						if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
							continue
						} else if err != nil {
							return err
						}
						return nil
					}
					if desiredInt == readyInt {
						if !loggedScaleUpSelfTerminal {
							loggedScaleUpSelfTerminal = true
							l.Info(logIdCommands, "{name}: ready ({ready}/{desired})",
								log.String("name", name), log.Int64("ready", readyInt), log.Int64("desired", desiredInt))
						}
						if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
							continue
						} else if err != nil {
							return err
						}
						return nil
					}
					if shouldLogProgress() {
						l.Info(logIdCommands, "{name}: {ready}/{desired} ready",
							log.String("name", name), log.Int64("ready", readyInt), log.Int64("desired", desiredInt))
					}
				} else {
					specInt, _ := toInt64(values.Lookup(obj.Object, "spec", "replicas"))
					readyInt, _ := toInt64(values.Lookup(obj.Object, "status", "readyReplicas"))
					if specInt > 0 && readyInt >= specInt {
						if !loggedScaleUpSelfTerminal {
							loggedScaleUpSelfTerminal = true
							l.Info(logIdCommands, "{name}: ready ({ready}/{spec})",
								log.String("name", name), log.Int64("ready", readyInt), log.Int64("spec", specInt))
						}
						if err := tryTransitiveAndReturn(); err == errScaleUpWaitContinue {
							continue
						} else if err != nil {
							return err
						}
						return nil
					}
					if shouldLogProgress() {
						l.Info(logIdCommands, "{name}: {ready}/{spec} ready",
							log.String("name", name), log.Int64("ready", readyInt), log.Int64("spec", specInt))
					}
				}
			}
		}
	}

	if startupPlan.requiredEntities.Len() > 0 {
		if err := TopologicalExecute(ctx, l, startupPlan.requiredEntities, startupPlan.requiredRefs, start, waitReady); err != nil {
			return err
		}
	}
	if startupPlan.optionalEntities.Len() > 0 {
		if err := TopologicalExecute(ctx, l, startupPlan.optionalEntities, startupPlan.optionalRefs, start, waitReady); err != nil {
			return err
		}
	}
	l.Info(logIdCommands, "scale up finished")
	return nil
}

// ScaleDownWorkloads is the symmetric counterpart of [ScaleUpWorkloads]: refsSync (template)
// drives [ResolveTransitiveWorkloadDeps] / topological ordering, fullRefs (merged inspect graph)
// is reserved for transitive reachability checks. Pass nil for fullRefs to fall back to refsSync
// (matches the previous single-list signature).
func ScaleDownWorkloads(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	entities entity.Entities,
	refsSync []htypes.Ref,
	fullRefs []htypes.Ref,
	key htypes.EntityKeyUnstructured,
	dryRun htypes.DryRun,
	forceScaleDown htypes.ForceScaleDown,
	scaleTimeout time.Duration,
	customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup,
) error {
	if fullRefs == nil {
		fullRefs = refsSync
	}
	_ = fullRefs // reserved for transitive readiness checks; mirrored from ScaleUpWorkloads
	targets, err := CollectScaleTargets(entities, key, customWorkloads...)
	if err != nil {
		return err
	}

	targetMap := make(map[htypes.Id]ScaleTarget, len(targets))
	for _, t := range targets {
		targetMap[t.Id] = t
	}

	workloadEntities, err := filterWorkloadEntities(entities, targets)
	if err != nil {
		return err
	}

	workloadIds := make(map[htypes.Id]bool, len(targets))
	for _, t := range targets {
		workloadIds[t.Id] = true
	}
	enrichedRefs := ResolveTransitiveWorkloadDeps(refsSync, workloadIds)

	pendingTargets, err := pendingScaleDownTargets(ctx, l, dynamicClient, targets)
	if err != nil {
		return err
	}
	if len(pendingTargets) == 0 {
		l.Info(logIdCommands, "nothing to scale down")
		return nil
	}
	pendingIds := make(map[htypes.Id]bool, len(pendingTargets))
	for _, target := range pendingTargets {
		pendingIds[target.Id] = true
	}
	pendingEntities, err := filterEntitiesByIds(workloadEntities, pendingIds)
	if err != nil {
		return err
	}
	graph, err := BuildDependencyGraph(pendingEntities, ReverseRefs(enrichedRefs))
	if err != nil {
		return err
	}
	l.Info(logIdCommands, "starting scale down")
	logWorkloadList(l, fmt.Sprintf("scaling %d resources:", pendingEntities.Len()), planEntryNames(PlanTopologicalOrder(graph)))

	drp := ""
	if dryRun {
		drp = "[dry-run] "
	}

	scaleDown := func(ctx context.Context, e entity.Entity) error {
		id, err := e.Id()
		if err != nil {
			return err
		}
		target, ok := targetMap[id]
		if !ok {
			return fmt.Errorf("scale target not found for %s", id)
		}

		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)

		if target.IsCustomWorkload {
			l.DebugLog(logIdCommands, drp+"scaling down custom workload {name}", log.String("name", name))
			if dryRun {
				return nil
			}
			patchObj := make(map[string]any)
			for _, path := range target.ReplicaPaths {
				parts := strings.Split(path, ".")
				setNestedInt64Patch(patchObj, 0, parts...)
			}
			patchData, mErr := json.Marshal(patchObj)
			if mErr != nil {
				return mErr
			}
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, patchData, metav1.PatchOptions{})
		} else if target.IsJob {
			l.DebugLog(logIdCommands, drp+"suspending job {name}", log.String("name", name))
			if dryRun {
				return nil
			}
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, []byte(`{"spec":{"suspend":true}}`), metav1.PatchOptions{})
		} else if target.IsDaemonSet {
			if !dryRun {
				obj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						l.DebugLog(logIdCommands, "{name} not found, skipping", log.String("name", name))
						return nil
					}
					return gErr
				}
				currentNS := nodeSelectorFromObject(obj)
				if currentNS[hydra.AnnotationHydraScaleDisabled] == "true" {
					l.DebugLog(logIdCommands, drp+"daemonset {name} already disabled, skipping",
						log.String("name", name))
					return nil
				}
			}

			l.DebugLog(logIdCommands, drp+"disabling daemonset {name}", log.String("name", name))
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
			_, err = client.Patch(ctx, name, ktypes.MergePatchType, patchData, metav1.PatchOptions{})
		} else {
			if !dryRun {
				obj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						l.DebugLog(logIdCommands, "{name} not found, skipping", log.String("name", name))
						return nil
					}
					return gErr
				}

				currentReplicas := int64(1)
				if r := values.Lookup(obj.Object, "spec", "replicas"); r != nil {
					currentReplicas, _ = toInt64(r)
				}
				if currentReplicas == 0 {
					l.DebugLog(logIdCommands, drp+"{name} is already scaled down with 0 replicas, skipping",
						log.String("name", name))
					return nil
				}
			}

			l.DebugLog(logIdCommands, drp+"scaling down {name} to 0 replicas", log.String("name", name))
			if dryRun {
				return nil
			}
			_, err = client.Patch(ctx, name, ktypes.MergePatchType,
				[]byte(`{"spec":{"replicas":0}}`), metav1.PatchOptions{})
		}

		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.DebugLog(logIdCommands, "{name} not found, skipping", log.String("name", name))
				return nil
			}
			return err
		}
		return nil
	}

	waitScaledDown := func(ctx context.Context, e entity.Entity) error {
		if dryRun {
			return nil
		}

		id, err := e.Id()
		if err != nil {
			return err
		}
		target, ok := targetMap[id]
		if !ok {
			return fmt.Errorf("scale target not found for %s", id)
		}

		if target.IsCustomWorkload {
			return nil
		}

		client := resourceClient(dynamicClient, target.Ns, target.GVR)
		name := string(target.Name)

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		deadline := time.After(scaleTimeout)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-deadline:
				if !forceScaleDown {
					return log.CreateError(herrors.ErrScaleDownTimeout,
						"aborted: pods did not terminate within {timeout}. To retry, run the same command again. To re-scale workloads, run: hydra gitops scale <params> up",
						log.String("timeout", scaleTimeout.String()))
				}

				l.Warn(logIdCommands, "force-deleting pods for {name}", log.String("name", name))
				workloadObj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						return nil
					}
					return gErr
				}
				workloadUID := workloadObj.GetUID()

				podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
				podClient := dynamicClient.Resource(podGVR).Namespace(string(target.Ns))
				podList, lErr := podClient.List(ctx, metav1.ListOptions{})
				if lErr != nil {
					return lErr
				}

				gracePeriod := int64(0)
				for _, pod := range podList.Items {
					for _, ownerRef := range pod.GetOwnerReferences() {
						if ownerRef.UID == workloadUID {
							l.Info(logIdCommands, "force-deleting pod {pod} in namespace {ns}",
								log.String("pod", pod.GetName()), log.String("ns", string(target.Ns)))
							dErr := podClient.Delete(ctx, pod.GetName(), metav1.DeleteOptions{
								GracePeriodSeconds: &gracePeriod,
							})
							if dErr != nil {
								if k8serrors.IsNotFound(dErr) {
									l.DebugLog(logIdCommands, "pod {pod} in namespace {ns} not found during workload force-delete (already gone)",
										log.String("pod", pod.GetName()), log.String("ns", string(target.Ns)))
								} else {
									l.Warn(logIdCommands, "failed to force-delete pod {pod}: {err}",
										log.String("pod", pod.GetName()), log.Err(dErr))
								}
							}
							break
						}
					}
				}
				return nil

			case <-ticker.C:
				obj, gErr := client.Get(ctx, name, metav1.GetOptions{})
				if gErr != nil {
					if k8serrors.IsNotFound(gErr) {
						return nil
					}
					return gErr
				}

				if target.IsJob {
					activeInt, _ := toInt64(values.Lookup(obj.Object, "status", "active"))
					if activeInt == 0 {
						l.DebugLog(logIdCommands, "{name} scaled down", log.String("name", name))
						return nil
					}
					l.DebugLog(logIdCommands, "{name}: {active} active pods",
						log.String("name", name), log.Int64("active", activeInt))
				} else if target.IsDaemonSet {
					scheduled := values.Lookup(obj.Object, "status", "currentNumberScheduled")
					scheduledInt, _ := scheduled.(int64)
					if scheduledInt == 0 {
						l.DebugLog(logIdCommands, "{name} scaled down", log.String("name", name))
						return nil
					}
					l.DebugLog(logIdCommands, "{name}: {scheduled} pods still scheduled",
						log.String("name", name), log.Int64("scheduled", scheduledInt))
				} else {
					statusReplicas := values.Lookup(obj.Object, "status", "replicas")
					statusInt, _ := statusReplicas.(int64)
					if statusInt == 0 {
						l.DebugLog(logIdCommands, "{name} scaled down", log.String("name", name))
						return nil
					}
					l.DebugLog(logIdCommands, "{name}: {replicas} replicas still running",
						log.String("name", name), log.Int64("replicas", statusInt))
				}
			}
		}
	}

	if err := TopologicalExecute(ctx, l, pendingEntities, ReverseRefs(enrichedRefs), scaleDown, waitScaledDown); err != nil {
		return err
	}
	l.Info(logIdCommands, "scale down finished")
	return nil
}
