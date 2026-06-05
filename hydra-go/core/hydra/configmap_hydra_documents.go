package hydra

import (
	"fmt"
	"sort"
	"strconv"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/values"
	coreyaml "hydra-gitops.org/hydra/hydra-go/core/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

// HydraConfigMapDocument is the full parsed YAML from ConfigMap data.hydra for a Hydra config ConfigMap
// (annotation AnnotationHydraConfig: "true").
type HydraConfigMapDocument struct {
	Id        types.Id               `yaml:"id"`
	Namespace string                 `yaml:"namespace"`
	Name      string                 `yaml:"name"`
	Scope     []HydraConfigScopeRule `yaml:"-"`
	Hydra     map[string]any         `yaml:"hydra"` // excludes scope key (cluster merge only)
}

// HydraConfigDocumentsFromEntities returns every v1/ConfigMap with AnnotationHydraConfig enabled and
// non-empty data.hydra, with the full parsed YAML document (not only refs).
func HydraConfigDocumentsFromEntities(entities entity.Entities, key types.EntityKeyUnstructured) ([]HydraConfigMapDocument, error) {
	var out []HydraConfigMapDocument
	seen := sets.New[types.Id]()
	for _, e := range entities.Items {
		gvk, err := e.GVKString()
		if err != nil || gvk != types.KubernetesGvkV1ConfigMap {
			continue
		}
		u, ok := e.Unstructured(key)
		if !ok {
			continue
		}
		id, err := e.Id()
		if err != nil {
			continue
		}
		if seen.Has(id) {
			continue
		}

		ann, _, _ := unstructured.NestedStringMap(u.Object, "metadata", "annotations")
		if !hydraConfigAnnotationEnabled(ann) {
			continue
		}

		data, _, _ := unstructured.NestedStringMap(u.Object, "data")
		hydraYAML, ok := data["hydra"]
		if !ok || hydraYAML == "" {
			continue
		}

		doc, err := coreyaml.FromYaml[map[string]any](types.YamlString(hydraYAML))
		if err != nil {
			return nil, fmt.Errorf("ConfigMap %s data.hydra: %w", id, err)
		}

		scope, err := ExtractAndRemoveScope(doc, id)
		if err != nil {
			return nil, err
		}

		ns, _, _ := unstructured.NestedString(u.Object, "metadata", "namespace")
		name, _, _ := unstructured.NestedString(u.Object, "metadata", "name")

		seen.Insert(id)
		out = append(out, HydraConfigMapDocument{
			Id:        id,
			Namespace: ns,
			Name:      name,
			Scope:     scope,
			Hydra:     doc,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return string(out[i].Id) < string(out[j].Id)
	})

	return out, nil
}

// IsHydraConfigDataConfigMap reports whether u is a v1 ConfigMap that contributes Hydra YAML from
// data.hydra with AnnotationHydraConfig enabled (same eligibility as [HydraConfigDocumentsFromEntities]).
func IsHydraConfigDataConfigMap(u *unstructured.Unstructured) bool {
	if u == nil {
		return false
	}
	if u.GetAPIVersion() != "v1" || u.GetKind() != "ConfigMap" {
		return false
	}
	ann, _, _ := unstructured.NestedStringMap(u.Object, "metadata", "annotations")
	if !hydraConfigAnnotationEnabled(ann) {
		return false
	}
	data, _, _ := unstructured.NestedStringMap(u.Object, "data")
	hydraYAML, ok := data["hydra"]
	return ok && hydraYAML != ""
}

// PartitionHydraConfigDocumentsByApp splits Hydra config ConfigMaps by how they attach to apps.
// - perApp: docs whose entity lists that app in AppIds (chart-owned ConfigMaps).
// - global: docs whose entity has an empty AppIds list (applies to every app when merging values).
//
// Returns an error if a Hydra ConfigMap entity has no appIds (missing key, wrong type, or no matching entity).
func PartitionHydraConfigDocumentsByApp(
	rendered entity.Entities,
	key types.EntityKeyUnstructured,
	clusterAppIds sets.Set[types.AppId],
) (perApp map[types.AppId][]HydraConfigMapDocument, global []HydraConfigMapDocument, err error) {
	full, err := HydraConfigDocumentsFromEntities(rendered, key)
	if err != nil {
		return nil, nil, err
	}
	idToEntity := make(map[types.Id]entity.Entity, len(rendered.Items))
	for i := range rendered.Items {
		e := rendered.Items[i]
		id, idErr := e.Id()
		if idErr != nil {
			continue
		}
		idToEntity[id] = e
	}

	perApp = make(map[types.AppId][]HydraConfigMapDocument)
	for _, doc := range full {
		e, ok := idToEntity[doc.Id]
		if !ok {
			return nil, nil, log.CreateError(errors.ErrHydraConfigError,
				"Hydra ConfigMap {id} ({namespace}/{name}) has no matching rendered entity; appIds cannot be resolved",
				log.String("id", string(doc.Id)),
				log.String("namespace", doc.Namespace),
				log.String("name", doc.Name))
		}
		appIds, errApp := e.AppIds()
		if errApp != nil {
			return nil, nil, log.CreateError(errors.ErrHydraConfigError,
				"Hydra ConfigMap {id} ({namespace}/{name}) requires entity appIds (noAppIds)",
				log.String("id", string(doc.Id)),
				log.String("namespace", doc.Namespace),
				log.String("name", doc.Name),
				log.Err(errApp))
		}
		if len(appIds) == 0 {
			global = append(global, doc)
			continue
		}
		for _, aid := range appIds {
			if clusterAppIds.Has(aid) {
				perApp[aid] = append(perApp[aid], doc)
			}
		}
	}
	return perApp, global, nil
}

// MergeHelmHydraWithConfigMapDocuments deep-merges Helm global.hydra (as a map) with each
// ConfigMap data.hydra in order (docs must already be sorted deterministically). For duplicate
// keys, later documents override per values.MergeValues semantics (nested maps merge; scalars
// and slices are replaced by the right-hand value).
func MergeHelmHydraWithConfigMapDocuments(helmHydra map[string]any, docs []HydraConfigMapDocument) types.ValuesMap {
	merged := types.ValuesMap{}
	if helmHydra != nil {
		merged = values.MergeValues(merged, helmHydra)
	}
	for i := range docs {
		if docs[i].Hydra != nil {
			merged = values.MergeValues(merged, docs[i].Hydra)
		}
	}
	return merged
}

func hydraConfigAnnotationEnabled(annotations map[string]string) bool {
	v, ok := annotations[AnnotationHydraConfig]
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	return err == nil && b
}
