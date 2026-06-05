package cel

import (
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"github.com/google/cel-go/common/types/traits"
	hydraTypes "hydra-gitops.org/hydra/hydra-go/core/types"
)

type utilSupport struct {
}

var _ cel.SingletonLibrary = (*utilSupport)(nil)

func (support *utilSupport) LibraryName() string {
	return "hydra.util-support"
}

func (support *utilSupport) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function(
			"ordinalRange",
			cel.Overload(
				"ordinalRange_int_int",
				[]*cel.Type{cel.IntType, cel.IntType},
				cel.ListType(cel.IntType),
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					startVal, ok := args[0].(types.Int)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					countVal, ok := args[1].(types.Int)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					count := int(countVal)
					if count <= 0 {
						return types.NewDynamicList(types.DefaultTypeAdapter, []any{})
					}
					start := int(startVal)
					elems := make([]any, 0, count)
					for i := 0; i < count; i++ {
						elems = append(elems, int64(start+i))
					}
					return types.NewDynamicList(types.DefaultTypeAdapter, elems)
				}),
			),
		),
		cel.Function(
			"vctPvcOrdinalName",
			cel.Overload(
				"vctPvcOrdinalName_string_string_string",
				[]*cel.Type{cel.StringType, cel.StringType, cel.StringType},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					full, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					vct, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					sts, ok := args[2].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[2])
					}
					vctStr := string(vct)
					stsStr := string(sts)
					if vctStr == "" || stsStr == "" {
						return types.Bool(false)
					}
					prefix := vctStr + "-" + stsStr + "-"
					fullStr := string(full)
					if !strings.HasPrefix(fullStr, prefix) {
						return types.Bool(false)
					}
					suffix := fullStr[len(prefix):]
					if suffix == "" {
						return types.Bool(false)
					}
					for _, r := range suffix {
						if r < '0' || r > '9' {
							return types.Bool(false)
						}
					}
					return types.Bool(true)
				}),
			),
		),
		cel.Function(
			"getOrEmpty",
			cel.MemberOverload(
				"getOrEmpty_map_string",
				[]*cel.Type{
					cel.MapType(cel.StringType, cel.StringType),
					cel.StringType,
				},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					m, ok := args[0].(traits.Mapper)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}

					key, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}

					if val, found := m.Find(key); found {
						return val
					}

					return types.String("")
				}),
			),
		),
		cel.Function(
			"objectsetRioOwnerGvkToHydraGvk",
			cel.Overload(
				"objectsetRioOwnerGvkToHydraGvk_string",
				[]*cel.Type{cel.StringType},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					s, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					return types.String(ObjectsetRioOwnerGVKToHydraGvk(string(s)))
				}),
			),
		),
		cel.Function(
			"id",
			cel.Overload(
				"id_string_string_string",
				[]*cel.Type{
					cel.StringType,                   // gvk
					cel.NullableType(cel.StringType), // ns (can be null or empty string for cluster-scoped)
					cel.StringType,                   // name
				},
				CelRefEndpointType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					gvk, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}

					var ns types.String
					if args[1] != types.NullValue {
						nsVal, ok := args[1].(types.String)
						if !ok {
							return types.MaybeNoSuchOverloadErr(args[1])
						}
						ns = nsVal
					}

					name, ok := args[2].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[2])
					}

					// Format: gvk/namespace/name (namespace can be empty for cluster-scoped)
					return CelRefEndpoint{
						RefEndpoint: hydraTypes.RefEndpoint{
							Type:  "id",
							Value: string(gvk) + "/" + string(ns) + "/" + string(name),
						},
					}
				}),
			),
		),
		cel.Function(
			"idString",
			cel.Overload(
				"idString_string_string_string",
				[]*cel.Type{
					cel.StringType,
					cel.NullableType(cel.StringType),
					cel.StringType,
				},
				cel.StringType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					gvk, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					var ns types.String
					if args[1] != types.NullValue {
						nsVal, ok := args[1].(types.String)
						if !ok {
							return types.MaybeNoSuchOverloadErr(args[1])
						}
						ns = nsVal
					}
					name, ok := args[2].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[2])
					}
					return types.String(string(gvk) + "/" + string(ns) + "/" + string(name))
				}),
			),
		),
		cel.Function(
			"ref",
			cel.Overload(
				"ref_string_string",
				[]*cel.Type{
					cel.StringType, // type
					cel.StringType, // value
				},
				CelRefEndpointType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					refType, ok := args[0].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}

					refValue, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}

					return CelRefEndpoint{
						RefEndpoint: hydraTypes.RefEndpoint{
							Type:  hydraTypes.RefEndpointType(refType),
							Value: string(refValue),
						},
					}
				}),
			),
		),
		cel.Function(
			"incoming",
			cel.MemberOverload(
				"ref_incoming_refendpoint",
				[]*cel.Type{
					CelRefType,         // receiver (CelRef builder)
					CelRefEndpointType, // endpoint from id() or ref()
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					endpoint, ok := args[1].(CelRefEndpoint)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					// Add incoming RefDefinition to builder
					newRefs := append(builder.Refs, hydraTypes.RefDefinition{
						Type:      hydraTypes.RefTypeDirect,
						Direction: hydraTypes.RefDirectionIncoming,
						Endpoint:  endpoint.RefEndpoint,
					})
					return CelRef{Refs: newRefs}
				}),
			),
		),
		cel.Function(
			"outgoing",
			cel.MemberOverload(
				"ref_outgoing_refendpoint",
				[]*cel.Type{
					CelRefType,         // receiver (CelRef builder)
					CelRefEndpointType, // endpoint from id() or ref()
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					endpoint, ok := args[1].(CelRefEndpoint)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					// Add outgoing RefDefinition to builder
					newRefs := append(builder.Refs, hydraTypes.RefDefinition{
						Type:      hydraTypes.RefTypeDirect,
						Direction: hydraTypes.RefDirectionOutgoing,
						Endpoint:  endpoint.RefEndpoint,
					})
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// refBuilder() function to create a new empty CelRef builder
		cel.Function(
			"refBuilder",
			cel.Overload(
				"refBuilder_void",
				[]*cel.Type{},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					return CelRef{Refs: []hydraTypes.RefDefinition{}}
				}),
			),
		),
		// label() function to set label on the last RefDefinition (only allowed for outgoing)
		cel.Function(
			"label",
			cel.MemberOverload(
				"ref_label_string",
				[]*cel.Type{
					CelRefType,     // receiver (CelRef builder)
					cel.StringType, // label string
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					labelStr, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("label() called on empty ref builder")
					}
					lastRef := builder.Refs[len(builder.Refs)-1]
					if lastRef.Direction != hydraTypes.RefDirectionOutgoing {
						return types.NewErr("label() is only allowed on outgoing references, not on %s", lastRef.Direction)
					}
					// Copy refs and set label on last entry
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Label = string(labelStr)
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// tag() function to add a tag to the last RefDefinition
		cel.Function(
			"tag",
			cel.MemberOverload(
				"ref_tag_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					tagStr, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("tag() called on empty ref builder")
					}
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Tags = append(
						append([]string{}, newRefs[len(newRefs)-1].Tags...),
						string(tagStr),
					)
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// desc() function to set a description on the last RefDefinition
		cel.Function(
			"desc",
			cel.MemberOverload(
				"ref_desc_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					descStr, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("desc() called on empty ref builder")
					}
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Desc = string(descStr)
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// attribute() function to add a structured attribute to the last RefDefinition
		cel.Function(
			"attribute",
			cel.MemberOverload(
				"ref_attribute_string_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
					cel.StringType,
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					attrType, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					attrValue, ok := args[2].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[2])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("attribute() called on empty ref builder")
					}
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Attributes = append(
						append([]hydraTypes.RefAttribute{}, newRefs[len(newRefs)-1].Attributes...),
						hydraTypes.RefAttribute{Type: string(attrType), Value: string(attrValue)},
					)
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// key() helper to add a key attribute to the last RefDefinition
		cel.Function(
			"key",
			cel.MemberOverload(
				"ref_key_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					keyName, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("key() called on empty ref builder")
					}
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Attributes = append(
						append([]hydraTypes.RefAttribute{}, newRefs[len(newRefs)-1].Attributes...),
						hydraTypes.RefAttribute{Type: "key", Value: string(keyName)},
					)
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// reverse() function to mark the last RefDefinition as reversed
		cel.Function(
			"reverse",
			cel.MemberOverload(
				"ref_reverse",
				[]*cel.Type{
					CelRefType, // receiver (CelRef builder)
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("reverse() called on empty ref builder")
					}
					// Copy refs and set reverse on last entry
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Reverse = true
					return CelRef{Refs: newRefs}
				}),
			),
		),
		// refType() sets types.RefDefinition.Type on the last ref (outgoing or incoming).
		cel.Function(
			"refType",
			cel.MemberOverload(
				"ref_refType_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
				},
				CelRefType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					rt, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					if len(builder.Refs) == 0 {
						return types.NewErr("refType() called on empty ref builder")
					}
					newRefs := make([]hydraTypes.RefDefinition, len(builder.Refs))
					copy(newRefs, builder.Refs)
					newRefs[len(newRefs)-1].Type = hydraTypes.RefType(string(rt))
					return CelRef{Refs: newRefs}
				}),
			),
		),
		cel.Function(
			"hasEndpoint",
			cel.MemberOverload(
				"ref_hasEndpoint_string",
				[]*cel.Type{
					CelRefType,
					cel.StringType,
				},
				cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					builder, ok := args[0].(CelRef)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[0])
					}
					endpointID, ok := args[1].(types.String)
					if !ok {
						return types.MaybeNoSuchOverloadErr(args[1])
					}
					for _, rd := range builder.Refs {
						if rd.Endpoint.Type == hydraTypes.RefEndpointTypeId && rd.Endpoint.Value == string(endpointID) {
							return types.True
						}
					}
					return types.False
				}),
			),
		),
	}
}

func (*utilSupport) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func UtilSupport() cel.EnvOption {
	return cel.Lib(&utilSupport{})
}
