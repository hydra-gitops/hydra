package entity

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/helm"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
)

func parseCRD(u unstructured.Unstructured) (*apiextv1.CustomResourceDefinition, error) {
	var crd apiextv1.CustomResourceDefinition
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &crd); err != nil {
		return nil, err
	}
	return &crd, nil
}

type Entities struct {
	// TODO rename
	Items     []Entity
	EntityMap EntityMap
	IdSet     sets.Set[types.Id]
	IdList    []types.Id
}

func MergeEntities(items ...[]Entity) (Entities, error) {
	merged := []Entity{}
	for _, itemSlice := range items {
		merged = append(merged, itemSlice...)
	}
	return NewEntities(merged)
}

// unstructuredWithSource pairs an unstructured object with its template source info
type unstructuredWithSource struct {
	u             unstructured.Unstructured
	templatePath  types.TemplatePath
	templateIndex types.TemplateIndex
}

func NewEntitiesFromYaml(
	l log.Logger,
	manifest types.YamlString,
	key types.EntityKeyUnstructured,
) (Entities, error) {
	us := []unstructuredWithSource{}
	for path, ys := range helm.SplitManifestMap(manifest) {
		for i, y := range ys {
			u, err := yaml.YamlToUnstructured(y)
			if err != nil {
				return Entities{}, err
			}
			if u.GetName() != "" && u.GetKind() != "" {
				us = append(us, unstructuredWithSource{
					u:             *u,
					templatePath:  types.TemplatePath(path),
					templateIndex: types.TemplateIndex(i + 1),
				})
			}
		}
	}

	slices.SortFunc(us, func(a, b unstructuredWithSource) int {
		return strings.Compare(a.u.GetNamespace(), b.u.GetNamespace())
	})

	result := []Entity{}
	for _, item := range us {
		u := item.u
		ns := u.GetNamespace()

		gvk := types.NewGVKFromK8s(u.GroupVersionKind())
		b := NewEntityBuilder().
			WithGVK(gvk).
			WithName(types.Name(u.GetName())).
			WithTemplatePath(item.templatePath).
			WithTemplateIndex(item.templateIndex)

		if ns != "" {
			b = b.
				WithNamespace(types.Namespace(ns)).
				WithNamespaced(types.NamespacedNo)
		}

		id, err := b.Id()
		if err != nil {
			return Entities{}, err
		}

		if ns == "" {
			l.DebugLog(logIdEntities, "found {id}", log.String("id", string(id)))
		} else {
			l.DebugLog(logIdEntities, "found {id} (default namespace {ns})",
				log.String("id", string(id)),
				log.String("ns", ns))
		}

		e, err := b.WithUnstructured(key, u).Build()
		if err != nil {
			return Entities{}, err
		}
		result = append(result, e)
	}

	return NewEntities(result)
}

func NewEntities(entities []Entity) (Entities, error) {
	entityMap, err := NewEntityMap(entities)
	if err != nil {
		return Entities{}, err
	}
	result := Entities{
		Items:     entities,
		EntityMap: entityMap,
		IdSet:     EntityMapIds(entityMap),
	}
	if result.IdSet.Len() != len(entities) {
		grouped, err := result.groupById()
		if err != nil {
			return Entities{}, err
		}

		ids := slices.SortedFunc(maps.Keys(grouped), func(a, b types.Id) int {
			return cmp.Compare(string(a), string(b))
		})

		deduped := make([]Entity, 0, len(ids))
		for _, id := range ids {
			group := grouped[id]
			count := len(group)
			if count > 1 {
				scope := duplicateAppScope(group)
				breakdown := duplicateEntitySourcesBreakdown(group)
				var msg string
				if scope == duplicateScopeSameApp {
					msg = fmt.Sprintf(
						"duplicate entity id %s within a single Helm app/chart render (%d occurrences); keeping last object only%s",
						string(id), count, breakdown)
				} else {
					msg = fmt.Sprintf(
						"duplicate entity id %s in template render (%d occurrences, scope=%s); keeping last object only%s",
						string(id), count, scope, breakdown)
				}
				l := log.Default()
				l.Warn(logIdEntities, msg,
					log.String("id", string(id)),
					log.Int("count", count),
					log.String("duplicateScope", scope),
				)
			}
			deduped = append(deduped, group[count-1])
		}
		return NewEntities(deduped)
	}
	return result, nil
}

const (
	duplicateScopeUnknown  = "unknown"
	duplicateScopeSameApp  = "same-app"
	duplicateScopeCrossApp = "cross-app"
)

// duplicateAppScope classifies duplicate entities: same Kubernetes id emitted from one app
// (e.g. duplicate manifests inside one Helm chart) vs multiple apps.
func duplicateAppScope(group []Entity) string {
	appSet := sets.New[types.AppId]()
	for _, e := range group {
		appIds, err := e.AppIds()
		if err != nil {
			continue
		}
		for _, a := range appIds {
			appSet.Insert(a)
		}
	}
	switch appSet.Len() {
	case 0:
		return duplicateScopeUnknown
	case 1:
		return duplicateScopeSameApp
	default:
		return duplicateScopeCrossApp
	}
}

type duplicateSourceKey struct {
	app string
	tpl string
}

// duplicateEntitySourcesBreakdown aggregates duplicate entities by app id(s) and template path
// and returns a multi-line list: "  - app / template (n)" per line.
func duplicateEntitySourcesBreakdown(group []Entity) string {
	counts := make(map[duplicateSourceKey]int)
	for _, e := range group {
		var key duplicateSourceKey
		if appIds, err := e.AppIds(); err == nil && len(appIds) > 0 {
			strs := make([]string, len(appIds))
			for i, a := range appIds {
				strs[i] = string(a)
			}
			slices.Sort(strs)
			key.app = strings.Join(strs, ", ")
		}
		if key.app == "" {
			key.app = "(unknown app)"
		}
		if tp, err := e.TemplatePath(); err == nil && tp != "" {
			key.tpl = string(tp)
		}
		if key.tpl == "" {
			key.tpl = "(unknown template)"
		}
		counts[key]++
	}
	keys := slices.SortedFunc(maps.Keys(counts), func(a, b duplicateSourceKey) int {
		if c := cmp.Compare(a.app, b.app); c != 0 {
			return c
		}
		return cmp.Compare(a.tpl, b.tpl)
	})
	var b strings.Builder
	b.WriteString("\n")
	for _, k := range keys {
		n := counts[k]
		fmt.Fprintf(&b, "  - %s / %s (%d)\n", k.app, k.tpl, n)
	}
	return b.String()
}

func (entities Entities) Len() int {
	return len(entities.Items)
}

func (entities Entities) Add(other Entities) (Entities, error) {
	return MergeEntities(entities.Items, other.Items)
}

func (entities Entities) UnselectAll() (Entities, error) {
	result := []Entity{}
	for _, e := range entities.Items {
		modified, err := e.Modify(func(b EntityBuilder) EntityBuilder {
			return b.WithoutSelected()
		})
		if err != nil {
			return Entities{}, err
		}
		result = append(result, modified)
	}
	return NewEntities(result)
}

func (entities Entities) ScopeInfoMapFromCrds(keys ...types.EntityKeyUnstructured) (types.ScopeInfoMap, error) {
	if entities.Len() == 0 {
		return types.ScopeInfoMap{}, nil
	}
	if len(keys) == 0 {
		return nil, log.CreateError(errors.ErrCrdGvkMismatch, "no keys provided to extract CRD items")
	}

	_, crds, err := entities.SelectCrds()
	if err != nil {
		return nil, err
	}

	scopeInfoMap := types.ScopeInfoMap{}
	for _, crd := range crds.Items {
		gvk, err := crd.GVKString()
		if err != nil {
			return nil, err
		}
		name, err := crd.Name()
		if err != nil {
			return nil, err
		}

		if gvk != types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition {
			return nil, log.CreateError(errors.ErrCrdGvkMismatch, "expected CRD '{name}' to have GVK {crdGvk}, found {gvk}",
				log.String("name", string(name)),
				log.String("crdGvk", string(types.KubernetesGvkApiextensionsK8sIoV1CustomResourceDefinition)),
				log.String("gvk", string(gvk)))
		}

		var parsedCrd *apiextv1.CustomResourceDefinition
		for _, key := range keys {
			item, ok := crd.Unstructured(key)
			if ok {
				parsedCrd, err = parseCRD(item)
				if err != nil {
					return nil, err
				}
				break
			}
		}
		if parsedCrd == nil {
			return nil, log.CreateError(errors.ErrCrdGvkMismatch, "failed to parse CRD {name}",
				log.String("name", string(name)))
		}

		ri := types.ScopeInfo{
			Resource:   types.Resource(parsedCrd.Spec.Names.Plural),
			Namespaced: parsedCrd.Spec.Scope == apiextv1.NamespaceScoped,
		}

		for _, version := range parsedCrd.Spec.Versions {
			parsedCrdGvk := types.NewGVK(
				types.Group(parsedCrd.Spec.Group),
				types.Version(version.Name),
				types.Kind(parsedCrd.Spec.Names.Kind),
			)

			scopeInfoMap[parsedCrdGvk.GVKString()] = ri
		}
	}

	return scopeInfoMap, nil
}

func (entities Entities) Required(key types.EntityKeyUnstructured) (Entities, error) {
	items := []Entity{}
	for _, e := range entities.Items {
		_, ok := e.Unstructured(key)
		if !ok {
			continue
		}
		items = append(items, e)
	}
	return NewEntities(items)
}

func (entities Entities) Selected() (Entities, error) {
	result := []Entity{}
	for _, e := range entities.Items {
		if e.Selected() {
			result = append(result, e)
		}
	}
	r, err := NewEntities(result)
	if err != nil {
		return Entities{}, err
	}
	return r.UnselectAll()
}

func (entities Entities) WithAppNamespace(appNamespace types.AppNamespace) (Entities, error) {
	result := []Entity{}
	for _, e := range entities.Items {
		modified, err := e.Modify(func(b EntityBuilder) EntityBuilder {
			return b.WithAppNamespace(appNamespace)
		})
		if err != nil {
			return Entities{}, err
		}
		result = append(result, modified)
	}
	return NewEntities(result)
}

func (entities Entities) MoveItems(from types.EntityKeyUnstructured, to types.EntityKeyUnstructured) (Entities, error) {
	result := []Entity{}
	for _, e := range entities.Items {
		u, ok := e.Unstructured(from)
		if ok {
			modified, err := e.Modify(func(b EntityBuilder) EntityBuilder {
				return b.WithoutUnstructured(from).WithUnstructured(to, u)
			})
			if err != nil {
				return Entities{}, err
			}
			e = modified
		}
		result = append(result, e)
	}

	return NewEntities(result)
}

func (entities Entities) CopyItems(from types.EntityKeyUnstructured, to types.EntityKeyUnstructured) (Entities, error) {
	result := []Entity{}
	for _, e := range entities.Items {
		u, ok := e.Unstructured(from)
		if ok {
			modified, err := e.Modify(func(b EntityBuilder) EntityBuilder {
				return b.WithUnstructured(to, u)
			})
			if err != nil {
				return Entities{}, err
			}
			e = modified
		}
		result = append(result, e)
	}

	return NewEntities(result)
}

func (entities Entities) Append(other Entities) (Entities, error) {
	return NewEntities(append(slices.Clone(entities.Items), other.Items...))
}

func (entities Entities) Without(other Entities) (Entities, error) {
	items := []Entity{}
	for id, item := range entities.EntityMap {
		if !other.IdSet.Has(id) {
			items = append(items, item)
		}
	}

	return NewEntities(items)
}

func (entities Entities) ToYaml(key types.EntityKeyUnstructured) (types.YamlString, error) {
	sb := strings.Builder{}
	first := true
	for _, e := range entities.Items {
		item, ok := e.Unstructured(key)
		if !ok {
			continue
		}
		if first {
			first = false
		} else {
			sb.WriteString("\n---\n")
		}
		str, err := yaml.PrintObject(types.KeepServerFieldsNo, nil, &item)
		if err != nil {
			return "", err
		}
		sb.WriteString(string(str))
	}
	sb.WriteString("\n")
	return types.YamlString(sb.String()), nil
}

// UidMap creates a map from Kubernetes UIDs to entities for quick lookups.
// Only entities that have a UID in the specified unstructured key are included.
// This is useful for resolving owner references and building relationship graphs.
func (entities Entities) UidMap(key types.EntityKeyUnstructured) map[types.Uid]Entity {
	uidMap := map[types.Uid]Entity{}
	for _, e := range entities.Items {
		uid, ok := e.Uid(key)
		if !ok {
			continue
		}
		uidMap[uid] = e
	}
	return uidMap
}

// RootOwnerUidMap builds a map of root owners to all UIDs they own (directly or indirectly).
// A root owner is an entity that has no owner references itself. The function recursively
// traverses the ownership chain to find the topmost owner and collects all UIDs in that chain.
//
// For example, if A owns B and B owns C, the result will be: {A: {B, C}}
// Note: The root owner itself is NOT included in its own set of owned UIDs.
//
// This is useful for grouping related resources by their top-level parent, such as
// finding all resources that belong to a Deployment (ReplicaSet, Pods, etc.).
func (entities Entities) RootOwnerUidMap(key types.EntityKeyUnstructured) map[types.Uid]sets.Set[types.Uid] {
	uidMap := entities.UidMap(key)
	result := map[types.Uid]sets.Set[types.Uid]{}

	var findRootOwner func(uid types.Uid, visited sets.Set[types.Uid]) (types.Uid, sets.Set[types.Uid])
	findRootOwner = func(uid types.Uid, visited sets.Set[types.Uid]) (types.Uid, sets.Set[types.Uid]) {
		if visited.Has(uid) {
			return uid, visited
		}
		visited = visited.Insert(uid)

		e, exists := uidMap[uid]
		if !exists {
			return uid, visited
		}

		ownerUids := e.OwnerUids(key)
		if ownerUids == nil || ownerUids.Len() == 0 {
			return uid, visited
		}

		for ownerUid := range ownerUids {
			rootUid, allVisited := findRootOwner(ownerUid, visited)
			return rootUid, allVisited
		}

		return uid, visited
	}

	for uid := range uidMap {
		rootUid, ownedUids := findRootOwner(uid, sets.New[types.Uid]())

		if _, exists := result[rootUid]; !exists {
			result[rootUid] = sets.New[types.Uid]()
		}

		for ownedUid := range ownedUids {
			if ownedUid != rootUid {
				result[rootUid] = result[rootUid].Insert(ownedUid)
			}
		}
	}

	return result
}
