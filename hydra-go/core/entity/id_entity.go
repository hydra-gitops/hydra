package entity

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
)

type IdEntity interface {
	Id() (types.Id, error)
	Group() (types.Group, error)
	Version() (types.Version, error)
	Resource() (types.Resource, error)
	Kind() (types.Kind, error)
	Namespace() (types.Namespace, error)
	HasNamespace() bool
	AppNamespace() (types.AppNamespace, error)
	Namespaced() (types.Namespaced, error)
	Name() (types.Name, error)
	Verbs() ([]types.KubernetesVerb, error)
	VerbsContains(verb types.KubernetesVerb) (bool, error)
	Selected() types.Selected
	AppId() (types.AppId, error)
	AppIds() ([]types.AppId, error)
	Unstructured(key types.EntityKeyUnstructured) (unstructured.Unstructured, bool)
	UnstructuredOrError(key types.EntityKeyUnstructured) (unstructured.Unstructured, error)
	Uid(key types.EntityKeyUnstructured) (types.Uid, bool)
	TemplatePath() (types.TemplatePath, error)
	RepoPath() (types.RepoPath, error)
	AbsPath() (types.AbsPath, error)
	TemplateIndex() (types.TemplateIndex, error)
	OwnerUids(key types.EntityKeyUnstructured) sets.Set[types.Uid]
	HasKey(key types.EntityKey) bool
	GVR() (types.GVR, error)
	GVK() (types.GVK, error)
	GVKString() (types.GVKString, error)
	GVKNString() (string, error)
	ApiVersion() (types.ApiVersion, error)
	ApiVersionString() (string, error)
	GVRString() (types.GVRString, error)
	IdUnknownNamespace() (string, error)
	String() string
}
