package types

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

type Group string
type Version string
type Resource string
type Kind string
type Uid string
type Namespace string
type AppNamespace string
type Name string
type Labels map[string]string
type Annotations map[string]string

type GVKString string
type GVRString string
type ApiVersionString string

func (gvk GVKString) CelPredicate() CelPredicate {
	return CelPredicate(`gvk == "` + string(gvk) + `"`)
}

// Components parses the GVKString and returns Group, Version, Kind
// Format: "group/version/kind" or "version/kind" (if group is empty)
func (gvk GVKString) Components() (Group, Version, Kind, error) {
	parts := strings.Split(string(gvk), "/")
	switch len(parts) {
	case 2:
		return "", Version(parts[0]), Kind(parts[1]), nil
	case 3:
		return Group(parts[0]), Version(parts[1]), Kind(parts[2]), nil
	default:
		return "", "", "", fmt.Errorf("invalid GVKString format: %q (expected 2 or 3 parts)", gvk)
	}
}

type ApiVersion struct {
	Group   Group
	Version Version
}

func NewApiVersion(group Group, version Version) ApiVersion {
	return ApiVersion{Group: group, Version: version}
}

func (a ApiVersion) K8s() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   string(a.Group),
		Version: string(a.Version),
	}
}

func (a ApiVersion) ApiVersionString() ApiVersionString {
	return ApiVersionString(a.String())
}

func (a ApiVersion) String() string {
	if a.Group == "" {
		return string(a.Version)
	}
	return fmt.Sprintf("%s/%s", a.Group, a.Version)
}

// group/version/kind/namespace/name
type Id string

// Components parses the Id and returns Group, Version, Kind, Namespace, Name
// Format: "group/version/kind/namespace/name" or "version/kind/namespace/name" (if group is empty)
func (id Id) Components() (Group, Version, Kind, Namespace, Name, error) {
	parts := strings.Split(string(id), "/")
	switch len(parts) {
	case 1:
		// empty id
		return "", "", "", "", "", nil
	case 4:
		// version/kind/namespace/name
		return "", Version(parts[0]), Kind(parts[1]), Namespace(parts[2]), Name(parts[3]), nil
	case 5:
		// group/version/kind/namespace/name
		return Group(parts[0]), Version(parts[1]), Kind(parts[2]), Namespace(parts[3]), Name(parts[4]), nil
	default:
		return "", "", "", "", "", fmt.Errorf("invalid Id format: %q (expected 4 or 5 parts)", id)
	}
}

// GVKNString returns gvk for cluster-scoped ids (empty namespace segment) and gvk/namespace
// for namespaced ids. It matches the interactive filter field GVKN and the CEL entity variable gvkn.
func (id Id) GVKNString() (string, error) {
	group, version, kind, namespace, _, err := id.Components()
	if err != nil {
		return "", err
	}
	var gvk string
	if group == "" {
		gvk = fmt.Sprintf("%s/%s", version, kind)
	} else {
		gvk = fmt.Sprintf("%s/%s/%s", group, version, kind)
	}
	if namespace == "" {
		return gvk, nil
	}
	return gvk + "/" + string(namespace), nil
}

// NewId builds an entity Id from GVK and namespace/name. Core group uses the 4-part form.
func NewId(group Group, version Version, kind Kind, namespace Namespace, name Name) Id {
	if group == "" {
		return Id(fmt.Sprintf("%s/%s/%s/%s", version, kind, namespace, name))
	}
	return Id(fmt.Sprintf("%s/%s/%s/%s/%s", group, version, kind, namespace, name))
}

func (id Id) Group() (Group, error) {
	group, _, _, _, _, err := id.Components()
	return group, err
}

func (id Id) Version() (Version, error) {
	_, version, _, _, _, err := id.Components()
	return version, err
}

func (id Id) Kind() (Kind, error) {
	_, _, kind, _, _, err := id.Components()
	return kind, err
}

func (id Id) Namespace() (Namespace, error) {
	_, _, _, namespace, _, err := id.Components()
	return namespace, err
}

func (id Id) Name() (Name, error) {
	_, _, _, _, name, err := id.Components()
	return name, err
}

// Kubernetes GroupVersionResource
type GVR struct {
	ApiVersion
	Resource Resource
}

func NewGVR(group Group, version Version, resource Resource) GVR {
	return NewGVRApiVersion(NewApiVersion(group, version), resource)
}

func NewGVRApiVersion(apiVersion ApiVersion, resource Resource) GVR {
	return GVR{ApiVersion: apiVersion, Resource: resource}
}

func (g GVR) K8s() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    string(g.Group),
		Version:  string(g.Version),
		Resource: string(g.Resource),
	}
}

func (g GVR) GVRString() GVRString {
	return GVRString(g.String())
}

func (g GVR) String() string {
	if g.Group == "" {
		return fmt.Sprintf("%s/%s", g.Version, g.Resource)
	}
	return fmt.Sprintf("%s/%s/%s", g.Group, g.Version, g.Resource)
}

type GVRNString string

func NewGVRNString(gvr GVR, namespace Namespace) GVRNString {
	return GVRNString(fmt.Sprintf("%s/%s", gvr.String(), string(namespace)))
}

func (g GVRNString) String() string {
	return string(g)
}

// Kubernetes GroupVersionKind
type GVK struct {
	ApiVersion
	Kind Kind
}

func NewGVK(group Group, version Version, kind Kind) GVK {
	return NewGVKApiVersion(NewApiVersion(group, version), kind)
}

func NewGVKApiVersion(apiVersion ApiVersion, kind Kind) GVK {
	return GVK{ApiVersion: apiVersion, Kind: kind}
}

func NewGVKFromK8s(k8s schema.GroupVersionKind) GVK {
	return NewGVK(
		Group(k8s.Group),
		Version(k8s.Version),
		Kind(k8s.Kind),
	)
}

func (g GVK) K8s() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   string(g.Group),
		Version: string(g.Version),
		Kind:    string(g.Kind),
	}
}

func (g GVK) GVKString() GVKString {
	return GVKString(g.String())
}

func (g GVK) String() string {
	if g.Group == "" {
		return fmt.Sprintf("%s/%s", g.Version, g.Kind)
	}
	return fmt.Sprintf("%s/%s/%s", g.Group, g.Version, g.Kind)
}

func (g GVK) GVK() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   string(g.Group),
		Version: string(g.Version),
		Kind:    string(g.Kind),
	}
}

// ParseApiVersion parses a group version string and returns Group and Version
func ParseApiVersion(gvString string) (ApiVersion, error) {
	gv, err := schema.ParseGroupVersion(gvString)
	if err != nil {
		return ApiVersion{}, err
	}
	return NewApiVersion(Group(gv.Group), Version(gv.Version)), nil
}

type TemplatePath string
type TemplateIndex int

type KubernetesVersion string
type KubernetesVersionOrFallback string

type KubernetesVerb string

const (
	VerbGet         KubernetesVerb = "get"
	VerbList        KubernetesVerb = "list"
	VerbWatch       KubernetesVerb = "watch"
	VerbCreate      KubernetesVerb = "create"
	VerbUpdate      KubernetesVerb = "update"
	VerbPatch       KubernetesVerb = "patch"
	VerbDelete      KubernetesVerb = "delete"
	VerbProxy       KubernetesVerb = "proxy"
	VerbImpersonate KubernetesVerb = "impersonate"
	VerbEscalate    KubernetesVerb = "escalate"
	VerbBind        KubernetesVerb = "bind"
)
