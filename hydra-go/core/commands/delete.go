package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	herrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
)

const maxDeletePasses = 3

// computeNamespaceRefs creates synthetic refs from namespaced entities to their
// Namespace entity so that the topological delete order deletes namespace contents
// before the namespace itself.
func computeNamespaceRefs(entities entity.Entities) ([]htypes.Ref, error) {
	var refs []htypes.Ref
	for _, e := range entities.Items {
		ns, _ := e.Namespace()
		if ns == "" {
			continue
		}
		id, err := e.Id()
		if err != nil {
			return nil, err
		}
		nsId := htypes.Id("v1/Namespace//" + string(ns))
		refs = append(refs, htypes.Ref{
			RefType:      htypes.RefTypeDirect,
			EndpointType: htypes.RefEndpointTypeId,
			From:         id,
			To:           nsId,
			Labels:       []string{htypes.RefLabelNamespace},
		})
	}
	return refs, nil
}

// computeDeleteOrder returns entities in reverse topological order so that
// dependents are deleted before their dependencies (e.g. namespace contents
// before the namespace).
func computeDeleteOrder(entities entity.Entities) ([]entity.Entity, error) {
	refs, err := computeNamespaceRefs(entities)
	if err != nil {
		return nil, err
	}

	graph, err := BuildDependencyGraph(entities, refs)
	if err != nil {
		return nil, err
	}

	plan := PlanTopologicalOrder(graph)
	slices.Reverse(plan)

	ordered := make([]entity.Entity, 0, len(plan))
	for _, entry := range plan {
		id := htypes.Id(entry.Name)
		if e, ok := graph.Entities[id]; ok {
			ordered = append(ordered, e)
		}
	}

	return ordered, nil
}

func dryRunOption(dryRun htypes.DryRun) []string {
	if dryRun {
		return []string{metav1.DryRunAll}
	}
	return nil
}

type ScaleDownTarget struct {
	Id               htypes.Id
	Name             htypes.Name
	Ns               htypes.Namespace
	GVR              htypes.GVR
	GVK              htypes.GVKString
	Replicas         int64
	IsCustomWorkload bool
	ReplicaPaths     []string
}

func scaleTargetsToScaleDownTargets(targets []ScaleTarget) []ScaleDownTarget {
	var out []ScaleDownTarget
	for _, t := range targets {
		if t.IsDaemonSet || t.IsJob {
			continue
		}
		if t.IsCustomWorkload {
			anyPositive := false
			for _, v := range t.OriginalReplicas {
				if v > 0 {
					anyPositive = true
					break
				}
			}
			if !anyPositive {
				continue
			}
			out = append(out, ScaleDownTarget{
				Id:               t.Id,
				Name:             t.Name,
				Ns:               t.Ns,
				GVR:              t.GVR,
				GVK:              t.GVK,
				Replicas:         0,
				IsCustomWorkload: true,
				ReplicaPaths:     t.ReplicaPaths,
			})
			continue
		}
		if t.Replicas == 0 {
			continue
		}
		out = append(out, ScaleDownTarget{
			Id:       t.Id,
			Name:     t.Name,
			Ns:       t.Ns,
			GVR:      t.GVR,
			GVK:      t.GVK,
			Replicas: t.Replicas,
		})
	}
	return out
}

func collectScaleDownTargets(entities entity.Entities, key htypes.EntityKeyUnstructured, customWorkloads ...map[htypes.GVKString]htypes.HydraScaleGroup) ([]ScaleDownTarget, error) {
	var userOverride map[htypes.GVKString]htypes.HydraScaleGroup
	if len(customWorkloads) > 0 && customWorkloads[0] != nil {
		userOverride = customWorkloads[0]
	}
	scaleTargets, err := CollectScaleTargets(entities, key, userOverride)
	if err != nil {
		return nil, err
	}
	return scaleTargetsToScaleDownTargets(scaleTargets), nil
}

func SplitWebhooks(entities entity.Entities) (webhooks entity.Entities, rest entity.Entities, err error) {
	var webhookItems []entity.Entity
	var restItems []entity.Entity
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil {
			return entity.Entities{}, entity.Entities{}, err
		}
		if gvk == htypes.KubernetesGvkAdmissionregistrationK8sIoV1ValidatingWebhookConfiguration ||
			gvk == htypes.KubernetesGvkAdmissionregistrationK8sIoV1MutatingWebhookConfiguration {
			webhookItems = append(webhookItems, e)
		} else {
			restItems = append(restItems, e)
		}
	}
	webhooks, err = entity.NewEntities(webhookItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	rest, err = entity.NewEntities(restItems)
	if err != nil {
		return entity.Entities{}, entity.Entities{}, err
	}
	return webhooks, rest, nil
}

// deleteAdmissionWebhookConfigurations deletes validating and mutating webhook configurations.
// dynamicClient must support cluster-scoped admissionregistration resources.
// It logs the uninstall phase line after a preflight list so the phase can be marked skipped when nothing exists in the cluster.
func deleteAdmissionWebhookConfigurations(
	h hydra.Hydra,
	dynamicClient dynamic.Interface,
	webhookEntities entity.Entities,
	useFooter bool,
	phaseOffset int,
	totalPhases int,
) (map[htypes.Id]bool, int, error) {
	gone := map[htypes.Id]bool{}
	deletedCount := 0
	if webhookEntities.Len() == 0 {
		return gone, 0, nil
	}

	l := h.L()
	dryRun := h.Config().DryRun()
	drp := DryRunPrefix(bool(dryRun))

	presentByID := make(map[htypes.Id]bool, webhookEntities.Len())
	for _, whEntity := range webhookEntities.Items {
		id, idErr := whEntity.Id()
		if idErr != nil {
			return nil, 0, idErr
		}
		gvr, gvrErr := whEntity.GVR()
		if gvrErr != nil {
			return nil, 0, gvrErr
		}
		name, nameErr := whEntity.Name()
		if nameErr != nil {
			return nil, 0, nameErr
		}
		_, getErr := dynamicClient.Resource(gvr.K8s()).Get(context.Background(), string(name), metav1.GetOptions{})
		if getErr != nil {
			if errors.IsNotFound(getErr) {
				continue
			}
			return nil, 0, getErr
		}
		presentByID[id] = true
	}

	phaseSkipped := len(presentByID) == 0
	l.Info(logIdCommands, PhaseMessage(phaseOffset, totalPhases, "deleting webhook configurations", phaseSkipped))

	if phaseSkipped {
		for _, whEntity := range webhookEntities.Items {
			id, idErr := whEntity.Id()
			if idErr != nil {
				return nil, 0, idErr
			}
			gone[id] = true
		}
		if useFooter {
			l.Info(logIdCommands, fmt.Sprintf("%s: skipped (%d planned; none in cluster)",
				uninstallFooterLabel(dryRun, "admission webhooks"), webhookEntities.Len()))
		} else {
			l.Info(logIdCommands, "{prefix}admission webhooks: skipped ({planned} in uninstall plan; none exist in the cluster)",
				log.String("prefix", drp),
				log.Int("planned", webhookEntities.Len()))
		}
		return gone, 0, nil
	}

	var bar log.Progress
	var webhookDetailTask log.ProgressTask
	if useFooter {
		var err error
		bar, err = l.NewProgress(uninstallFooterLabel(dryRun, "admission webhooks"), webhookEntities.Len())
		if err != nil {
			return nil, 0, err
		}
		webhookDetailTask = bar.NewTask("")
	}
	planned := webhookEntities.Len()
	for wi, whEntity := range webhookEntities.Items {
		gvkStr, whErr := whEntity.GVKString()
		if whErr != nil {
			return nil, 0, whErr
		}
		name, whErr := whEntity.Name()
		if whErr != nil {
			return nil, 0, whErr
		}
		id, idErr := whEntity.Id()
		if idErr != nil {
			return nil, 0, idErr
		}

		if !presentByID[id] {
			l.Info(logIdCommands, "{kind} {name} not found, skipping",
				log.String("kind", string(gvkStr)),
				log.String("name", string(name)))
			gone[id] = true
			if bar != nil && webhookDetailTask != nil {
				webhookDetailTask.SetDetail(k8s.TruncateFooterDetail(string(id)))
				bar.Advance(wi+1, planned)
			}
			continue
		}

		l.Info(logIdCommands, drp+"deleting {kind} {name}",
			log.String("kind", string(gvkStr)),
			log.String("name", string(name)))

		if !dryRun {
			gvr, gvrErr := whEntity.GVR()
			if gvrErr != nil {
				return nil, 0, gvrErr
			}
			err := dynamicClient.Resource(gvr.K8s()).Delete(
				context.Background(),
				string(name),
				metav1.DeleteOptions{},
			)
			if err != nil {
				if errors.IsNotFound(err) {
					l.Info(logIdCommands, "{kind} {name} not found, skipping",
						log.String("kind", string(gvkStr)),
						log.String("name", string(name)))
				} else {
					return nil, 0, err
				}
			} else {
				deletedCount++
			}
		}

		gone[id] = true
		if bar != nil && webhookDetailTask != nil {
			webhookDetailTask.SetDetail(k8s.TruncateFooterDetail(string(id)))
			bar.Advance(wi+1, planned)
		}
	}
	if bar != nil {
		_ = bar.Close()
		absent := planned - len(presentByID)
		label := uninstallFooterLabel(dryRun, "admission webhooks")
		if dryRun {
			l.Info(logIdCommands, fmt.Sprintf("%s: dry-run finished for %d planned configuration(s) present in the cluster",
				label, len(presentByID)))
		} else if absent > 0 {
			l.Info(logIdCommands, fmt.Sprintf("%s: deleted %d configuration(s); %d planned object(s) were already absent",
				label, deletedCount, absent))
		} else {
			l.Info(logIdCommands, fmt.Sprintf("%s: deleted %d configuration(s)", label, deletedCount))
		}
	}
	return gone, deletedCount, nil
}

// DeleteAdmissionWebhookConfigurations deletes validating and mutating webhook configurations that are part of an uninstall plan.
// Call before other cluster mutations (e.g. RemoveUninstallFinalizers) so admission controllers do not block PATCH/DELETE on CRs.
// The returned count is admission webhook objects successfully deleted from the API (excludes objects that were already gone).
func DeleteAdmissionWebhookConfigurations(h hydra.Hydra, webhookEntities entity.Entities, phaseOffset int, totalPhases int, useFooter bool) (int, error) {
	if webhookEntities.Len() == 0 {
		return 0, nil
	}
	restConfig, err := RestConfigForHydra(h)
	if err != nil {
		return 0, err
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return 0, err
	}
	_, deleted, err := deleteAdmissionWebhookConfigurations(h, dynamicClient, webhookEntities, useFooter, phaseOffset, totalPhases)
	return deleted, err
}

func uninstallFooterLabel(dryRun htypes.DryRun, tail string) string {
	if dryRun {
		return "dry-run uninstall · " + tail
	}
	return "uninstall · " + tail
}

func DeleteResources(
	h hydra.Hydra,
	deletes entity.Entities,
	key htypes.EntityKeyUnstructured,
	forceScaleDown htypes.ForceScaleDown,
	scaleTimeout time.Duration,
	phaseOffset int,
	totalPhases int,
	useFooter bool,
	webhookPhaseAlreadyDone bool,
) error {
	restConfig, err := RestConfigForHydra(h)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	l := h.L()
	clientCache := cache.NewCache[htypes.GVRNString, dynamic.ResourceInterface](l, "client-cache", true, nil)

	client := func(ns htypes.Namespace, gvr htypes.GVR) (dynamic.ResourceInterface, error) {
		return clientCache.GetOrLoad(htypes.NewGVRNString(gvr, ns), func() (dynamic.ResourceInterface, error) {
			resourceClient := dynamicClient.Resource(gvr.K8s())
			if ns == "" {
				return resourceClient, nil
			}
			return resourceClient.Namespace(string(ns)), nil
		})
	}

	dryRun := h.Config().DryRun()
	drp := DryRunPrefix(bool(dryRun))

	gone := map[htypes.Id]bool{}

	webhookEntities, deletesWithoutWebhooks, splitErr := SplitWebhooks(deletes)
	if splitErr != nil {
		return splitErr
	}
	deletes = deletesWithoutWebhooks

	if !webhookPhaseAlreadyDone {
		if webhookEntities.Len() == 0 {
			l.Info(logIdCommands, PhaseMessage(phaseOffset, totalPhases, "deleting webhook configurations", true))
		} else {
			var whErr error
			gone, _, whErr = deleteAdmissionWebhookConfigurations(h, dynamicClient, webhookEntities, useFooter, phaseOffset, totalPhases)
			if whErr != nil {
				return whErr
			}
		}
	}
	scaleMap, err := ScaleWorkloadMap(h, htypes.HelmNetworkModeOffline)
	if err != nil {
		return err
	}
	targets, err := collectScaleDownTargets(deletes, key, scaleMap)
	if err != nil {
		return err
	}

	l.Info(logIdCommands, PhaseMessage(phaseOffset+1, totalPhases, "scaling down workloads before deletion", len(targets) == 0))
	var scaleBar log.Progress
	var scaleDetailTask log.ProgressTask
	if len(targets) > 0 && useFooter {
		var err error
		scaleBar, err = l.NewProgress(uninstallFooterLabel(dryRun, "workload scale-down"), len(targets))
		if err != nil {
			return err
		}
		scaleDetailTask = scaleBar.NewTask("")
	}
	for ti, tgt := range targets {
		resourceClient, err := client(tgt.Ns, tgt.GVR)
		if err != nil {
			return err
		}
		if tgt.IsCustomWorkload {
			l.Info(logIdCommands, drp+"scaling down custom workload {entity} (replica paths to zero)",
				log.String("entity", string(tgt.Id)))
			patchObj := make(map[string]any)
			for _, path := range tgt.ReplicaPaths {
				parts := strings.Split(path, ".")
				setNestedInt64Patch(patchObj, 0, parts...)
			}
			patchData, mErr := json.Marshal(patchObj)
			if mErr != nil {
				return mErr
			}
			_, err = resourceClient.Patch(
				context.Background(),
				string(tgt.Name),
				types.MergePatchType,
				patchData,
				metav1.PatchOptions{
					DryRun: dryRunOption(dryRun),
				},
			)
		} else {
			l.Info(logIdCommands, drp+"scaling down {entity} from {replicas} to {zero} replicas",
				log.String("entity", string(tgt.Id)),
				log.Int64("replicas", tgt.Replicas),
				log.Int("zero", 0),
			)
			_, err = resourceClient.Patch(
				context.Background(),
				string(tgt.Name),
				types.MergePatchType,
				[]byte(`{"spec":{"replicas":0}}`),
				metav1.PatchOptions{
					DryRun: dryRunOption(dryRun),
				},
			)
		}
		if err != nil {
			if errors.IsNotFound(err) {
				l.DebugLog(logIdCommands, "{entity} not found", log.String("entity", string(tgt.Id)))
				gone[tgt.Id] = true
			} else {
				l.Warn(logIdCommands, "failed to scale down {entity} {err}", log.Err(err), log.String("entity", string(tgt.Id)))
			}
		}
		if scaleBar != nil && scaleDetailTask != nil {
			scaleDetailTask.SetDetail(k8s.TruncateFooterDetail(string(tgt.Id)))
			scaleBar.Advance(ti+1, len(targets))
		}
	}
	if scaleBar != nil {
		_ = scaleBar.Close()
		l.Info(logIdCommands, fmt.Sprintf("%s: completed %d workload(s)",
			uninstallFooterLabel(dryRun, "workload scale-down"), len(targets)))
	}

	// Wait for workloads to actually scale down (skip during dry-run).
	pending := []ScaleDownTarget{}
	for _, tgt := range targets {
		if !gone[tgt.Id] {
			if tgt.IsCustomWorkload {
				continue
			}
			pending = append(pending, tgt)
		}
	}

	if len(pending) > 0 && !dryRun {
		l.Info(logIdCommands, "waiting for {count} workloads to scale down", log.Int("count", len(pending)))

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		timeout := time.After(scaleTimeout)

	pollLoop:
		for {
			select {
			case <-timeout:
				names := make([]string, len(pending))
				for i, p := range pending {
					names[i] = string(p.Id)
				}
				slices.Sort(names)
				workloadList := strings.Join(names, ", ")

				if !forceScaleDown {
					l.Error(logIdCommands, "pods did not terminate within {timeout} for workloads: {workloads}",
						log.String("timeout", scaleTimeout.String()), log.String("workloads", workloadList))
					l.Info(logIdCommands, "to retry uninstall, run the same command again")
					l.Info(logIdCommands, "to re-scale workloads, run: hydra gitops scale <params> up")
					return log.CreateError(herrors.ErrScaleDownTimeout, "aborted: pods did not terminate within {timeout}",
						log.String("timeout", scaleTimeout.String()))
				}

				l.Warn(logIdCommands, "force-deleting pods for workloads that did not scale down: {workloads}",
					log.String("workloads", workloadList))

				podGVR := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
				for _, tgt := range pending {
					workloadClient, cErr := client(tgt.Ns, tgt.GVR)
					if cErr != nil {
						return cErr
					}
					workloadObj, gErr := workloadClient.Get(context.Background(), string(tgt.Name), metav1.GetOptions{})
					if gErr != nil {
						if errors.IsNotFound(gErr) {
							continue
						}
						return gErr
					}
					workloadUID := workloadObj.GetUID()

					podClient := dynamicClient.Resource(podGVR).Namespace(string(tgt.Ns))
					podList, lErr := podClient.List(context.Background(), metav1.ListOptions{})
					if lErr != nil {
						return lErr
					}

					gracePeriod := int64(0)
					for _, pod := range podList.Items {
						for _, ownerRef := range pod.GetOwnerReferences() {
							if ownerRef.UID == workloadUID {
								l.Info(logIdCommands, "force-deleting pod {pod} in namespace {ns}",
									log.String("pod", pod.GetName()),
									log.String("ns", string(tgt.Ns)))
								dErr := podClient.Delete(context.Background(), pod.GetName(), metav1.DeleteOptions{
									GracePeriodSeconds: &gracePeriod,
								})
								if dErr != nil && !errors.IsNotFound(dErr) {
									l.Warn(logIdCommands, "failed to force-delete pod {pod}: {err}",
										log.String("pod", pod.GetName()), log.Err(dErr))
								}
								break
							}
						}
					}
				}
				break pollLoop

			case <-ticker.C:
				stillPending := []ScaleDownTarget{}
				for _, tgt := range pending {
					resourceClient, cErr := client(tgt.Ns, tgt.GVR)
					if cErr != nil {
						return cErr
					}
					result, gErr := resourceClient.Get(context.Background(), string(tgt.Name), metav1.GetOptions{})
					if gErr != nil {
						if errors.IsNotFound(gErr) {
							l.DebugLog(logIdCommands, "{entity} is gone", log.String("entity", string(tgt.Id)))
							continue
						}
						return gErr
					}

					specReplicas := values.Lookup(result.Object, "spec", "replicas")
					if specReplicasInt, ok := specReplicas.(int64); ok && specReplicasInt != 0 {
						l.DebugLog(logIdCommands, "re-patching {entity} to 0 replicas", log.String("entity", string(tgt.Id)))
						_, _ = resourceClient.Patch(
							context.Background(),
							string(tgt.Name),
							types.MergePatchType,
							[]byte(`{"spec":{"replicas":0}}`),
							metav1.PatchOptions{
								DryRun: dryRunOption(dryRun),
							},
						)
						stillPending = append(stillPending, tgt)
						continue
					}

					statusReplicas := values.Lookup(result.Object, "status", "replicas")
					if statusReplicas == nil {
						l.DebugLog(logIdCommands, "{entity} has no status.replicas, considering scaled down",
							log.String("entity", string(tgt.Id)))
						continue
					}
					if statusReplicasInt, ok := statusReplicas.(int64); ok && statusReplicasInt == 0 {
						l.DebugLog(logIdCommands, "{entity} scaled down to 0", log.String("entity", string(tgt.Id)))
						continue
					}

					l.DebugLog(logIdCommands, "{entity} still has running replicas", log.String("entity", string(tgt.Id)))
					stillPending = append(stillPending, tgt)
				}
				pending = stillPending
				if len(pending) == 0 {
					l.Info(logIdCommands, "all workloads scaled down successfully")
					break pollLoop
				}
				l.DebugLog(logIdCommands, "{count} workloads still scaling down", log.Int("count", len(pending)))
			}
		}
	} else if len(pending) > 0 && dryRun {
		l.Info(logIdCommands, "[dry-run] would poll for {count} workloads to scale down", log.Int("count", len(pending)))
	}

	l.Info(logIdCommands, PhaseMessage(phaseOffset+2, totalPhases,
		fmt.Sprintf("deleting %d resources", deletes.Len()), false))

	orderedItems, orderErr := computeDeleteOrder(deletes)
	if orderErr != nil {
		return orderErr
	}

	count := len(orderedItems)
	deleted := map[htypes.Id]bool{}
	for id := range gone {
		deleted[id] = true
	}

	deleteFooterStep := 0
	deleteFooterSeen := map[htypes.Id]bool{}
	var deleteBar log.Progress
	var deleteDetailTask log.ProgressTask
	if useFooter && count > 0 {
		var err error
		deleteBar, err = l.NewProgress(uninstallFooterLabel(dryRun, "delete cluster objects"), count)
		if err != nil {
			return err
		}
		deleteDetailTask = deleteBar.NewTask("")
	}

	for i := range maxDeletePasses {
		if len(deleted) >= count {
			break
		}

		l.DebugLog(logIdCommands, "deletion pass {pass} of {max} for {remaining} of {count} remaining resources",
			log.Int("pass", i+1),
			log.Int("max", maxDeletePasses),
			log.Int("remaining", count-len(deleted)),
			log.Int("count", count),
		)

		for _, item := range orderedItems {
			id, err := item.Id()
			if err != nil {
				return err
			}
			if deleted[id] {
				continue
			}
			if !deleteFooterSeen[id] {
				deleteFooterSeen[id] = true
				deleteFooterStep++
				if deleteBar != nil && deleteDetailTask != nil {
					deleteDetailTask.SetDetail(k8s.TruncateFooterDetail(string(id)))
					deleteBar.Advance(deleteFooterStep, count)
				}
			}

			gvr, err := item.GVR()
			if err != nil {
				return err
			}
			ns, _ := item.Namespace()
			resourceClient, err := client(ns, gvr)
			if err != nil {
				return err
			}
			name, err := item.Name()
			if err != nil {
				return err
			}
			u, err := item.UnstructuredOrError(key)
			if err != nil {
				return err
			}

			if values.Lookup(u.Object, "metadata", "finalizers") != nil {
				l.Info(logIdCommands, drp+"removing finalizers from {entity}", log.String("entity", string(id)))
				_, err = resourceClient.Patch(
					context.Background(),
					string(name),
					types.MergePatchType,
					[]byte(`{"metadata":{"finalizers":[]}}`),
					metav1.PatchOptions{
						DryRun: dryRunOption(dryRun),
					},
				)
				if err != nil {
					if errors.IsNotFound(err) {
						l.DebugLog(logIdCommands, "{entity} not found", log.String("entity", string(id)))
						deleted[id] = true
						continue
					}
					l.Warn(logIdCommands, "failed to remove finalizers {err}", log.Err(err))
				}
			}

			canDelete, err := item.VerbsContains(htypes.VerbDelete)
			if err == nil && !canDelete {
				l.DebugLog(logIdCommands, "skipping deletion of {entity} since delete verb is not allowed",
					log.String("entity", string(id)))
				deleted[id] = true
				continue
			}
			l.Info(logIdCommands, drp+"deleting {entity}", log.String("entity", string(id)))
			b := metav1.DeletePropagationForeground
			err = resourceClient.Delete(
				context.Background(),
				string(name),
				metav1.DeleteOptions{
					DryRun:            dryRunOption(dryRun),
					PropagationPolicy: &b,
				},
			)
			if err != nil && !errors.IsNotFound(err) && !errors.IsMethodNotSupported(err) {
				return err
			}
			deleted[id] = true
		}
	}
	if deleteBar != nil {
		_ = deleteBar.Close()
		l.Info(logIdCommands, fmt.Sprintf("%s: processed %d resource(s)",
			uninstallFooterLabel(dryRun, "delete cluster objects"), count))
	}
	return nil
}
