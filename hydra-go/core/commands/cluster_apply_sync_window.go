package commands

import (
	"encoding/json"

	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	htypes "hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const appProjectGVK htypes.GVKString = "argoproj.io/v1alpha1/AppProject"

// SetAppProjectSyncWindowsWithMutationCount sets kind and manualSync on every syncWindow entry for each
// AppProject in entities (same semantics as hydra gitops sync auto|manual|prevent). Returns how many
// AppProjects were modified. Projects with no syncWindows log a warning and are left unchanged.
func SetAppProjectSyncWindowsWithMutationCount(
	l log.Logger,
	entities entity.Entities,
	key htypes.EntityKeyUnstructured,
	wantKind string,
	wantManualSync bool,
) (entity.Entities, int, error) {
	var items []entity.Entity
	mutations := 0
	for _, item := range entities.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return entity.Entities{}, 0, err
		}
		if gvk != appProjectGVK {
			items = append(items, item)
			continue
		}

		u, err := item.UnstructuredOrError(key)
		if err != nil {
			items = append(items, item)
			continue
		}

		modified := *u.DeepCopy()
		specMap, ok := modified.Object["spec"].(map[string]any)
		if !ok {
			items = append(items, item)
			continue
		}

		raw, exists := specMap["syncWindows"]
		if !exists || raw == nil {
			name, _ := item.Name()
			l.Warn(logIdCommands, "AppProject '{name}' has no syncWindows configured, skipping sync update",
				log.String("name", string(name)))
			items = append(items, item)
			continue
		}

		syncWindows, ok := raw.([]any)
		if !ok || len(syncWindows) == 0 {
			name, _ := item.Name()
			l.Warn(logIdCommands, "AppProject '{name}' has no syncWindows configured, skipping sync update",
				log.String("name", string(name)))
			items = append(items, item)
			continue
		}

		needsMutation := false
		for _, sw := range syncWindows {
			swMap, ok := sw.(map[string]any)
			if !ok {
				continue
			}
			kind, _ := swMap["kind"].(string)
			ms, hasMS := swMap["manualSync"]
			msBool, _ := ms.(bool)
			if kind != wantKind || !hasMS || msBool != wantManualSync {
				needsMutation = true
				break
			}
		}

		if !needsMutation {
			items = append(items, item)
			continue
		}

		for _, sw := range syncWindows {
			swMap, ok := sw.(map[string]any)
			if !ok {
				continue
			}
			swMap["kind"] = wantKind
			swMap["manualSync"] = wantManualSync
		}

		modifiedEntity, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(key, modified)
		})
		if modErr != nil {
			return entity.Entities{}, 0, modErr
		}
		items = append(items, modifiedEntity)
		mutations++
	}

	out, err := entity.NewEntities(items)
	if err != nil {
		return entity.Entities{}, 0, err
	}
	return out, mutations, nil
}

// indexAppProjectUnstructuredByID maps Hydra resource id to AppProject unstructured (template key).
func indexAppProjectUnstructuredByID(ents entity.Entities, key htypes.EntityKeyUnstructured) (map[htypes.Id]*unstructured.Unstructured, error) {
	idx := make(map[htypes.Id]*unstructured.Unstructured)
	for _, item := range ents.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return nil, err
		}
		if gvk != appProjectGVK {
			continue
		}
		id, err := item.Id()
		if err != nil {
			return nil, err
		}
		u, err := item.UnstructuredOrError(key)
		if err != nil {
			continue
		}
		idx[id] = u.DeepCopy()
	}
	return idx, nil
}

// CopyClusterAppProjectSyncWindowsIntoTemplates replaces spec.syncWindows on template AppProject entities
// with the values from live cluster entities when the same resource id exists in clusterEnts.
func CopyClusterAppProjectSyncWindowsIntoTemplates(
	templateEnts entity.Entities,
	clusterEnts entity.Entities,
	templateKey htypes.EntityKeyUnstructured,
	clusterKey htypes.EntityKeyUnstructured,
) (entity.Entities, error) {
	clusterIdx, err := indexAppProjectUnstructuredByID(clusterEnts, clusterKey)
	if err != nil {
		return entity.Entities{}, err
	}
	if len(clusterIdx) == 0 {
		return templateEnts, nil
	}

	var out []entity.Entity
	for _, item := range templateEnts.Items {
		gvk, err := item.GVKString()
		if err != nil {
			return entity.Entities{}, err
		}
		if gvk != appProjectGVK {
			out = append(out, item)
			continue
		}
		id, err := item.Id()
		if err != nil {
			return entity.Entities{}, err
		}
		clusterU, ok := clusterIdx[id]
		if !ok {
			out = append(out, item)
			continue
		}
		tu, err := item.UnstructuredOrError(templateKey)
		if err != nil {
			return entity.Entities{}, err
		}
		modified := *tu.DeepCopy()
		specMap, ok := modified.Object["spec"].(map[string]any)
		if !ok {
			specMap = map[string]any{}
			modified.Object["spec"] = specMap
		}
		clusterSpec, ok := clusterU.Object["spec"].(map[string]any)
		if !ok {
			delete(specMap, "syncWindows")
		} else if sw, found := clusterSpec["syncWindows"]; found {
			cloned, err := deepCopyJSONValue(sw)
			if err != nil {
				return entity.Entities{}, err
			}
			specMap["syncWindows"] = cloned
		} else {
			delete(specMap, "syncWindows")
		}

		updated, modErr := item.Modify(func(b entity.EntityBuilder) entity.EntityBuilder {
			return b.WithUnstructured(templateKey, modified)
		})
		if modErr != nil {
			return entity.Entities{}, modErr
		}
		out = append(out, updated)
	}
	return entity.NewEntities(out)
}

func deepCopyJSONValue(v any) (any, error) {
	if v == nil {
		return nil, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ApplyClusterApplySyncWindowToEntities applies the selected --sync policy to AppProjects in ents.
// When isNew is true, existing cluster sync configuration is ignored (except for keep-or-* modes, which still
// only copy when isNew is false — callers pass isNew accordingly). clusterEnts is only used for keep-or-* on updates.
func ApplyClusterApplySyncWindowToEntities(
	l log.Logger,
	ents entity.Entities,
	clusterEnts entity.Entities,
	templateKey htypes.EntityKeyUnstructured,
	clusterKey htypes.EntityKeyUnstructured,
	mode htypes.ClusterApplySyncWindow,
	isNew bool,
) (entity.Entities, int, error) {
	switch mode {
	case htypes.ClusterApplySyncWindowDefault:
		return ents, 0, nil
	case htypes.ClusterApplySyncWindowManual, htypes.ClusterApplySyncWindowAuto, htypes.ClusterApplySyncWindowPrevent:
		kind, ms, ok := mode.ClusterSyncKindManual()
		if !ok {
			return ents, 0, nil
		}
		return SetAppProjectSyncWindowsWithMutationCount(l, ents, templateKey, kind, ms)
	case htypes.ClusterApplySyncWindowKeepOrManual, htypes.ClusterApplySyncWindowKeepOrAuto,
		htypes.ClusterApplySyncWindowKeepOrPrevent, htypes.ClusterApplySyncWindowKeepOrDefault:
		if !isNew {
			out, err := CopyClusterAppProjectSyncWindowsIntoTemplates(ents, clusterEnts, templateKey, clusterKey)
			return out, 0, err
		}
		kind, manual, useTemplate := mode.KeepOrNewKindManual()
		if useTemplate {
			return ents, 0, nil
		}
		return SetAppProjectSyncWindowsWithMutationCount(l, ents, templateKey, kind, manual)
	default:
		return ents, 0, nil
	}
}
