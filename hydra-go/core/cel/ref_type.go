package cel

import (
	"fmt"
	"reflect"
	"slices"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	hydraTypes "hydra-gitops.org/hydra/hydra-go/core/types"
)

// CelRefEndpoint wraps types.RefEndpoint to implement ref.Val
type CelRefEndpoint struct {
	RefEndpoint hydraTypes.RefEndpoint
}

var (
	CelRefEndpointType = cel.OpaqueType("hydra.RefEndpoint")
)

// ConvertToNative implements ref.Val
func (re CelRefEndpoint) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if typeDesc == reflect.TypeFor[CelRefEndpoint]() {
		return re, nil
	}
	if typeDesc == reflect.TypeFor[hydraTypes.RefEndpoint]() {
		return re.RefEndpoint, nil
	}
	if typeDesc == reflect.TypeFor[map[string]any]() {
		return map[string]any{
			"type":  string(re.RefEndpoint.Type),
			"value": re.RefEndpoint.Value,
		}, nil
	}
	return nil, fmt.Errorf("cannot convert CelRefEndpoint to %v", typeDesc)
}

// ConvertToType implements ref.Val
func (re CelRefEndpoint) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.TypeType:
		return CelRefEndpointType
	case types.StringType:
		return types.String(re.RefEndpoint.Value)
	}
	return types.NewErr("type conversion error from 'hydra.RefEndpoint' to '%s'", typeVal)
}

// Equal implements ref.Val
func (re CelRefEndpoint) Equal(other ref.Val) ref.Val {
	otherRe, ok := other.(CelRefEndpoint)
	if !ok {
		return types.False
	}
	return types.Bool(re.RefEndpoint.Type == otherRe.RefEndpoint.Type && re.RefEndpoint.Value == otherRe.RefEndpoint.Value)
}

// Type implements ref.Val
func (re CelRefEndpoint) Type() ref.Type {
	return CelRefEndpointType
}

// Value implements ref.Val
func (re CelRefEndpoint) Value() any {
	return re.RefEndpoint
}

var _ ref.Val = CelRefEndpoint{}

// CelRef is a builder that collects RefDefinitions via incoming()/outgoing() calls
type CelRef struct {
	Refs []hydraTypes.RefDefinition
}

var (
	CelRefType = cel.OpaqueType("hydra.Ref")
)

// ConvertToNative implements ref.Val
func (r CelRef) ConvertToNative(typeDesc reflect.Type) (any, error) {
	if typeDesc == reflect.TypeFor[CelRef]() {
		return r, nil
	}
	if typeDesc == reflect.TypeFor[[]hydraTypes.RefDefinition]() {
		return r.Refs, nil
	}
	return nil, fmt.Errorf("cannot convert CelRef to %v", typeDesc)
}

// ConvertToType implements ref.Val
func (r CelRef) ConvertToType(typeVal ref.Type) ref.Val {
	switch typeVal {
	case types.TypeType:
		return CelRefType
	}
	return types.NewErr("type conversion error from 'hydra.Ref' to '%s'", typeVal)
}

// Equal implements ref.Val
func (r CelRef) Equal(other ref.Val) ref.Val {
	otherRef, ok := other.(CelRef)
	if !ok {
		return types.False
	}
	if len(r.Refs) != len(otherRef.Refs) {
		return types.False
	}
	for i, rd := range r.Refs {
		o := otherRef.Refs[i]
		if rd.Owner != o.Owner || rd.Type != o.Type || rd.Direction != o.Direction ||
			rd.Endpoint != o.Endpoint || rd.Label != o.Label || rd.Desc != o.Desc ||
			rd.Reverse != o.Reverse || !slices.Equal(rd.Tags, o.Tags) {
			return types.False
		}
	}
	return types.True
}

// Type implements ref.Val
func (r CelRef) Type() ref.Type {
	return CelRefType
}

// Value implements ref.Val
func (r CelRef) Value() any {
	return r.Refs
}

var _ ref.Val = CelRef{}
