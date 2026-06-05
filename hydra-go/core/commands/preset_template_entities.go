package commands

import (
	"sort"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/hydra"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// PresetTemplateEntities materializes minimal template-shaped entities for the explicit anchor IDs
// declared by every enabled cluster-defaults preset. Each entity is grouped under the synthetic
// builtin app id `{cluster}.preset.{presetId}` ([types.NewPresetAppId]) so the standard
// template-id ownership pipeline (template id -> [AssignClusterEntitiesToAtMostOneAppByRefs] ->
// ownerRefs / workload closure / refs) treats presets as first-class apps without per-preset
// special cases.
//
// CEL-only preset predicates are intentionally not materialized here: only explicit `ids` entries
// from each predicate group are turned into entities. CEL matchers remain a separate, last-resort
// fallback for live cluster objects with no template/ref owner.
//
// k8sMinor gates per-predicate Kubernetes minor windows ([hydra.ClusterDefaultsPredicateMinorApplies]);
// pass <= 0 to disable gating (treated as a sufficiently new cluster).
//
// Returns an error when the same explicit ID is declared by more than one enabled preset
// (single-preset-per-entity invariant).
func PresetTemplateEntities(
	cluster types.ClusterName,
	effective []hydra.ClusterDefaultsPresetEffective,
	k8sMinor int,
) (map[types.AppId]entity.Entities, error) {
	type pendingEntry struct {
		appId  types.AppId
		entity entity.Entity
		preset string
	}
	pending := make(map[types.AppId][]pendingEntry)
	presetByID := make(map[types.Id]string)

	for _, eff := range effective {
		if !eff.Enabled {
			continue
		}
		appId, err := types.NewPresetAppId(cluster, eff.ID)
		if err != nil {
			return nil, err
		}
		for _, pe := range eff.Predicates {
			if !pe.Enabled || !hydra.ClusterDefaultsPredicateMinorApplies(k8sMinor, pe) {
				continue
			}
			for _, idLine := range pe.Ids {
				id := types.Id(idLine.Id)
				if existingPreset, ok := presetByID[id]; ok {
					if existingPreset == eff.ID {
						continue
					}
					return nil, log.CreateError(errors.ErrUninstallAmbiguousRefOwnership,
						"preset anchor id {id} is declared by more than one enabled cluster-defaults preset ({a}, {b})",
						log.String("id", string(id)),
						log.String("a", existingPreset),
						log.String("b", eff.ID))
				}
				ent, eerr := minimalTemplateEntityForPresetAnchor(id, appId)
				if eerr != nil {
					return nil, eerr
				}
				pending[appId] = append(pending[appId], pendingEntry{appId: appId, entity: ent, preset: eff.ID})
				presetByID[id] = eff.ID
			}
		}
	}

	out := make(map[types.AppId]entity.Entities, len(pending))
	appOrder := make([]types.AppId, 0, len(pending))
	for appId := range pending {
		appOrder = append(appOrder, appId)
	}
	sort.Slice(appOrder, func(i, j int) bool { return string(appOrder[i]) < string(appOrder[j]) })
	for _, appId := range appOrder {
		items := pending[appId]
		ents := make([]entity.Entity, 0, len(items))
		for _, it := range items {
			ents = append(ents, it.entity)
		}
		built, err := entity.NewEntities(ents)
		if err != nil {
			return nil, err
		}
		out[appId] = built
	}
	return out, nil
}

// minimalTemplateEntityForPresetAnchor returns a template-shaped entity carrying only the GVK,
// namespace, name and the owning builtin app id derived from a preset anchor id. This is enough
// for the existing template-id ownership pass and ref evaluation (overlay-aware) to attribute
// related cluster resources via refs and workload closure.
func minimalTemplateEntityForPresetAnchor(id types.Id, appId types.AppId) (entity.Entity, error) {
	g, ver, k, ns, name, err := id.Components()
	if err != nil {
		return entity.Entity{}, err
	}
	apiVersion := string(ver)
	if g != "" {
		apiVersion = string(g) + "/" + string(ver)
	}
	u := unstructured.Unstructured{Object: map[string]any{
		"apiVersion": apiVersion,
		"kind":       string(k),
		"metadata": map[string]any{
			"name": string(name),
		},
	}}
	if ns != "" {
		u.Object["metadata"].(map[string]any)["namespace"] = string(ns)
	}
	b := entity.NewEntityBuilder().
		WithGroup(g).
		WithVersion(ver).
		WithKind(k).
		WithName(name).
		WithNamespace(ns).
		WithUnstructured(types.KeyTemplateEntity, u).
		WithAppIds([]types.AppId{appId}).
		WithAppId(appId)
	if ns == "" {
		b = b.WithNamespaced(types.NamespacedNo)
	} else {
		b = b.WithNamespaced(types.NamespacedYes)
	}
	return b.Build()
}
