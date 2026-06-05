package types

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

type EntityKey interface {
	String() string
	CanCompare() bool
}

type EntityKeyString string

var _ EntityKey = EntityKeyString("")

func (a EntityKeyString) String() string {
	return string(a)
}

func (EntityKeyString) CanCompare() bool {
	return true
}

type EntityKeyBool string

var _ EntityKey = EntityKeyBool("")

func (a EntityKeyBool) String() string {
	return string(a)
}

func (EntityKeyBool) CanCompare() bool {
	return true
}

type EntityKeySlice[T any] string

var _ EntityKey = EntityKeySlice[string]("")

func (a EntityKeySlice[T]) String() string {
	return string(a)
}

func (EntityKeySlice[T]) CanCompare() bool {
	return false
}

type EntityKeyInt string

var _ EntityKey = EntityKeyInt("")

func (a EntityKeyInt) String() string {
	return string(a)
}

func (EntityKeyInt) CanCompare() bool {
	return true
}

type EntityKeyUnstructured string

var _ EntityKey = EntityKeyUnstructured("")

func (a EntityKeyUnstructured) String() string {
	return string(a)
}

func (EntityKeyUnstructured) CanCompare() bool {
	return false
}

const (
	KeyApiVersion     EntityKeyString                = "apiVersion"
	KeyAppId          EntityKeyString                = "appId"
	KeyAppIds         EntityKeySlice[AppId]          = "appIds"
	KeyAppNamespace   EntityKeyString                = "appNamespace"
	KeyAppOwned       EntityKeyBool                  = "appOwned"
	KeyBuiltIn        EntityKeyBool                  = "builtIn"
	KeyClusterEntity  EntityKeyUnstructured          = "clusterEntity"
	KeyDryRunEntity   EntityKeyUnstructured          = "dryRunEntity"
	KeyEntity         EntityKeyUnstructured          = "entity"
	KeyGroup          EntityKeyString                = "group"
	KeyGVK            EntityKeyString                = "gvk"
	KeyGVKN           EntityKeyString                = "gvkn"
	KeyGVR            EntityKeyString                = "gvr"
	KeyId             EntityKeyString                = "id"
	KeyKind           EntityKeyString                = "kind"
	KeyLeftEntity     EntityKeyUnstructured          = "leftEntity"
	KeyName           EntityKeyString                = "name"
	KeyNamespace      EntityKeyString                = "ns"
	KeyNamespaced     EntityKeyBool                  = "namespaced"
	KeyResource       EntityKeyString                = "resource"
	KeyRightEntity    EntityKeyUnstructured          = "rightEntity"
	KeySelected       EntityKeyBool                  = "selected"
	KeyTemplateEntity EntityKeyUnstructured          = "templateEntity"
	KeyTemplateIndex  EntityKeyInt                   = "templateIndex"
	KeyTemplatePath   EntityKeyString                = "templatePath"
	KeyRepoPath       EntityKeyString                = "repoPath"
	KeyAbsPath        EntityKeyString                = "absPath"
	KeyVerbs          EntityKeySlice[KubernetesVerb] = "verbs"
	KeyVersion        EntityKeyString                = "version"
)

func EntityKeys() sets.Set[EntityKey] {
	return sets.New[EntityKey](
		KeyApiVersion,
		KeyAppId,
		KeyAppIds,
		KeyAppNamespace,
		KeyAppOwned,
		KeyBuiltIn,
		KeyClusterEntity,
		KeyDryRunEntity,
		KeyEntity,
		KeyGroup,
		KeyGVK,
		KeyGVKN,
		KeyGVR,
		KeyId,
		KeyKind,
		KeyLeftEntity,
		KeyName,
		KeyNamespace,
		KeyNamespaced,
		KeyResource,
		KeyRightEntity,
		KeySelected,
		KeyTemplateEntity,
		KeyTemplateIndex,
		KeyTemplatePath,
		KeyRepoPath,
		KeyAbsPath,
		KeyVerbs,
		KeyVersion,
	)
}
