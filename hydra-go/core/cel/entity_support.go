package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/functions"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	hydraTypes "hydra-gitops.org/hydra/hydra-go/core/types"
)

type entitySupport struct {
}

var _ cel.SingletonLibrary = (*entitySupport)(nil)

func (ss *entitySupport) LibraryName() string {
	return "hydra.entity-support"
}

func (ss *entitySupport) CompileOptions() []cel.EnvOption {
	options := []cel.EnvOption{
		cel.Function(
			"annotations",
			cel.MemberOverload(
				"entity_annotations",
				[]*cel.Type{celUnstructuredType()},
				cel.MapType(cel.StringType, cel.StringType),
				cel.FunctionBinding(
					functions.FunctionOp(untypedAnnotations),
				),
			),
		),
		cel.Function(
			"labels",
			cel.MemberOverload(
				"entity_labels",
				[]*cel.Type{celUnstructuredType()},
				cel.MapType(cel.StringType, cel.StringType),
				cel.FunctionBinding(
					functions.FunctionOp(untypedLabels),
				),
			),
		),
		cel.Function(
			"gvk",
			cel.Overload(
				"gvk_unstructured",
				[]*cel.Type{celUnstructuredType()},
				cel.StringType,
				cel.FunctionBinding(
					functions.FunctionOp(untypedGVK),
				),
			),
		),
		cel.Function(
			"gvkn",
			cel.Overload(
				"gvkn_unstructured",
				[]*cel.Type{celUnstructuredType()},
				cel.StringType,
				cel.FunctionBinding(
					functions.FunctionOp(untypedGVKN),
				),
			),
		),
	}

	for key, reader := range readers() {
		options = append(options, cel.Variable(key.String(), reader.Type()))
	}

	return options
}

func (*entitySupport) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}

func EntitySupport() cel.EnvOption {
	return cel.Lib(&entitySupport{})
}

func untypedLabels(args ...ref.Val) ref.Val {
	return untypedStringStringMap(args[0],
		func(u unstructured.Unstructured) map[string]string {
			return u.GetLabels()
		},
	)
}

func untypedAnnotations(args ...ref.Val) ref.Val {
	return untypedStringStringMap(args[0],
		func(u unstructured.Unstructured) map[string]string {
			return u.GetAnnotations()
		},
	)
}

func untypedGVK(args ...ref.Val) ref.Val {
	gvk, _, errVal := untypedGVKAndNamespace(args[0])
	if errVal != nil {
		return errVal
	}
	return types.String(gvk)
}

func untypedGVKN(args ...ref.Val) ref.Val {
	gvk, ns, errVal := untypedGVKAndNamespace(args[0])
	if errVal != nil {
		return errVal
	}
	if gvk == "" {
		return types.String("")
	}
	if ns == "" {
		return types.String(gvk)
	}
	return types.String(gvk + "/" + ns)
}

func untypedGVKAndNamespace(arg ref.Val) (string, string, ref.Val) {
	if arg == types.NullValue {
		return "", "", nil
	}
	v, ok := arg.Value().(map[string]any)
	if !ok {
		return "", "", types.MaybeNoSuchOverloadErr(arg)
	}
	apiVersionRaw, _ := v["apiVersion"].(string)
	kindRaw, _ := v["kind"].(string)
	if apiVersionRaw == "" || kindRaw == "" {
		return "", "", nil
	}
	apiVersion, err := hydraTypes.ParseApiVersion(apiVersionRaw)
	if err != nil {
		return "", "", types.NewErr("invalid apiVersion %q: %v", apiVersionRaw, err)
	}
	gvk := hydraTypes.NewGVK(apiVersion.Group, apiVersion.Version, hydraTypes.Kind(kindRaw)).GVKString()
	namespace := ""
	if metadataRaw, ok := v["metadata"].(map[string]any); ok {
		namespace, _ = metadataRaw["namespace"].(string)
	}
	if namespace == "" {
		if nsRaw, ok := v["namespace"].(string); ok {
			namespace = nsRaw
		}
	}
	return string(gvk), namespace, nil
}

func untypedStringStringMap(arg ref.Val, mapper func(u unstructured.Unstructured) map[string]string,
) ref.Val {
	if arg == types.NullValue {
		return types.NewStringStringMap(types.DefaultTypeAdapter, map[string]string{})
	}
	if v, ok := arg.Value().(map[string]any); ok {
		u := unstructured.Unstructured{
			Object: v,
		}
		annotations := mapper(u)
		if annotations == nil {
			annotations = map[string]string{}
		}
		return types.NewStringStringMap(types.DefaultTypeAdapter, annotations)
	} else {
		return types.MaybeNoSuchOverloadErr(arg)
	}
}

func celUnstructuredType() *types.Type {
	return cel.NullableType(
		cel.MapType(cel.StringType, cel.DynType),
	)
}
