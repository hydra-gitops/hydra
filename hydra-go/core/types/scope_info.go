package types

import (
	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
)

type ScopeInfo struct {
	Group      Group
	Version    Version
	Resource   Resource
	Kind       Kind
	Namespaced Namespaced
}

func (info ScopeInfo) GVK() GVK {
	return NewGVK(info.Group, info.Version, info.Kind)
}

func (info ScopeInfo) GVKString() GVKString {
	return info.GVK().GVKString()
}

func (info ScopeInfo) Clone() ScopeInfo {
	return ScopeInfo{
		Group:      info.Group,
		Version:    info.Version,
		Resource:   info.Resource,
		Kind:       info.Kind,
		Namespaced: info.Namespaced,
	}
}

type ScopeInfoMap map[GVKString]ScopeInfo

type GroupKindKey string

func NewGroupKindKey(group Group, kind Kind) GroupKindKey {
	if group == "" {
		return GroupKindKey(string(kind))
	}
	return GroupKindKey(string(group) + "/" + string(kind))
}

// PreferredVersionMap builds one preferred Version per GroupKindKey from ScopeInfoMap keys.
// If two distinct GVK strings map to the same group/kind but different API versions, it returns
// ErrConflictingPreferredApiVersions so callers do not pick a winner by map iteration order.
func PreferredVersionMap(scopeInfoMap ScopeInfoMap) (map[GroupKindKey]Version, error) {
	result := map[GroupKindKey]Version{}
	for gvkStr := range scopeInfoMap {
		group, version, kind, err := gvkStr.Components()
		if err != nil {
			continue
		}
		key := NewGroupKindKey(group, kind)
		if prev, ok := result[key]; ok && prev != version {
			return nil, log.CreateError(errors.ErrConflictingPreferredApiVersions,
				"scope info map lists conflicting API versions for the same group/kind: {groupKind} has {first} and {second} (from GVK keys {firstGvk} and {secondGvk})",
				log.String("groupKind", string(key)),
				log.String("first", string(prev)),
				log.String("second", string(version)),
				log.String("firstGvk", findConflictingGvkString(scopeInfoMap, key, prev)),
				log.String("secondGvk", string(gvkStr)))
		}
		result[key] = version
	}
	return result, nil
}

func findConflictingGvkString(scopeInfoMap ScopeInfoMap, wantKey GroupKindKey, wantVersion Version) string {
	for gvkStr := range scopeInfoMap {
		group, version, kind, err := gvkStr.Components()
		if err != nil {
			continue
		}
		if NewGroupKindKey(group, kind) == wantKey && version == wantVersion {
			return string(gvkStr)
		}
	}
	return ""
}
