package entity

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

type entityData map[types.EntityKey]types.Value

// entityBase holds the shared data map and provides all getter methods
// that are common to both Entity and EntityBuilder.
type entityBase struct {
	data entityData
}

// Entity is an immutable entity with a pre-computed Id.
// It can only be created via EntityBuilder.Build().
type Entity struct {
	entityBase
	id    types.Id
	cache entityCache
}

var _ IdEntity = Entity{}

type entityCache struct {
	group         types.Group
	version       types.Version
	resource      types.Resource
	kind          types.Kind
	namespace     types.Namespace
	appNamespace  types.AppNamespace
	name          types.Name
	namespaced    types.Namespaced
	gvk           types.GVK
	gvr           types.GVR
	apiVersion    types.ApiVersion
	gvkString     types.GVKString
	gvknString    string
	apiVersionStr string
	gvrString     types.GVRString
	hasResource   bool
	hasNamespace  bool
	hasAppNs      bool
	hasName       bool
	hasNamespaced bool
}

func (e Entity) Id() (types.Id, error) {
	return e.id, nil
}

func (e Entity) Group() (types.Group, error) {
	return e.cache.group, nil
}

func (e Entity) Version() (types.Version, error) {
	return e.cache.version, nil
}

func (e Entity) Resource() (types.Resource, error) {
	if !e.cache.hasResource {
		return e.entityBase.Resource()
	}
	return e.cache.resource, nil
}

func (e Entity) Kind() (types.Kind, error) {
	return e.cache.kind, nil
}

func (e Entity) Namespace() (types.Namespace, error) {
	if !e.cache.hasNamespace {
		return e.entityBase.Namespace()
	}
	return e.cache.namespace, nil
}

func (e Entity) HasNamespace() bool {
	return e.cache.hasNamespace
}

func (e Entity) AppNamespace() (types.AppNamespace, error) {
	if !e.cache.hasAppNs {
		return e.entityBase.AppNamespace()
	}
	return e.cache.appNamespace, nil
}

func (e Entity) Namespaced() (types.Namespaced, error) {
	if !e.cache.hasNamespaced {
		return e.entityBase.Namespaced()
	}
	return e.cache.namespaced, nil
}

func (e Entity) Name() (types.Name, error) {
	if !e.cache.hasName {
		return e.entityBase.Name()
	}
	return e.cache.name, nil
}

// GVKNString returns gvk for cluster-scoped resources and gvk/ns for namespaced resources,
// matching [types.Id.GVKNString] (same string as the TUI filter field GVKN).
func (e Entity) GVKNString() (string, error) {
	return e.cache.gvknString, nil
}

func (e Entity) GVK() (types.GVK, error) {
	return e.cache.gvk, nil
}

func (e Entity) GVR() (types.GVR, error) {
	if !e.cache.hasResource {
		return e.entityBase.GVR()
	}
	return e.cache.gvr, nil
}

func (e Entity) GVKString() (types.GVKString, error) {
	return e.cache.gvkString, nil
}

func (e Entity) ApiVersion() (types.ApiVersion, error) {
	return e.cache.apiVersion, nil
}

func (e Entity) ApiVersionString() (string, error) {
	return e.cache.apiVersionStr, nil
}

func (e Entity) GVRString() (types.GVRString, error) {
	if !e.cache.hasResource {
		return e.entityBase.GVRString()
	}
	return e.cache.gvrString, nil
}

func (e Entity) IdUnknownNamespace() (string, error) {
	if !e.cache.hasAppNs {
		return e.entityBase.IdUnknownNamespace()
	}
	namespace := e.cache.namespace
	if !e.cache.hasNamespace {
		namespace = "<unknown>"
	}
	return fmt.Sprintf("%s/%s/%s/%s", e.cache.gvkString, namespace, e.cache.name, e.cache.appNamespace), nil
}

func (e Entity) ToBuilder() EntityBuilder {
	return EntityBuilder{entityBase: entityBase{data: maps.Clone(e.data)}}
}

func (e Entity) Modify(fn func(EntityBuilder) EntityBuilder) (Entity, error) {
	return fn(e.ToBuilder()).Build()
}

func (e Entity) Equal(other Entity) bool {
	if e.id != other.id {
		return false
	}
	if len(e.data) != len(other.data) {
		return false
	}
	for k, v := range e.data {
		ov, ok := other.data[k]
		if !ok {
			return false
		}
		if fmt.Sprintf("%v", v) != fmt.Sprintf("%v", ov) {
			return false
		}
	}
	return true
}

func (e Entity) String() string {
	parts := []string{fmt.Sprintf("id=%s", e.id)}
	for key := range types.EntityKeys() {
		if v, ok := e.data[key]; ok {
			parts = append(parts, fmt.Sprintf("%s=%v", key, v))
		}
	}
	return strings.Join(parts, ", ")
}

// --- entityBase: shared getter methods ---

func (e entityBase) Group() (types.Group, error) {
	return dataString[types.Group](e.data, types.KeyGroup)
}

func (e entityBase) Version() (types.Version, error) {
	return dataString[types.Version](e.data, types.KeyVersion)
}

func (e entityBase) Resource() (types.Resource, error) {
	return dataString[types.Resource](e.data, types.KeyResource)
}

func (e entityBase) Kind() (types.Kind, error) {
	return dataString[types.Kind](e.data, types.KeyKind)
}

func (e entityBase) Namespace() (types.Namespace, error) {
	return dataString[types.Namespace](e.data, types.KeyNamespace)
}

func (e entityBase) HasNamespace() bool {
	_, ok := e.data[types.KeyNamespace]
	return ok
}

func (e entityBase) AppNamespace() (types.AppNamespace, error) {
	return dataString[types.AppNamespace](e.data, types.KeyAppNamespace)
}

func (e entityBase) Namespaced() (types.Namespaced, error) {
	return dataBool[types.Namespaced](e.data, types.KeyNamespaced)
}

func (e entityBase) Name() (types.Name, error) {
	return dataString[types.Name](e.data, types.KeyName)
}

func (e entityBase) Verbs() ([]types.KubernetesVerb, error) {
	return dataSlice(e.data, types.KeyVerbs)
}

func (e entityBase) VerbsContains(verb types.KubernetesVerb) (bool, error) {
	verbs, err := e.Verbs()
	if err != nil {
		return false, err
	}
	return slices.Contains(verbs, verb), nil
}

func (e entityBase) Selected() types.Selected {
	selected, err := dataBool[types.Selected](e.data, types.KeySelected)
	if err != nil {
		return types.SelectedNo
	}
	return selected
}

func (e entityBase) AppOwned() bool {
	appOwned, err := dataBool[bool](e.data, types.KeyAppOwned)
	if err != nil {
		return false
	}
	return appOwned
}

func (e entityBase) BuiltIn() bool {
	builtIn, err := dataBool[bool](e.data, types.KeyBuiltIn)
	if err != nil {
		return false
	}
	return builtIn
}

func (e entityBase) Ownership() types.EntityOwnership {
	if e.AppOwned() {
		return types.EntityOwnershipAppOwned
	}
	if e.BuiltIn() {
		return types.EntityOwnershipBuiltIn
	}
	return types.EntityOwnershipUntracked
}

func (e entityBase) AppIds() ([]types.AppId, error) {
	return dataSlice(e.data, types.KeyAppIds)
}

// AppId returns the primary owning Hydra app id when set ([types.KeyAppId]), otherwise the first
// entry from [types.KeyAppIds] when present.
func (e entityBase) AppId() (types.AppId, error) {
	if _, ok := e.data[types.KeyAppId]; ok {
		return dataString[types.AppId](e.data, types.KeyAppId)
	}
	appIds, err := e.AppIds()
	if err != nil {
		return "", err
	}
	if len(appIds) == 0 {
		return "", fmt.Errorf("entity has no template app id")
	}
	return appIds[0], nil
}

func (e entityBase) Unstructured(key types.EntityKeyUnstructured) (unstructured.Unstructured, bool) {
	item, ok := e.data[key]
	if !ok {
		return unstructured.Unstructured{}, false
	}
	u := item.(types.ValueUnstructured).Unstructured()
	return u, true
}

// ReadOnlyUnstructured returns the stored Kubernetes object without a defensive deep copy.
// Callers must treat the returned value as immutable.
func (e entityBase) ReadOnlyUnstructured(key types.EntityKeyUnstructured) (unstructured.Unstructured, bool) {
	item, ok := e.data[key]
	if !ok {
		return unstructured.Unstructured{}, false
	}
	return item.(types.ValueUnstructured).RawUnstructured(), true
}

func (e entityBase) UnstructuredOrError(key types.EntityKeyUnstructured) (unstructured.Unstructured, error) {
	value, ok := e.data[key]
	if !ok {
		return unstructured.Unstructured{}, log.CreateError(errors.ErrKeyNotFound, "key {key} not found in entity",
			log.String("key", key.String()))
	}
	u := value.(types.ValueUnstructured).Unstructured()
	return u, nil
}

func (e entityBase) Uid(key types.EntityKeyUnstructured) (types.Uid, bool) {
	u, ok := e.Unstructured(key)
	if !ok {
		return "", false
	}
	return types.Uid(u.GetUID()), true
}

func (e entityBase) ReadOnlyUid(key types.EntityKeyUnstructured) (types.Uid, bool) {
	u, ok := e.ReadOnlyUnstructured(key)
	if !ok {
		return "", false
	}
	return types.Uid(u.GetUID()), true
}

func (e entityBase) TemplatePath() (types.TemplatePath, error) {
	return dataString[types.TemplatePath](e.data, types.KeyTemplatePath)
}

func (e entityBase) RepoPath() (types.RepoPath, error) {
	return dataString[types.RepoPath](e.data, types.KeyRepoPath)
}

func (e entityBase) AbsPath() (types.AbsPath, error) {
	return dataString[types.AbsPath](e.data, types.KeyAbsPath)
}

func (e entityBase) TemplateIndex() (types.TemplateIndex, error) {
	return dataInt[types.TemplateIndex](e.data, types.KeyTemplateIndex)
}

func (e entityBase) OwnerUids(key types.EntityKeyUnstructured) sets.Set[types.Uid] {
	u, ok := e.Unstructured(key)
	if !ok {
		return nil
	}
	result := sets.New[types.Uid]()
	for _, owner := range u.GetOwnerReferences() {
		result = result.Insert(types.Uid(owner.UID))
	}
	return result
}

func (e entityBase) HasKey(key types.EntityKey) bool {
	_, ok := e.data[key]
	return ok
}

// --- internal helper functions operating on entityData ---

func lookupData(data entityData, key types.EntityKey) (types.Value, error) {
	result, ok := data[key]
	if !ok {
		return nil, log.CreateError(errors.ErrKeyNotFound, "key {key} not found in entity",
			log.String("key", key.String()))
	}
	return result, nil
}

func dataString[T ~string](data entityData, key types.EntityKeyString) (T, error) {
	result, err := lookupData(data, key)
	if err != nil {
		return "", err
	}
	value, ok := result.Value().(string)
	if !ok {
		return "", log.CreateError(errors.ErrKeyTypeMismatch, "key {key} value has unexpected type, expected string: '{value}'",
			log.String("key", string(key)), log.Any("value", result.Value()))
	}
	return T(value), nil
}

func dataBool[T ~bool](data entityData, key types.EntityKeyBool) (T, error) {
	result, err := lookupData(data, key)
	if err != nil {
		return false, err
	}
	value, ok := result.Value().(bool)
	if !ok {
		return false, log.CreateError(errors.ErrKeyTypeMismatch, "key {key} value has unexpected type, expected bool: '{value}'",
			log.String("key", string(key)), log.Any("value", result.Value()))
	}
	return T(value), nil
}

func dataSlice[T ~string](data entityData, key types.EntityKeySlice[T]) ([]T, error) {
	entry, err := lookupData(data, key)
	if err != nil {
		return nil, err
	}
	result, ok := entry.Value().([]T)
	if !ok {
		return nil, log.CreateError(errors.ErrKeyTypeMismatch, "key {key} value has unexpected type, expected []string: '{value}'",
			log.String("key", string(key)), log.Any("value", entry.Value()))
	}
	return slices.Clone(result), nil
}

func dataInt[T ~int](data entityData, key types.EntityKeyInt) (T, error) {
	result, err := lookupData(data, key)
	if err != nil {
		return 0, err
	}
	value, ok := result.Value().(int)
	if !ok {
		return 0, log.CreateError(errors.ErrKeyTypeMismatch, "key {key} value has unexpected type, expected int: '{value}'",
			log.String("key", string(key)), log.Any("value", result.Value()))
	}
	return T(value), nil
}

func optionalDataString[T ~string](data entityData, key types.EntityKeyString) (T, error) {
	entry, ok := data[key]
	if !ok {
		return "", fmt.Errorf("missing key")
	}
	value, ok := entry.Value().(string)
	if !ok {
		return "", fmt.Errorf("unexpected type")
	}
	return T(value), nil
}

func optionalDataBool[T ~bool](data entityData, key types.EntityKeyBool) (T, error) {
	entry, ok := data[key]
	if !ok {
		return false, fmt.Errorf("missing key")
	}
	value, ok := entry.Value().(bool)
	if !ok {
		return false, fmt.Errorf("unexpected type")
	}
	return T(value), nil
}

// --- internal mutator helpers for EntityBuilder ---

func cloneData(data entityData) entityData {
	if data == nil {
		return entityData{}
	}
	return maps.Clone(data)
}

func dataWithString[T ~string](data entityData, key types.EntityKeyString, value T) entityData {
	data = cloneData(data)
	data[key] = types.NewValueString(string(value))
	return data
}

func dataWithoutString[T ~string](data entityData, key types.EntityKeyString) entityData {
	data = cloneData(data)
	delete(data, key)
	return data
}

func dataWithBool[T ~bool](data entityData, key types.EntityKeyBool, value T) entityData {
	data = cloneData(data)
	data[key] = types.NewValueBool(bool(value))
	return data
}

func dataWithoutBool[T ~bool](data entityData, key types.EntityKeyBool) entityData {
	data = cloneData(data)
	delete(data, key)
	return data
}

func dataWithSlice[T ~string](data entityData, key types.EntityKeySlice[T], value []T) entityData {
	data = cloneData(data)
	data[key] = types.NewValueSlice(value)
	return data
}

func dataWithoutSlice[T ~string](data entityData, key types.EntityKeySlice[T]) entityData {
	data = cloneData(data)
	delete(data, key)
	return data
}

func dataWithInt[T ~int](data entityData, key types.EntityKeyInt, value T) entityData {
	data = cloneData(data)
	data[key] = types.NewValueInt(int(value))
	return data
}
