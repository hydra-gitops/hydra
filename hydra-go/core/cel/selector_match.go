package cel

import (
	goocel "github.com/google/cel-go/cel"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

func selectorMatchesEntity(selector types.RefSelector, e entity.Entity) bool {
	if selector.IsZero() {
		return true
	}
	id, err := e.Id()
	if err != nil {
		return false
	}
	group, version, kind, namespace, name, err := id.Components()
	if err != nil {
		return false
	}
	return selectorMatchesComponents(selector, group, version, kind, namespace, name)
}

func selectorMatchesMap(selector types.RefSelector, m map[string]any) bool {
	if selector.IsZero() {
		return true
	}
	group, _ := mapString(m, types.KeyGroup.String())
	version, _ := mapString(m, types.KeyVersion.String())
	kind, _ := mapString(m, types.KeyKind.String())
	namespace, _ := mapString(m, types.KeyNamespace.String())
	name, _ := mapString(m, types.KeyName.String())
	return selectorMatchesComponents(
		selector,
		types.Group(group),
		types.Version(version),
		types.Kind(kind),
		types.Namespace(namespace),
		types.Name(name),
	)
}

func selectorMatchesInput(selector types.RefSelector, input any) bool {
	if selector.IsZero() {
		return true
	}
	switch x := input.(type) {
	case map[string]any:
		return selectorMatchesMap(selector, x)
	case *entityActivation:
		return selectorMatchesEntity(selector, x.entity)
	case goocel.Activation:
		// For unknown activation implementations we cannot reliably evaluate selector fields.
		return false
	default:
		return false
	}
}

func selectorMatchesComponents(selector types.RefSelector, group types.Group, version types.Version, kind types.Kind, namespace types.Namespace, name types.Name) bool {
	if selector.Group != "" && selector.Group != group {
		return false
	}
	if selector.Version != "" && selector.Version != version {
		return false
	}
	if selector.Kind != "" && selector.Kind != kind {
		return false
	}
	if selector.Namespace != "" && selector.Namespace != namespace {
		return false
	}
	if selector.Name != "" && selector.Name != name {
		return false
	}
	return true
}

func mapString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok || v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return x, true
	case interface{ Value() any }:
		// Covers cel ref.Val and other wrapper types with Value().
		val := x.Value()
		s, isString := val.(string)
		return s, isString
	default:
		return "", false
	}
}
