package commands

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
)

func currentCustomReplicaValue(obj map[string]any, path string) int64 {
	value := values.Lookup(obj, splitDotPath(path)...)
	replicas, ok := toInt64(value)
	if !ok {
		return 0
	}
	return replicas
}

func splitDotPath(path string) []string {
	if path == "" {
		return nil
	}
	var parts []string
	start := 0
	for i := 0; i < len(path); i++ {
		if path[i] != '.' {
			continue
		}
		parts = append(parts, path[start:i])
		start = i + 1
	}
	parts = append(parts, path[start:])
	return parts
}

func ScaleDownWillMutate(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	customWorkloads ...map[types.GVKString]types.HydraScaleGroup,
) (bool, error) {
	targets, err := CollectScaleTargets(entities, templateKey, customWorkloads...)
	if err != nil {
		return false, err
	}
	for _, target := range targets {
		item, ok := entities.EntityMap[target.Id]
		if !ok {
			continue
		}
		live, ok := item.Unstructured(liveKey)
		if !ok {
			continue
		}

		switch {
		case target.IsCustomWorkload:
			for _, path := range target.ReplicaPaths {
				if currentCustomReplicaValue(live.Object, path) != 0 {
					return true, nil
				}
			}
		case target.IsJob:
			suspended, _ := values.Lookup(live.Object, "spec", "suspend").(bool)
			if !suspended {
				return true, nil
			}
		case target.IsDaemonSet:
			current := nodeSelectorFromObject(&live)
			if len(current) != 1 || current[hydra.AnnotationHydraScaleDisabled] != "true" {
				return true, nil
			}
		default:
			replicas, ok := toInt64(values.Lookup(live.Object, "spec", "replicas"))
			if !ok || replicas != 0 {
				return true, nil
			}
		}
	}
	return false, nil
}

func ScaleUpWillMutate(
	entities entity.Entities,
	templateKey types.EntityKeyUnstructured,
	liveKey types.EntityKeyUnstructured,
	customWorkloads ...map[types.GVKString]types.HydraScaleGroup,
) (bool, error) {
	targets, err := CollectScaleTargets(entities, templateKey, customWorkloads...)
	if err != nil {
		return false, err
	}
	for _, target := range targets {
		item, ok := entities.EntityMap[target.Id]
		if !ok {
			continue
		}
		live, ok := item.Unstructured(liveKey)
		if !ok {
			continue
		}

		switch {
		case target.IsCustomWorkload:
			for path, expected := range target.OriginalReplicas {
				if currentCustomReplicaValue(live.Object, path) != expected {
					return true, nil
				}
			}
		case target.IsJob:
			suspended, _ := values.Lookup(live.Object, "spec", "suspend").(bool)
			if suspended {
				return true, nil
			}
		case target.IsDaemonSet:
			current := nodeSelectorFromObject(&live)
			currentExists := nodeSelectorFieldExists(&live)
			targetExists := target.NodeSelector != nil
			if currentExists != targetExists || !nodeSelectorEqual(current, target.NodeSelector) {
				return true, nil
			}
		default:
			replicas, ok := toInt64(values.Lookup(live.Object, "spec", "replicas"))
			if !ok || replicas != target.Replicas {
				return true, nil
			}
		}
	}
	return false, nil
}
