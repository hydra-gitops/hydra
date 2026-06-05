package commands

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"sync/atomic"

	"hydra-gitops.org/hydra/hydra-go/base/cache"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/k8s"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
)

type discoveryCacheKey struct {
	contextPath types.ContextPath
	clusterName types.ClusterName
}

func (k discoveryCacheKey) String() string {
	return fmt.Sprintf("%s:%s", k.contextPath, k.clusterName)
}

type discoveryResult struct {
	apiResourceLists []*metav1.APIResourceList
	partialErr       error
}

var discoveryCache *cache.Cache[discoveryCacheKey, discoveryResult]

func initDiscoveryCache(l log.Logger) {
	if discoveryCache == nil {
		discoveryCache = cache.NewCache[discoveryCacheKey, discoveryResult](l, "discovery-cache", true, nil)
	}
}

var deprecatedResources = map[types.GVKString]bool{
	"v1/ComponentStatus": true,
	"v1/Endpoints":       true,
}

// discoveryGVKJob is one listable API resource type row after the same filters as [VisitResources].
type discoveryGVKJob struct {
	gv  types.ApiVersion
	r   metav1.APIResource
	gvk types.GVKString
}

// collectDiscoveryGVKJobs walks apiResourceLists that are already sorted (outer list by GroupVersion,
// each inner APIResources slice by Name). It must stay aligned with [visitOneDiscoveryGVK].
func collectDiscoveryGVKJobs(apiResourceLists []*metav1.APIResourceList, handlers *VisitorHandlers) ([]discoveryGVKJob, error) {
	var jobs []discoveryGVKJob
	for _, apiList := range apiResourceLists {
		gv, err := types.ParseApiVersion(apiList.GroupVersion)
		if err != nil {
			continue
		}
		if handlers.HandleApiList != nil {
			handle, err := handlers.HandleApiList(apiList)
			if err != nil {
				return nil, err
			}
			if !handle {
				continue
			}
		}
		for _, r := range apiList.APIResources {
			eb := entity.NewEntityBuilder().
				WithApiVersion(gv).
				WithResource(types.Resource(r.Name)).
				WithKind(types.Kind(r.Kind)).
				WithNamespaced(types.Namespaced(r.Namespaced))
			gvk, err := eb.GVKString()
			if err != nil {
				continue
			}
			// Skip listing core v1/Event only: noisy/legacy; events.k8s.io/v1 Event is listed so CEL
			// involvedObjectEvents(...) and uninstall refs can see the modern Event API in inventory.
			if gvk == types.KubernetesGvkV1Event {
				continue
			}
			if deprecatedResources[gvk] {
				continue
			}
			if !slices.Contains(r.Verbs, "list") {
				continue
			}
			jobs = append(jobs, discoveryGVKJob{gv: gv, r: r, gvk: gvk})
		}
	}
	return jobs, nil
}

func visitOneDiscoveryGVK(
	ctx context.Context,
	dynamicClient dynamic.Interface,
	key types.EntityKeyUnstructured,
	handlers *VisitorHandlers,
	job discoveryGVKJob,
) error {
	gv := job.gv
	r := job.r
	eb := entity.NewEntityBuilder().
		WithApiVersion(gv).
		WithResource(types.Resource(r.Name)).
		WithKind(types.Kind(r.Kind)).
		WithNamespaced(types.Namespaced(r.Namespaced))
	gvr, err := eb.GVR()
	if err != nil {
		return err
	}
	if !r.Namespaced {
		if handlers.HandleClusterResource != nil {
			e, buildErr := eb.Build()
			if buildErr != nil {
				return buildErr
			}
			handle, err := handlers.HandleClusterResource(e, &r)
			if err != nil {
				return err
			}
			if !handle {
				return nil
			}
		}
		if handlers.HandleClusterResourceItem == nil {
			return nil
		}
		resourceClient := dynamicClient.Resource(gvr.K8s())
		resources, err := resourceClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			return log.CreateError(errors.ErrFailedToListResources, "failed to list resources",
				log.String("gvr", gvr.String()), log.Err(err))
		}
		for _, u := range resources.Items {
			verbs := []types.KubernetesVerb{}
			for _, v := range r.Verbs {
				verbs = append(verbs, types.KubernetesVerb(v))
			}
			name := types.Name(u.GetName())
			itemEntity, buildErr := eb.
				WithName(name).
				WithUnstructured(key, u).
				WithVerbs(verbs).
				Build()
			if buildErr != nil {
				return buildErr
			}
			err := handlers.HandleClusterResourceItem(itemEntity)
			if err != nil {
				return err
			}
		}
		return nil
	}
	if handlers.HandleNamespacedResource != nil {
		e, buildErr := eb.Build()
		if buildErr != nil {
			return buildErr
		}
		handle, err := handlers.HandleNamespacedResource(e, &r)
		if err != nil {
			return err
		}
		if !handle {
			return nil
		}
	}
	resourceClient := dynamicClient.Resource(gvr.K8s())
	resources, err := resourceClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return log.CreateError(errors.ErrFailedToListResources, "failed to list namespaced resources",
			log.String("gvr", gvr.String()), log.Err(err))
	}
	namespaces := unstructuredNamespaces(resources.Items)
	itemsByNamespace := unstructuredItemsByNamespace(resources.Items)
	for _, ns := range namespaces {
		nsBuilder := eb.WithNamespace(ns)
		if handlers.HandleNamespacedResourceList != nil {
			nsEntity, buildErr := nsBuilder.Build()
			if buildErr != nil {
				return buildErr
			}
			handled, err := handlers.HandleNamespacedResourceList(nsEntity, itemsByNamespace[ns])
			if err != nil {
				return err
			}
			if !handled {
				continue
			}
		}
		if handlers.HandleNamespacedResourceItem != nil {
			for _, item := range itemsByNamespace[ns] {
				name := types.Name(item.GetName())
				itemEntity, buildErr := nsBuilder.
					WithName(name).
					WithUnstructured(key, item).
					Build()
				if buildErr != nil {
					return buildErr
				}
				err := handlers.HandleNamespacedResourceItem(itemEntity)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// VisitorHandlers contains callback functions for visiting Kubernetes resources
type VisitorHandlers struct {
	HandleApiList                func(*metav1.APIResourceList) (bool, error)
	HandleClusterResource        func(entity.Entity, *metav1.APIResource) (bool, error)
	HandleClusterResourceItem    func(entity.Entity) error
	HandleNamespacedResource     func(entity.Entity, *metav1.APIResource) (bool, error)
	HandleNamespacedResourceList func(entity.Entity, []unstructured.Unstructured) (bool, error)
	HandleNamespacedResourceItem func(entity.Entity) error
}

// VisitResources lists all API resources from the cluster using the provided handlers.
// When quiet is true, partial discovery errors are logged at DEBUG instead of WARN.
// When showProgress is true, footer progress uses one step per listable API resource type (GVK row)
// after discovery, matching the same ordering and filters as this function.
// When parallel is 0, the worker count is [runtime.GOMAXPROCS](0) (clamped to [1, 64]). When parallel
// is greater than 1, that many workers list GVKs concurrently; the footer shows one
// [log.ProgressTask] line per worker. Values above 64 are clamped to 64.
// On success, the returned [log.Progress] must be closed by the caller (after logging a result line if desired).
// On failure, any started progress bar is closed and a failure line is logged before returning.
func VisitResources(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	handlers *VisitorHandlers,
	quiet bool,
	showProgress bool,
	parallel int,
) (p log.Progress, err error) {
	l := cluster.L()
	initDiscoveryCache(l)

	restConfig, err := RestConfigForHydra(cluster)
	if err != nil {
		return nil, err
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	cacheKey := discoveryCacheKey{
		contextPath: cluster.Context.ContextPath,
		clusterName: cluster.ClusterName,
	}

	result, err := discoveryCache.GetOrLoad(cacheKey, func() (discoveryResult, error) {
		discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
		if err != nil {
			return discoveryResult{}, err
		}

		apiResourceLists, err := discoveryClient.ServerPreferredResources()
		if err != nil {
			if errGroupDiscoveryFailed, ok := err.(*discovery.ErrGroupDiscoveryFailed); ok {
				if quiet {
					l.DebugLog(logIdCommands, "some API groups could not be discovered, proceeding with the available resources")
					for gv, partialErr := range errGroupDiscoveryFailed.Groups {
						l.DebugLog(logIdCommands, "failed to discover API group {gv}", log.String("gv", gv.String()), log.Err(partialErr))
					}
				} else {
					l.Warn(logIdCommands, "some API groups could not be discovered, proceeding with the available resources")
					for gv, partialErr := range errGroupDiscoveryFailed.Groups {
						l.Warn(logIdCommands, "failed to discover API group {gv}", log.String("gv", gv.String()), log.Err(partialErr))
					}
				}
				return discoveryResult{apiResourceLists: apiResourceLists, partialErr: err}, nil
			}
			return discoveryResult{}, err
		}

		return discoveryResult{apiResourceLists: apiResourceLists}, nil
	})
	if err != nil {
		return nil, err
	}

	apiResourceLists := result.apiResourceLists

	par := EffectiveClusterWorkerParallelism(parallel)

	// Sort apiResourceLists by GroupVersion for consistent ordering
	slices.SortFunc(apiResourceLists, func(a, b *metav1.APIResourceList) int {
		apiVersionA, err := types.ParseApiVersion(a.GroupVersion)
		if err != nil {
			l.Warn(logIdCommands, "failed to parse GroupVersion {gv}", log.String("gv", a.GroupVersion))
			return 0
		}
		apiVersionB, err := types.ParseApiVersion(b.GroupVersion)
		if err != nil {
			l.Warn(logIdCommands, "failed to parse GroupVersion {gv}", log.String("gv", b.GroupVersion))
			return 0
		}

		return cmp.Or(
			strings.Compare(string(apiVersionA.Group), string(apiVersionB.Group)),
			strings.Compare(string(apiVersionA.Version), string(apiVersionB.Version)),
		)
	})

	for _, apiList := range apiResourceLists {
		if _, err := types.ParseApiVersion(apiList.GroupVersion); err != nil {
			continue
		}
		slices.SortFunc(apiList.APIResources, func(a, b metav1.APIResource) int {
			return strings.Compare(a.Name, b.Name)
		})
	}

	jobs, err := collectDiscoveryGVKJobs(apiResourceLists, handlers)
	if err != nil {
		return nil, err
	}
	gvkProgressTotal := len(jobs)

	if showProgress && gvkProgressTotal > 0 {
		p, err = l.NewProgress("discovery", gvkProgressTotal)
		if err != nil {
			return nil, err
		}
	}
	var detailTask log.ProgressTask
	if p != nil && par <= 1 {
		detailTask = p.NewTask("")
	}
	defer func() {
		if err != nil && p != nil {
			_ = p.Close()
			l.Info(logIdCommands, fmt.Sprintf("discovery: failed: %v", err))
		}
	}()

	listCtx := context.Background()

	if par <= 1 {
		for i, job := range jobs {
			if p != nil && detailTask != nil {
				detailTask.SetDetail(k8s.TruncateFooterDetail(string(job.gvk)))
				p.Advance(i+1, gvkProgressTotal)
			}
			if err := visitOneDiscoveryGVK(listCtx, dynamicClient, key, handlers, job); err != nil {
				return nil, err
			}
		}
		return p, nil
	}

	workerN := par
	if workerN > len(jobs) {
		workerN = len(jobs)
	}
	if workerN < 1 {
		workerN = 1
	}
	workerTasks := make([]log.ProgressTask, workerN)
	if p != nil {
		for w := 0; w < workerN; w++ {
			workerTasks[w] = p.NewTask("")
		}
	}
	var nextIdx atomic.Uint64
	var completedN atomic.Uint64
	var stepErrMu sync.Mutex
	var stepErr error
	var wg sync.WaitGroup
	for w := 0; w < workerN; w++ {
		wg.Add(1)
		wid := w
		go func() {
			defer wg.Done()
			var wt log.ProgressTask
			if wid < len(workerTasks) {
				wt = workerTasks[wid]
			}
			for {
				i := int(nextIdx.Add(1)) - 1
				if i >= len(jobs) {
					return
				}
				job := jobs[i]
				if wt != nil {
					wt.SetDetail(k8s.TruncateFooterDetail(string(job.gvk)))
				}
				e := visitOneDiscoveryGVK(listCtx, dynamicClient, key, handlers, job)
				if e != nil {
					stepErrMu.Lock()
					if stepErr == nil {
						stepErr = e
					}
					stepErrMu.Unlock()
				}
				c := completedN.Add(1)
				if p != nil {
					p.Advance(int(c), gvkProgressTotal)
				}
			}
		}()
	}
	wg.Wait()
	if stepErr != nil {
		return nil, stepErr
	}
	return p, nil
}

func unstructuredNamespaces(items []unstructured.Unstructured) []types.Namespace {
	// Collect unique namespaces from items
	namespaceMap := map[types.Namespace]bool{}
	for _, item := range items {
		ns := item.GetNamespace()
		if ns != "" {
			namespaceMap[types.Namespace(ns)] = true
		}
	}

	// Sort namespaces
	namespaces := []types.Namespace{}
	for ns := range namespaceMap {
		namespaces = append(namespaces, ns)
	}
	slices.Sort(namespaces)

	return namespaces
}

func unstructuredItemsByNamespace(items []unstructured.Unstructured) map[types.Namespace][]unstructured.Unstructured {
	// Group items by namespace
	itemsByNamespace := map[types.Namespace][]unstructured.Unstructured{}
	for _, item := range items {
		ns := types.Namespace(item.GetNamespace())
		itemsByNamespace[ns] = append(itemsByNamespace[ns], item)
	}
	return itemsByNamespace
}
