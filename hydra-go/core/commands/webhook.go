package commands

import (
	"fmt"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
)

type WebhookProvider struct {
	WebhookConfig entity.Entity
	ServiceName   types.Name
	ServiceNs     types.Namespace
	Workload      entity.Entity
}

type serviceKey struct {
	name      types.Name
	namespace types.Namespace
}

func ResolveWebhookProviders(
	l log.Logger,
	webhookEntities entity.Entities,
	allEntities entity.Entities,
	key types.EntityKeyUnstructured,
) ([]WebhookProvider, error) {
	var result []WebhookProvider
	seenWorkloads := map[types.Id]bool{}

	for _, whEntity := range webhookEntities.Items {
		u, err := whEntity.UnstructuredOrError(key)
		if err != nil {
			continue
		}

		whName, _ := whEntity.Name()

		webhooksRaw := values.Lookup(u.Object, "webhooks")
		if webhooksRaw == nil {
			continue
		}
		webhooksList, ok := webhooksRaw.([]any)
		if !ok {
			continue
		}

		seenServices := map[serviceKey]bool{}

		for _, whRaw := range webhooksList {
			wh, ok := whRaw.(map[string]any)
			if !ok {
				continue
			}

			svcNameRaw := values.Lookup(wh, "clientConfig", "service", "name")
			svcNsRaw := values.Lookup(wh, "clientConfig", "service", "namespace")
			if svcNameRaw == nil || svcNsRaw == nil {
				continue
			}

			svcName, ok := svcNameRaw.(string)
			if !ok {
				continue
			}
			svcNs, ok := svcNsRaw.(string)
			if !ok {
				continue
			}

			sk := serviceKey{name: types.Name(svcName), namespace: types.Namespace(svcNs)}
			if seenServices[sk] {
				continue
			}
			seenServices[sk] = true

			svcEntity := findServiceEntity(allEntities, sk)
			if svcEntity == nil {
				l.Warn(logIdCommands, "webhook {webhook}: service {service} in namespace {namespace} not found",
					log.String("webhook", string(whName)),
					log.String("service", svcName),
					log.String("namespace", svcNs))
				continue
			}

			svcU, err := svcEntity.UnstructuredOrError(key)
			if err != nil {
				continue
			}

			selectorRaw := values.Lookup(svcU.Object, "spec", "selector")
			if selectorRaw == nil {
				l.Warn(logIdCommands, "webhook {webhook}: service {service} has no selector",
					log.String("webhook", string(whName)),
					log.String("service", svcName))
				continue
			}
			selectorMap, ok := selectorRaw.(map[string]any)
			if !ok || len(selectorMap) == 0 {
				l.Warn(logIdCommands, "webhook {webhook}: service {service} has no selector",
					log.String("webhook", string(whName)),
					log.String("service", svcName))
				continue
			}

			selector := make(map[string]string, len(selectorMap))
			for k, v := range selectorMap {
				if vs, ok := v.(string); ok {
					selector[k] = vs
				}
			}

			workload := findWorkloadEntity(allEntities, types.Namespace(svcNs), selector, key)
			if workload == nil {
				l.Warn(logIdCommands, "webhook {webhook}: no workload found for service {service} in namespace {namespace}",
					log.String("webhook", string(whName)),
					log.String("service", svcName),
					log.String("namespace", svcNs))
				continue
			}

			workloadId, err := workload.Id()
			if err != nil {
				return nil, err
			}
			if seenWorkloads[workloadId] {
				continue
			}
			seenWorkloads[workloadId] = true

			result = append(result, WebhookProvider{
				WebhookConfig: whEntity,
				ServiceName:   types.Name(svcName),
				ServiceNs:     types.Namespace(svcNs),
				Workload:      *workload,
			})
		}
	}

	return result, nil
}

// SetWebhookFailurePolicy rewrites .webhooks[].failurePolicy for admission webhook
// configuration entities and preserves non-webhook entities unchanged.
func SetWebhookFailurePolicy(
	webhookEntities entity.Entities,
	key types.EntityKeyUnstructured,
	failurePolicy string,
) (entity.Entities, error) {
	var out []entity.Entity
	for _, item := range webhookEntities.Items {
		u, err := item.UnstructuredOrError(key)
		if err != nil {
			out = append(out, item)
			continue
		}

		modified := *u.DeepCopy()
		webhooksRaw := values.Lookup(modified.Object, "webhooks")
		webhooksList, ok := webhooksRaw.([]any)
		if !ok || len(webhooksList) == 0 {
			out = append(out, item)
			continue
		}

		for _, whRaw := range webhooksList {
			wh, ok := whRaw.(map[string]any)
			if !ok {
				continue
			}
			wh["failurePolicy"] = failurePolicy
		}

		modifiedItem, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(key, modified)
		})
		if modErr != nil {
			return entity.Entities{}, modErr
		}
		out = append(out, modifiedItem)
	}
	return entity.NewEntities(out)
}

// PlanWebhookApplyOrder orders webhook configurations by the startup order of
// their backing provider workloads. Webhooks without a resolved backing
// workload keep their original relative order and are appended after the
// provider-backed set.
func PlanWebhookApplyOrder(
	l log.Logger,
	webhookEntities entity.Entities,
	allEntities entity.Entities,
	refsSync []types.Ref,
	key types.EntityKeyUnstructured,
) (entity.Entities, error) {
	if webhookEntities.Len() == 0 {
		return webhookEntities, nil
	}

	workloadsByID := make(map[types.Id]entity.Entity)
	webhooksByWorkloadID := make(map[types.Id][]entity.Entity)
	var fallback []entity.Entity

	for _, whEntity := range webhookEntities.Items {
		singleEntities, err := entity.NewEntities([]entity.Entity{whEntity})
		if err != nil {
			return entity.Entities{}, err
		}

		providers, err := ResolveWebhookProviders(l, singleEntities, allEntities, key)
		if err != nil {
			return entity.Entities{}, err
		}
		if len(providers) == 0 {
			fallback = append(fallback, whEntity)
			continue
		}

		seenProviders := map[types.Id]bool{}
		for _, provider := range providers {
			workloadID, err := provider.Workload.Id()
			if err != nil {
				return entity.Entities{}, err
			}
			if seenProviders[workloadID] {
				continue
			}
			seenProviders[workloadID] = true
			workloadsByID[workloadID] = provider.Workload
			webhooksByWorkloadID[workloadID] = append(webhooksByWorkloadID[workloadID], whEntity)
		}
	}

	if len(workloadsByID) == 0 {
		return webhookEntities, nil
	}

	workloadItems := make([]entity.Entity, 0, len(workloadsByID))
	workloadIDs := make(map[types.Id]bool, len(workloadsByID))
	for id, workload := range workloadsByID {
		workloadItems = append(workloadItems, workload)
		workloadIDs[id] = true
	}
	workloadEntities, err := entity.NewEntities(workloadItems)
	if err != nil {
		return entity.Entities{}, err
	}

	enrichedRefs := ResolveTransitiveWorkloadDeps(refsSync, workloadIDs)
	startupPlan, err := buildStartupExecutionPlan(workloadEntities, enrichedRefs)
	if err != nil {
		return entity.Entities{}, err
	}

	ordered := make([]entity.Entity, 0, webhookEntities.Len())
	seenWebhookIDs := map[types.Id]bool{}
	appendWebhook := func(wh entity.Entity) error {
		whID, err := wh.Id()
		if err != nil {
			return err
		}
		if seenWebhookIDs[whID] {
			return nil
		}
		seenWebhookIDs[whID] = true
		ordered = append(ordered, wh)
		return nil
	}

	for _, entry := range startupPlan.combined {
		workloadID := types.Id(entry.Name)
		for _, whEntity := range webhooksByWorkloadID[workloadID] {
			if err := appendWebhook(whEntity); err != nil {
				return entity.Entities{}, err
			}
		}
	}

	for _, whEntity := range fallback {
		if err := appendWebhook(whEntity); err != nil {
			return entity.Entities{}, err
		}
	}

	for _, whEntity := range webhookEntities.Items {
		if err := appendWebhook(whEntity); err != nil {
			return entity.Entities{}, err
		}
	}

	return entity.NewEntities(ordered)
}

func findServiceEntity(allEntities entity.Entities, sk serviceKey) *entity.Entity {
	for i, item := range allEntities.Items {
		gvk, err := item.GVKString()
		if err != nil || gvk != types.KubernetesGvkV1Service {
			continue
		}
		name, err := item.Name()
		if err != nil || name != sk.name {
			continue
		}
		ns, _ := item.Namespace()
		if ns != sk.namespace {
			continue
		}
		return &allEntities.Items[i]
	}
	return nil
}

var workloadGVKPriority = map[types.GVKString]int{
	types.KubernetesGvkAppsV1Deployment:  0,
	types.KubernetesGvkAppsV1StatefulSet: 1,
	types.KubernetesGvkAppsV1DaemonSet:   2,
}

func findWorkloadEntity(
	allEntities entity.Entities,
	namespace types.Namespace,
	selector map[string]string,
	key types.EntityKeyUnstructured,
) *entity.Entity {
	workloadGVKs := []types.GVKString{
		types.KubernetesGvkAppsV1Deployment,
		types.KubernetesGvkAppsV1StatefulSet,
		types.KubernetesGvkAppsV1DaemonSet,
	}

	var matches []int
	for i, item := range allEntities.Items {
		gvk, err := item.GVKString()
		if err != nil || !slices.Contains(workloadGVKs, gvk) {
			continue
		}
		ns, _ := item.Namespace()
		if ns != namespace {
			continue
		}

		u, err := item.UnstructuredOrError(key)
		if err != nil {
			continue
		}

		labelsRaw := values.Lookup(u.Object, "spec", "template", "metadata", "labels")
		if labelsRaw == nil {
			continue
		}
		labelsMap, ok := labelsRaw.(map[string]any)
		if !ok {
			continue
		}

		if matchesSelector(labelsMap, selector) {
			matches = append(matches, i)
		}
	}

	if len(matches) == 0 {
		return nil
	}

	if len(matches) > 1 {
		slices.SortFunc(matches, func(a, b int) int {
			gvkA, _ := allEntities.Items[a].GVKString()
			gvkB, _ := allEntities.Items[b].GVKString()
			prioA := workloadGVKPriority[gvkA]
			prioB := workloadGVKPriority[gvkB]
			if prioA != prioB {
				return prioA - prioB
			}
			nameA, _ := allEntities.Items[a].Name()
			nameB, _ := allEntities.Items[b].Name()
			if nameA < nameB {
				return -1
			}
			if nameA > nameB {
				return 1
			}
			return 0
		})

		nameFirst, _ := allEntities.Items[matches[0]].Name()
		var names []string
		for _, idx := range matches {
			n, _ := allEntities.Items[idx].Name()
			names = append(names, string(n))
		}
		_ = nameFirst
		l := log.Default()
		l.Warn(logIdCommands, "multiple workloads match service selector, using {name}: {all}",
			log.String("name", string(nameFirst)),
			log.String("all", fmt.Sprintf("%v", names)))
	}

	return &allEntities.Items[matches[0]]
}

func matchesSelector(labels map[string]any, selector map[string]string) bool {
	for k, v := range selector {
		labelVal, ok := labels[k]
		if !ok {
			return false
		}
		labelStr, ok := labelVal.(string)
		if !ok || labelStr != v {
			return false
		}
	}
	return true
}

type WebhookRule struct {
	ApiGroups  []string
	Resources  []string
	Operations []string
}

// ExtractWebhookRules parses .webhooks[].rules[] from a webhook configuration entity.
func ExtractWebhookRules(e entity.Entity, key types.EntityKeyUnstructured) []WebhookRule {
	u, err := e.UnstructuredOrError(key)
	if err != nil {
		return nil
	}

	webhooksRaw := values.Lookup(u.Object, "webhooks")
	if webhooksRaw == nil {
		return nil
	}
	webhooksList, ok := webhooksRaw.([]any)
	if !ok {
		return nil
	}

	var result []WebhookRule
	for _, whRaw := range webhooksList {
		wh, ok := whRaw.(map[string]any)
		if !ok {
			continue
		}
		rulesRaw, ok := wh["rules"]
		if !ok {
			continue
		}
		rulesSlice, ok := rulesRaw.([]any)
		if !ok {
			continue
		}
		for _, ruleRaw := range rulesSlice {
			rule, ok := ruleRaw.(map[string]any)
			if !ok {
				continue
			}
			result = append(result, WebhookRule{
				ApiGroups:  toStringSlice(rule["apiGroups"]),
				Resources:  toStringSlice(rule["resources"]),
				Operations: toStringSlice(rule["operations"]),
			})
		}
	}
	return result
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// WebhookMatchesEntities checks whether any webhook rule intercepts any of the given entities.
// Only rules with CREATE or UPDATE operations are considered relevant (SSA triggers these).
func WebhookMatchesEntities(rules []WebhookRule, entities entity.Entities) (bool, error) {
	for _, rule := range rules {
		if !ruleHasRelevantOperation(rule) {
			continue
		}
		for _, e := range entities.Items {
			group, err := e.Group()
			if err != nil {
				return false, err
			}
			resource, err := e.Resource()
			if err != nil {
				continue
			}
			if ruleMatchesGroupResource(rule, string(group), string(resource)) {
				return true, nil
			}
		}
	}
	return false, nil
}

func ruleHasRelevantOperation(rule WebhookRule) bool {
	if len(rule.Operations) == 0 {
		return true
	}
	for _, op := range rule.Operations {
		if op == "CREATE" || op == "UPDATE" || op == "*" {
			return true
		}
	}
	return false
}

func ruleMatchesGroupResource(rule WebhookRule, group, resource string) bool {
	return matchesAny(rule.ApiGroups, group) && matchesAnyResource(rule.Resources, resource)
}

func matchesAny(patterns []string, value string) bool {
	for _, p := range patterns {
		if p == "*" || p == value {
			return true
		}
	}
	return false
}

func matchesAnyResource(patterns []string, value string) bool {
	for _, p := range patterns {
		if p == "*" || p == "*/*" || p == value {
			return true
		}
	}
	return false
}

// FilterWebhooksToDisable determines which webhook configurations should be disabled.
// Webhooks whose rules intercept applied resources AND whose backing workload is not ready
// are returned in toDisable. All others are returned in toKeep.
func FilterWebhooksToDisable(
	l log.Logger,
	webhookEntities entity.Entities,
	nonWebhookEntities entity.Entities,
	allEntities entity.Entities,
	key types.EntityKeyUnstructured,
	isProviderReady func(provider WebhookProvider) (bool, error),
) (toDisable entity.Entities, toKeep entity.Entities, err error) {
	var disableItems []entity.Entity
	var keepItems []entity.Entity

	for _, whEntity := range webhookEntities.Items {
		whId, _ := whEntity.Id()

		rules := ExtractWebhookRules(whEntity, key)
		if len(rules) == 0 {
			keepItems = append(keepItems, whEntity)
			continue
		}

		matches, err := WebhookMatchesEntities(rules, nonWebhookEntities)
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if !matches {
			l.DebugLog(logIdCommands, "webhook {id}: rules do not match any applied resources, skipping",
				log.String("id", string(whId)))
			keepItems = append(keepItems, whEntity)
			continue
		}

		singleEntities, singleErr := entity.NewEntities([]entity.Entity{whEntity})
		if singleErr != nil {
			return entity.Entities{}, entity.Entities{}, singleErr
		}

		providers, provErr := ResolveWebhookProviders(l, singleEntities, allEntities, key)
		if provErr != nil {
			return entity.Entities{}, entity.Entities{}, provErr
		}

		if len(providers) == 0 {
			if webhookHasURLConfig(whEntity, key) {
				l.DebugLog(logIdCommands, "webhook {id}: URL-based webhook, treating as ready",
					log.String("id", string(whId)))
				keepItems = append(keepItems, whEntity)
			} else {
				l.Info(logIdCommands, "webhook {id}: no backing workload found, will disable",
					log.String("id", string(whId)))
				disableItems = append(disableItems, whEntity)
			}
			continue
		}

		allReady := true
		for _, p := range providers {
			ready, readyErr := isProviderReady(p)
			if readyErr != nil {
				return entity.Entities{}, entity.Entities{}, readyErr
			}
			if !ready {
				allReady = false
				break
			}
		}

		if allReady {
			l.DebugLog(logIdCommands, "webhook {id}: provider is ready, keeping",
				log.String("id", string(whId)))
			keepItems = append(keepItems, whEntity)
		} else {
			l.Info(logIdCommands, "webhook {id}: provider is not ready, will disable",
				log.String("id", string(whId)))
			disableItems = append(disableItems, whEntity)
		}
	}

	toDisable, err = entity.NewEntities(disableItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	toKeep, err = entity.NewEntities(keepItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return toDisable, toKeep, nil
}

func webhookHasURLConfig(e entity.Entity, key types.EntityKeyUnstructured) bool {
	u, err := e.UnstructuredOrError(key)
	if err != nil {
		return false
	}
	webhooksRaw := values.Lookup(u.Object, "webhooks")
	if webhooksRaw == nil {
		return false
	}
	webhooksList, ok := webhooksRaw.([]any)
	if !ok {
		return false
	}
	for _, whRaw := range webhooksList {
		wh, ok := whRaw.(map[string]any)
		if !ok {
			continue
		}
		if values.Lookup(wh, "clientConfig", "url") != nil {
			return true
		}
	}
	return false
}
