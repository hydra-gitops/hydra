package entity

import (
	"fmt"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// --- Composed getters on entityBase (shared by Entity and EntityBuilder) ---

func (e entityBase) GVR() (types.GVR, error) {
	group, err := e.Group()
	if err != nil {
		return types.GVR{}, err
	}
	version, err := e.Version()
	if err != nil {
		return types.GVR{}, err
	}
	resource, err := e.Resource()
	if err != nil {
		return types.GVR{}, err
	}
	return types.NewGVR(group, version, resource), nil
}

func (e entityBase) GVK() (types.GVK, error) {
	group, err := e.Group()
	if err != nil {
		return types.GVK{}, err
	}
	version, err := e.Version()
	if err != nil {
		return types.GVK{}, err
	}
	kind, err := e.Kind()
	if err != nil {
		return types.GVK{}, err
	}
	return types.NewGVK(group, version, kind), nil
}

func (e entityBase) GVKString() (types.GVKString, error) {
	group, err := e.Group()
	if err != nil {
		return "", err
	}
	version, err := e.Version()
	if err != nil {
		return "", err
	}
	kind, err := e.Kind()
	if err != nil {
		return "", err
	}
	gvk := ""
	if group == "" {
		gvk = fmt.Sprintf("%s/%s", version, kind)
	} else {
		gvk = fmt.Sprintf("%s/%s/%s", group, version, kind)
	}
	return types.GVKString(gvk), nil
}

func (e entityBase) ApiVersion() (types.ApiVersion, error) {
	group, err := e.Group()
	if err != nil {
		return types.ApiVersion{}, err
	}
	version, err := e.Version()
	if err != nil {
		return types.ApiVersion{}, err
	}
	return types.NewApiVersion(group, version), nil
}

func (e entityBase) ApiVersionString() (string, error) {
	apiVersion, err := e.ApiVersion()
	if err != nil {
		return "", err
	}
	return apiVersion.String(), nil
}

func (e entityBase) GVRString() (types.GVRString, error) {
	gvr, err := e.GVR()
	if err != nil {
		return "", err
	}
	if gvr.Group == "" {
		return types.GVRString(fmt.Sprintf("%s/%s", gvr.Version, gvr.Resource)), nil
	}
	return types.GVRString(fmt.Sprintf("%s/%s/%s", gvr.Group, gvr.Version, gvr.Resource)), nil
}

func (e entityBase) IdUnknownNamespace() (string, error) {
	gvk, err := e.GVKString()
	if err != nil {
		return "", err
	}
	namespace, err := e.Namespace()
	if err != nil {
		namespace = "<unknown>"
	}
	name, err := e.Name()
	if err != nil {
		return "", err
	}
	appNamespace, err := e.AppNamespace()
	if err != nil {
		return "", err
	}
	id := fmt.Sprintf("%s/%s/%s/%s", gvk, namespace, name, appNamespace)
	return id, nil
}

func buildEntityCache(data entityData, id types.Id) entityCache {
	cache := entityCache{}
	group, version, kind, _, _, err := id.Components()
	if err == nil {
		cache.group = group
		cache.version = version
		cache.kind = kind
		cache.gvk = types.NewGVK(group, version, kind)
		cache.gvkString = cache.gvk.GVKString()
		cache.apiVersion = types.NewApiVersion(group, version)
		cache.apiVersionStr = cache.apiVersion.String()
	}
	if namespace, err := optionalDataString[types.Namespace](data, types.KeyNamespace); err == nil {
		cache.namespace = namespace
		cache.hasNamespace = true
	}
	if name, err := optionalDataString[types.Name](data, types.KeyName); err == nil {
		cache.name = name
		cache.hasName = true
	}
	if cache.hasNamespace && cache.namespace != "" {
		cache.gvknString = string(cache.gvkString) + "/" + string(cache.namespace)
	} else {
		cache.gvknString = string(cache.gvkString)
	}
	if resource, err := optionalDataString[types.Resource](data, types.KeyResource); err == nil {
		cache.resource = resource
		cache.hasResource = true
		cache.gvr = types.NewGVR(cache.group, cache.version, resource)
		cache.gvrString = cache.gvr.GVRString()
	}
	if appNamespace, err := optionalDataString[types.AppNamespace](data, types.KeyAppNamespace); err == nil {
		cache.appNamespace = appNamespace
		cache.hasAppNs = true
	}
	if namespaced, err := optionalDataBool[types.Namespaced](data, types.KeyNamespaced); err == nil {
		cache.namespaced = namespaced
		cache.hasNamespaced = true
	}
	return cache
}

// --- Id computation (used by EntityBuilder.Id and Build) ---

func computeId(data entityData) (types.Id, error) {
	group, err := dataString[types.Group](data, types.KeyGroup)
	if err != nil {
		return "", err
	}
	version, err := dataString[types.Version](data, types.KeyVersion)
	if err != nil {
		return "", err
	}
	kind, err := dataString[types.Kind](data, types.KeyKind)
	if err != nil {
		return "", err
	}

	gvk := ""
	if group == "" {
		gvk = fmt.Sprintf("%s/%s", version, kind)
	} else {
		gvk = fmt.Sprintf("%s/%s/%s", group, version, kind)
	}

	namespace, _ := dataString[types.Namespace](data, types.KeyNamespace)

	// Instance name is absent for API-resource-type entities (e.g. VisitResources
	// callbacks before listing objects). Treat like optional namespace.
	name, _ := dataString[types.Name](data, types.KeyName)

	id := fmt.Sprintf("%s/%s/%s", gvk, namespace, name)
	return types.Id(id), nil
}

// dataToString formats entityData as a string for display purposes.
func dataToString(data entityData) string {
	result := []string{}
	for key := range types.EntityKeys() {
		if v, ok := data[key]; ok {
			result = append(result, fmt.Sprintf("%s=%v", key, v))
		}
	}
	return strings.Join(result, ", ")
}
