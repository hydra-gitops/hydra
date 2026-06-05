package entity

import (
	"maps"
	"slices"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// EntityBuilder is a mutable builder for Entity.
// All setter methods return a new EntityBuilder (copy-on-write).
type EntityBuilder struct {
	entityBase
}

var _ IdEntity = EntityBuilder{}

func NewEntityBuilder() EntityBuilder {
	return EntityBuilder{entityBase: entityBase{data: entityData{}}}
}

func (b EntityBuilder) Build() (Entity, error) {
	id, err := computeId(b.data)
	if err != nil {
		return Entity{}, err
	}
	return Entity{
		entityBase: entityBase{data: maps.Clone(b.data)},
		id:         id,
		cache:      buildEntityCache(b.data, id),
	}, nil
}

func (b EntityBuilder) String() string {
	return dataToString(b.data)
}

func newBuilder(data entityData) EntityBuilder {
	return EntityBuilder{entityBase: entityBase{data: data}}
}

// --- Id ---

func (b EntityBuilder) Id() (types.Id, error) {
	return computeId(b.data)
}

func (b EntityBuilder) GVKNString() (string, error) {
	id, err := b.Id()
	if err != nil {
		return "", err
	}
	return id.GVKNString()
}

// --- Setters ---

func (b EntityBuilder) WithApiVersion(apiVersion types.ApiVersion) EntityBuilder {
	return b.WithGroup(apiVersion.Group).WithVersion(apiVersion.Version)
}

func (b EntityBuilder) WithGroup(group types.Group) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyGroup, group))
}

func (b EntityBuilder) WithVersion(version types.Version) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyVersion, version))
}

func (b EntityBuilder) WithResource(resource types.Resource) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyResource, resource))
}

func (b EntityBuilder) WithGVR(gvr types.GVR) EntityBuilder {
	return b.
		WithGroup(types.Group(gvr.Group)).
		WithVersion(types.Version(gvr.Version)).
		WithResource(types.Resource(gvr.Resource))
}

func (b EntityBuilder) WithGVK(gvk types.GVK) EntityBuilder {
	return b.
		WithGroup(types.Group(gvk.Group)).
		WithVersion(types.Version(gvk.Version)).
		WithKind(types.Kind(gvk.Kind))
}

func (b EntityBuilder) WithKind(kind types.Kind) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyKind, kind))
}

func (b EntityBuilder) WithNamespace(namespace types.Namespace) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyNamespace, namespace))
}

func (b EntityBuilder) WithAppNamespace(appNamespace types.AppNamespace) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyAppNamespace, appNamespace))
}

func (b EntityBuilder) WithoutAppNamespace() EntityBuilder {
	return newBuilder(dataWithoutString[types.AppNamespace](b.data, types.KeyAppNamespace))
}

func (b EntityBuilder) WithNamespaced(namespaced types.Namespaced) EntityBuilder {
	return newBuilder(dataWithBool(b.data, types.KeyNamespaced, namespaced))
}

func (b EntityBuilder) WithName(name types.Name) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyName, name))
}

func (b EntityBuilder) WithVerbs(verbs []types.KubernetesVerb) EntityBuilder {
	return newBuilder(dataWithSlice(b.data, types.KeyVerbs, verbs))
}

func (b EntityBuilder) WithoutVerbs() EntityBuilder {
	return newBuilder(dataWithoutSlice(b.data, types.KeyVerbs))
}

func (b EntityBuilder) WithSelected() EntityBuilder {
	return newBuilder(dataWithBool(b.data, types.KeySelected, types.SelectedYes))
}

func (b EntityBuilder) WithoutSelected() EntityBuilder {
	return newBuilder(dataWithoutBool[types.Selected](b.data, types.KeySelected))
}

func (b EntityBuilder) WithAppOwned() EntityBuilder {
	return newBuilder(dataWithBool(b.data, types.KeyAppOwned, true))
}

func (b EntityBuilder) WithoutAppOwned() EntityBuilder {
	return newBuilder(dataWithoutBool[bool](b.data, types.KeyAppOwned))
}

func (b EntityBuilder) WithBuiltIn() EntityBuilder {
	return newBuilder(dataWithBool(b.data, types.KeyBuiltIn, true))
}

func (b EntityBuilder) WithoutBuiltIn() EntityBuilder {
	return newBuilder(dataWithoutBool[bool](b.data, types.KeyBuiltIn))
}

func (b EntityBuilder) WithAppIds(appIds []types.AppId) EntityBuilder {
	data := dataWithSlice(b.data, types.KeyAppIds, appIds)
	if len(appIds) > 0 {
		data = dataWithBool(data, types.KeyAppOwned, true)
	}
	return newBuilder(data)
}

// WithAppId sets the primary owning Hydra app id for template entities (singular metadata).
func (b EntityBuilder) WithAppId(appId types.AppId) EntityBuilder {
	data := dataWithString(b.data, types.KeyAppId, appId)
	data = dataWithBool(data, types.KeyAppOwned, true)
	return newBuilder(data)
}

func (b EntityBuilder) WithoutAppId() EntityBuilder {
	return newBuilder(dataWithoutString[types.AppId](b.data, types.KeyAppId))
}

func (b EntityBuilder) WithoutAppIds() EntityBuilder {
	return newBuilder(dataWithoutSlice(b.data, types.KeyAppIds))
}

func (b EntityBuilder) AddAppId(appId types.AppId) (EntityBuilder, error) {
	appIds, err := b.AppIds()
	if err != nil {
		if errors.ErrKeyNotFound.MatchesError(err) {
			appIds = nil
		} else {
			return EntityBuilder{}, err
		}
	}
	if slices.Contains(appIds, appId) {
		return b, nil
	}
	appIds = append(appIds, appId)
	return b.WithAppIds(appIds), nil
}

func (b EntityBuilder) WithUnstructured(
	key types.EntityKeyUnstructured,
	item unstructured.Unstructured,
) EntityBuilder {
	data := cloneData(b.data)
	data[key] = types.NewValueUnstructured(item)
	return newBuilder(data)
}

func (b EntityBuilder) WithoutUnstructured(
	key types.EntityKeyUnstructured,
) EntityBuilder {
	data := cloneData(b.data)
	delete(data, key)
	return newBuilder(data)
}

func (b EntityBuilder) WithTemplatePath(templatePath types.TemplatePath) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyTemplatePath, templatePath))
}

func (b EntityBuilder) WithRepoPath(repoPath types.RepoPath) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyRepoPath, repoPath))
}

func (b EntityBuilder) WithAbsPath(absPath types.AbsPath) EntityBuilder {
	return newBuilder(dataWithString(b.data, types.KeyAbsPath, absPath))
}

func (b EntityBuilder) WithTemplateIndex(templateIndex types.TemplateIndex) EntityBuilder {
	return newBuilder(dataWithInt(b.data, types.KeyTemplateIndex, templateIndex))
}

// MergeKeysWithoutUnstructured copies all non-unstructured keys from other into this builder.
func (b EntityBuilder) MergeKeysWithoutUnstructured(other IdEntity) EntityBuilder {
	data := cloneData(b.data)
	otherData := idEntityData(other)
	for k, v := range otherData {
		if _, ok := k.(types.EntityKeyUnstructured); ok {
			continue
		}
		data[k] = v
	}
	return newBuilder(data)
}

// idEntityData extracts the internal entityData from an IdEntity.
// This is package-private and works for both Entity and EntityBuilder.
func idEntityData(e IdEntity) entityData {
	switch v := e.(type) {
	case Entity:
		return v.data
	case EntityBuilder:
		return v.data
	default:
		return nil
	}
}
