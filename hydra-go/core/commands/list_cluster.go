package commands

import (
	"fmt"
	"sync"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	hcel "hydra-gitops.org/hydra/hydra-go/core/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ListClusterAll(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	showProgress bool,
	parallel int,
) (entity.Entities, error) {
	l := cluster.L()
	l.Info(logIdCommands, "listing all resources of cluster '{cluster}'", log.String("cluster", string(cluster.ClusterName)))

	entities, err := ListCluster(cluster, key, func(entity.Entity, types.MissingKeys) (bool, error) {
		return true, nil
	}, showProgress, parallel)
	if err != nil {
		return entity.Entities{}, err
	}

	l.Info(
		logIdCommands,
		"listed {count} resources of cluster '{cluster}'",
		log.Int("count", entities.Len()),
		log.String("cluster", string(cluster.ClusterName)),
	)

	return entities, nil
}

func ListClusterPredicate(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	predicate hcel.Predicate,
) (entity.Entities, error) {
	return ListCluster(cluster, key, predicate.EvalBool, false, 0)
}

// ListClusterStrictPredicate lists only cluster entities that match the CEL predicate at API-resource
// and per-object granularity. Unlike ListClusterPredicate, it never retries without a resource-type
// filter when no API resource type matches at aggregate level — a false empty result is preferable
// to listing the entire cluster (e.g. hydra gitops apply --include/--exclude).
func ListClusterStrictPredicate(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	predicate hcel.Predicate,
	showProgress bool,
	parallel int,
) (entity.Entities, error) {
	l := cluster.L()
	l.Info(logIdCommands,
		"listing cluster '{cluster}' resources matching resource filters (--include/--exclude)",
		log.String("cluster", string(cluster.ClusterName)))

	entities, err := listClusterStrict(cluster, key, predicate.EvalBool, showProgress, parallel)
	if err != nil {
		return entity.Entities{}, err
	}

	l.Info(logIdCommands,
		"listed {count} cluster resources matching resource filters for '{cluster}'",
		log.Int("count", entities.Len()),
		log.String("cluster", string(cluster.ClusterName)))

	return entities, nil
}

func listClusterStrict(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	eval func(entity.Entity, types.MissingKeys) (bool, error),
	showProgress bool,
	parallel int,
) (entity.Entities, error) {
	l := cluster.L()
	l.DebugLog(logIdCommands, "listing cluster '{cluster}' (strict resource filter)",
		log.String("cluster", string(cluster.ClusterName)))

	par := EffectiveClusterWorkerParallelism(parallel)

	result := []entity.Entity{}
	resourceFilter := func(e entity.Entity) (bool, error) {
		return eval(e, types.MissingKeysAccept)
	}
	var resultMu *sync.Mutex
	if par > 1 {
		m := sync.Mutex{}
		resultMu = &m
	}
	handlers := listClusterHandlers(l, &result, resourceFilter, eval, resultMu)
	p, err := VisitResources(cluster, key, handlers, false, showProgress, par)
	if err != nil {
		return entity.Entities{}, err
	}
	defer func() {
		if p != nil {
			_ = p.Close()
			l.Info(logIdCommands, fmt.Sprintf("discovery: listed %d resource(s)", len(result)))
		}
	}()
	return entity.NewEntities(result)
}

func ListCluster(
	cluster *hydra.Cluster,
	key types.EntityKeyUnstructured,
	eval func(entity.Entity, types.MissingKeys) (bool, error),
	showProgress bool,
	parallel int,
) (entity.Entities, error) {
	l := cluster.L()
	l.DebugLog(logIdCommands, "listing cluster '{cluster}'", log.String("cluster", string(cluster.ClusterName)))

	par := EffectiveClusterWorkerParallelism(parallel)

	resourceAccepted := false
	result := []entity.Entity{}

	resourceFilter := func(e entity.Entity) (bool, error) {
		evalResult, err := eval(e, types.MissingKeysAccept)
		if err != nil {
			return false, err
		}
		if evalResult {
			resourceAccepted = true
		}
		return evalResult, nil
	}

	var resultMu *sync.Mutex
	if par > 1 {
		m := sync.Mutex{}
		resultMu = &m
	}
	handlers := listClusterHandlers(l, &result, resourceFilter, eval, resultMu)

	p, err := VisitResources(cluster, key, handlers, false, showProgress, par)
	if err != nil {
		return entity.Entities{}, err
	}

	if !resourceAccepted {
		l.DebugLog(logIdCommands, "no resources matched at resource level, retrying without resource filter")
		result = []entity.Entity{}
		acceptAll := func(e entity.Entity) (bool, error) { return true, nil }
		if p != nil {
			_ = p.Close()
			l.Info(logIdCommands, "discovery: no resource types matched filter; retrying without resource filter")
		}
		handlers = listClusterHandlers(l, &result, acceptAll, eval, resultMu)
		p, err = VisitResources(cluster, key, handlers, false, showProgress, par)
		if err != nil {
			return entity.Entities{}, err
		}
	}

	defer func() {
		if p != nil {
			_ = p.Close()
			l.Info(logIdCommands, fmt.Sprintf("discovery: listed %d resource(s)", len(result)))
		}
	}()

	return entity.NewEntities(result)
}

func listClusterHandlers(
	l log.Logger,
	result *[]entity.Entity,
	resourceFilter func(entity.Entity) (bool, error),
	eval func(entity.Entity, types.MissingKeys) (bool, error),
	resultMu *sync.Mutex,
) *VisitorHandlers {
	appendResult := func(e entity.Entity) {
		if resultMu != nil {
			resultMu.Lock()
			defer resultMu.Unlock()
		}
		*result = append(*result, e)
	}
	return &VisitorHandlers{
		HandleClusterResource: func(e entity.Entity, r *metav1.APIResource) (bool, error) {
			accepted, err := resourceFilter(e)
			if err != nil {
				return false, err
			}
			if accepted {
				gvkString, err := e.GVKString()
				if err != nil {
					return false, err
				}
				l.DebugLog(logIdCommands, "listing GVK '{gvk}'", log.String("gvk", string(gvkString)))
			}
			return accepted, nil
		},
		HandleClusterResourceItem: func(e entity.Entity) error {
			evalResult, err := eval(e, types.MissingKeysError)
			if err != nil {
				return err
			}
			if evalResult {
				name, err := e.Name()
				if err != nil {
					return err
				}
				l.DebugLog(logIdCommands, "  - {name}", log.String("name", string(name)))
				appendResult(e)
			}
			return nil
		},
		HandleNamespacedResource: func(e entity.Entity, r *metav1.APIResource) (bool, error) {
			accepted, err := resourceFilter(e)
			if err != nil {
				return false, err
			}
			if accepted {
				gvkString, err := e.GVKString()
				if err != nil {
					return false, err
				}
				l.DebugLog(logIdCommands, "listing GVK '{gvk}'", log.String("gvk", string(gvkString)))
			}
			return accepted, nil
		},
		HandleNamespacedResourceList: func(e entity.Entity, list []unstructured.Unstructured) (bool, error) {
			evalResult, err := eval(e, types.MissingKeysAccept)
			if err != nil {
				return false, err
			}
			if evalResult {
				namespace, err := e.Namespace()
				if err != nil {
					return false, err
				}
				l.DebugLog(logIdCommands, "  * {namespace}", log.String("namespace", string(namespace)))
			}
			return evalResult, nil
		},
		HandleNamespacedResourceItem: func(e entity.Entity) error {
			evalResult, err := eval(e, types.MissingKeysError)
			if err != nil {
				return err
			}
			if evalResult {
				name, err := e.Name()
				if err != nil {
					return err
				}
				l.DebugLog(logIdCommands, "    - {name}", log.String("name", string(name)))
				appendResult(e)
			}
			return nil
		},
	}
}
