package commands

import (
	"context"
	"encoding/json"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
)

type FinalizerPatch struct {
	Id                htypes.Id
	FinalizersToKeep  []string
	FinalizersRemoved []string
}

func collectFinalizerPatches(entities entity.Entities, key htypes.EntityKeyUnstructured, finalizers []string) ([]FinalizerPatch, error) {
	if len(finalizers) == 0 {
		return nil, nil
	}

	removeSet := sets.New[string](finalizers...)

	var patches []FinalizerPatch
	for _, item := range entities.Items {
		u, ok := item.Unstructured(key)
		if !ok {
			continue
		}

		raw := values.Lookup(u.Object, "metadata", "finalizers")
		if raw == nil {
			continue
		}

		original, ok := raw.([]any)
		if !ok {
			continue
		}

		var kept []string
		var removed []string
		for _, f := range original {
			s, ok := f.(string)
			if !ok {
				continue
			}
			if removeSet.Has(s) {
				removed = append(removed, s)
			} else {
				kept = append(kept, s)
			}
		}

		if len(removed) > 0 {
			id, err := item.Id()
			if err != nil {
				return nil, err
			}
			patches = append(patches, FinalizerPatch{
				Id:                id,
				FinalizersToKeep:  kept,
				FinalizersRemoved: removed,
			})
		}
	}

	return patches, nil
}

func RemoveUninstallFinalizers(h hydra.Hydra, clusterEntities entity.Entities, appIds sets.Set[htypes.AppId]) (int, error) {
	cluster := h.AsCluster()

	rendered, err := RenderClusterSelectedApps(cluster, htypes.HelmNetworkModeOffline, "", appIds, htypes.KeyTemplateEntity,
		WithSkipFoundDefinitionsInfoLog())
	if err != nil {
		return 0, err
	}

	finalizerNames, err := hydra.HydraAppUninstallFinalizers(cluster, appIds, htypes.HelmNetworkModeOffline, rendered)
	if err != nil {
		return 0, err
	}
	if len(finalizerNames) == 0 {
		return 0, nil
	}

	l := h.L()
	l.Info(logIdCommands, "uninstall-finalizer: loaded {count} configured finalizer name(s) from the selected app(s)",
		log.Int("count", len(finalizerNames)))

	patches, err := collectFinalizerPatches(clusterEntities, htypes.KeyClusterEntity, finalizerNames)
	if err != nil {
		return 0, err
	}
	if len(patches) == 0 {
		l.Info(logIdCommands, "uninstall-finalizer: no objects in the current cluster inventory list those finalizers; nothing to patch.")
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

	patchedOK := 0
	for _, p := range patches {
		for _, item := range clusterEntities.Items {
			id, err := item.Id()
			if err != nil {
				return 0, err
			}
			if id != p.Id {
				continue
			}

			for _, finalizer := range p.FinalizersRemoved {
				if dryRun {
					l.Info(logIdCommands, "[dry-run] removing finalizer {finalizer} from {entity}",
						log.String("finalizer", finalizer),
						log.String("entity", string(id)))
				} else {
					l.Info(logIdCommands, "removing finalizer {finalizer} from {entity}",
						log.String("finalizer", finalizer),
						log.String("entity", string(id)))
				}
			}

			gvr, err := item.GVR()
			if err != nil {
				return 0, err
			}
			ns, _ := item.Namespace()
			name, err := item.Name()
			if err != nil {
				return 0, err
			}

			finalizers := p.FinalizersToKeep
			if finalizers == nil {
				finalizers = []string{}
			}
			patchPayload := map[string]any{
				"metadata": map[string]any{
					"finalizers": finalizers,
				},
			}
			patchBytes, err := json.Marshal(patchPayload)
			if err != nil {
				return 0, err
			}

			resourceClient, err := client(ns, gvr)
			if err != nil {
				return 0, err
			}

			_, err = resourceClient.Patch(
				context.Background(),
				string(name),
				types.MergePatchType,
				patchBytes,
				metav1.PatchOptions{
					DryRun: dryRunOption(dryRun),
				},
			)
			if err != nil {
				if errors.IsNotFound(err) {
					l.DebugLog(logIdCommands, "{entity} not found", log.String("entity", string(id)))
				} else {
					return 0, err
				}
			} else {
				patchedOK++
			}

			break
		}
	}

	return patchedOK, nil
}
