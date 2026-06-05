package k8s

import (
	"context"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

func DeleteWebhookConfigs(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	webhookEntities entity.Entities,
	key types.EntityKeyUnstructured,
	dryRun types.DryRun,
) error {
	for _, e := range webhookEntities.Items {
		gvkStr, err := e.GVKString()
		if err != nil {
			return err
		}

		var gvr schema.GroupVersionResource
		switch gvkStr {
		case types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration:
			gvr = schema.GroupVersionResource{
				Group:    "admissionregistration.k8s.io",
				Version:  "v1",
				Resource: "validatingwebhookconfigurations",
			}
		case types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration:
			gvr = schema.GroupVersionResource{
				Group:    "admissionregistration.k8s.io",
				Version:  "v1",
				Resource: "mutatingwebhookconfigurations",
			}
		default:
			continue
		}

		name, err := e.Name()
		if err != nil {
			return err
		}

		if dryRun {
			l.Info(logIdK8s, "would delete {kind} {name}",
				log.String("kind", string(gvkStr)),
				log.String("name", string(name)))
			continue
		}

		err = dynamicClient.Resource(gvr).Delete(ctx, string(name), metav1.DeleteOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.Info(logIdK8s, "{kind} {name} not found, skipping",
					log.String("kind", string(gvkStr)),
					log.String("name", string(name)))
				continue
			}
			return err
		}

		l.Info(logIdK8s, "deleted {kind} {name}",
			log.String("kind", string(gvkStr)),
			log.String("name", string(name)))
	}

	return nil
}

func webhookGVR(gvkStr types.GVKString) (schema.GroupVersionResource, bool) {
	switch gvkStr {
	case types.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration:
		return schema.GroupVersionResource{
			Group:    "admissionregistration.k8s.io",
			Version:  "v1",
			Resource: "validatingwebhookconfigurations",
		}, true
	case types.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration:
		return schema.GroupVersionResource{
			Group:    "admissionregistration.k8s.io",
			Version:  "v1",
			Resource: "mutatingwebhookconfigurations",
		}, true
	default:
		return schema.GroupVersionResource{}, false
	}
}

// DisableWebhookConfigs patches webhook configurations by setting failurePolicy
// to "Ignore" for all webhook entries. This makes the webhooks non-blocking
// when their backing service is not reachable. The subsequent webhook apply
// phase restores the correct failurePolicy via SSA.
func DisableWebhookConfigs(
	ctx context.Context,
	l log.Logger,
	dynamicClient dynamic.Interface,
	webhookEntities entity.Entities,
	key types.EntityKeyUnstructured,
	dryRun types.DryRun,
) error {
	for _, e := range webhookEntities.Items {
		gvkStr, err := e.GVKString()
		if err != nil {
			return err
		}

		gvr, ok := webhookGVR(gvkStr)
		if !ok {
			continue
		}

		name, err := e.Name()
		if err != nil {
			return err
		}

		if dryRun {
			l.Info(logIdK8s, "would disable {kind} {name} (set failurePolicy=Ignore)",
				log.String("kind", string(gvkStr)),
				log.String("name", string(name)))
			continue
		}

		existing, err := dynamicClient.Resource(gvr).Get(ctx, string(name), metav1.GetOptions{})
		if err != nil {
			if k8serrors.IsNotFound(err) {
				l.Info(logIdK8s, "{kind} {name} not found in cluster, skipping disable",
					log.String("kind", string(gvkStr)),
					log.String("name", string(name)))
				continue
			}
			return err
		}

		webhooksRaw := values.Lookup(existing.Object, "webhooks")
		if webhooksRaw == nil {
			continue
		}
		webhooksList, ok := webhooksRaw.([]any)
		if !ok || len(webhooksList) == 0 {
			continue
		}

		allIgnore := true
		for _, whRaw := range webhooksList {
			wh, ok := whRaw.(map[string]any)
			if !ok {
				continue
			}
			fp, _ := wh["failurePolicy"].(string)
			if fp != "Ignore" {
				allIgnore = false
				wh["failurePolicy"] = "Ignore"
			}
		}

		if allIgnore {
			l.DebugLog(logIdK8s, "{kind} {name} already has failurePolicy=Ignore on all entries, skipping",
				log.String("kind", string(gvkStr)),
				log.String("name", string(name)))
			continue
		}

		_, err = dynamicClient.Resource(gvr).Update(ctx, existing, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		l.Info(logIdK8s, "disabled {kind} {name} (set failurePolicy=Ignore)",
			log.String("kind", string(gvkStr)),
			log.String("name", string(name)))
	}

	return nil
}

// IsWorkloadReady checks whether the given workload entity has at least one
// ready replica in the cluster (readyReplicas != 0 for Deployment/StatefulSet,
// numberReady != 0 for DaemonSet).
func IsWorkloadReady(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	workload entity.Entity,
) (bool, error) {
	gvr, err := workload.GVR()
	if err != nil {
		return false, err
	}
	name, err := workload.Name()
	if err != nil {
		return false, err
	}
	ns, _ := workload.Namespace()

	var client dynamic.ResourceInterface
	if ns == "" {
		client = dynamicClient.Resource(gvr.K8s())
	} else {
		client = dynamicClient.Resource(gvr.K8s()).Namespace(string(ns))
	}

	obj, err := client.Get(ctx, string(name), metav1.GetOptions{})
	if err != nil {
		if k8serrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	gvkStr, _ := workload.GVKString()
	if gvkStr == types.KubernetesGvkAppsV1DaemonSet {
		ready := values.Lookup(obj.Object, "status", "numberReady")
		readyInt, _ := ready.(int64)
		return readyInt != 0, nil
	}

	ready := values.Lookup(obj.Object, "status", "readyReplicas")
	readyInt, _ := ready.(int64)
	return readyInt != 0, nil
}
