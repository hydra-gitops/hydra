package yaml

import (
	"encoding/json"
	"math"
	"strings"

	"hydra-gitops.org/hydra/hydra-go/core/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/cli-runtime/pkg/printers"
)

var printer = printers.OmitManagedFieldsPrinter{Delegate: &printers.YAMLPrinter{}}

// normalizeJSONValueForDeepCopy returns a deep copy of v with numeric types adjusted so
// k8s.io/apimachinery/pkg/runtime.DeepCopyJSONValue does not panic. That function only
// accepts int64 (not int), and rejects other integer kinds that sometimes appear in
// map[string]interface{} built from Go code rather than encoding/json.
func normalizeJSONValueForDeepCopy(v interface{}) interface{} {
	switch x := v.(type) {
	case map[string]interface{}:
		out := make(map[string]interface{}, len(x))
		for k, val := range x {
			out[k] = normalizeJSONValueForDeepCopy(val)
		}
		return out
	case []interface{}:
		out := make([]interface{}, len(x))
		for i, val := range x {
			out[i] = normalizeJSONValueForDeepCopy(val)
		}
		return out
	case int:
		return int64(x)
	case int8:
		return int64(x)
	case int16:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case uint:
		if x <= uint(math.MaxInt64) {
			return int64(x)
		}
		return float64(x)
	case uint8:
		return int64(x)
	case uint16:
		return int64(x)
	case uint32:
		return int64(x)
	case uint64:
		if x <= uint64(math.MaxInt64) {
			return int64(x)
		}
		return float64(x)
	case string, bool, float64, nil, json.Number:
		return x
	default:
		return x
	}
}

func normalizeUnstructuredObjectForDeepCopy(m map[string]interface{}) map[string]interface{} {
	if m == nil {
		return nil
	}
	return normalizeJSONValueForDeepCopy(m).(map[string]interface{})
}

// NormalizeUnstructuredObjectForDeepCopy returns a recursive copy of m with numeric types
// adjusted so *unstructured.Unstructured.DeepCopy (runtime.DeepCopyJSONValue) does not panic.
// yaml.v3 decoding and global.hydra.templatePatches yq round-trips often yield Go int, which
// Kubernetes JSON deep-copy rejects (only int64 among integers).
func NormalizeUnstructuredObjectForDeepCopy(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	return normalizeJSONValueForDeepCopy(m).(map[string]any)
}

func PrintObject(
	keepServerFields types.KeepServerFields,
	comment *string,
	object runtime.Object,
) (types.YamlString, error) {
	var err error

	if !keepServerFields {
		if u, ok := object.(*unstructured.Unstructured); ok {
			object = &unstructured.Unstructured{
				Object: normalizeUnstructuredObjectForDeepCopy(u.Object),
			}
		}
		object = object.DeepCopyObject()
		object, err = StripServerFields(object)
		if err != nil {
			return "", err
		}
	}

	sb := &strings.Builder{}
	err = printer.PrintObj(object, sb)
	if err != nil {
		return "", err
	}

	str := sb.String()
	str = strings.TrimPrefix(str, "---\n")
	str = strings.TrimSuffix(str, "\n")

	if comment != nil {
		sb.Reset()
		sb.WriteString("# ")
		sb.WriteString(*comment)
		sb.WriteString("\n")
		sb.WriteString(str)
		return types.YamlString(sb.String()), nil
	}

	return types.YamlString(str), nil
}

func StripServerFields(obj runtime.Object) (runtime.Object, error) {
	// Avoid runtime.DefaultUnstructuredConverter.ToUnstructured for values that are already
	// unstructured: that path JSON-round-trips the object map and can strip a leading newline
	// inside ConfigMap/Secret string data (differs from helm template / kubectl).
	if uin, ok := obj.(*unstructured.Unstructured); ok {
		u := uin.DeepCopy()
		stripMetadata(u)
		stripStatus(u)
		return u, nil
	}

	unstructuredObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{Object: unstructuredObj}

	stripMetadata(u)
	stripStatus(u)

	return u, nil
}

func stripMetadata(u *unstructured.Unstructured) {
	m, ok := u.Object["metadata"].(map[string]any)
	if !ok {
		return
	}

	delete(m, "managedFields")
	delete(m, "creationTimestamp")
	delete(m, "resourceVersion")
	delete(m, "uid")
	delete(m, "generation")
	delete(m, "selfLink")
	delete(m, "deletionTimestamp")
	delete(m, "deletionGracePeriodSeconds")
	delete(m, "ownerReferences")
}

func stripStatus(u *unstructured.Unstructured) {
	delete(u.Object, "status")
}
