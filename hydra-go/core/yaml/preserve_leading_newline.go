package yaml

import (
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

// preservedString forces double-quoted YAML so strings starting with "\n" survive
// yaml.v3's Encode path (literal block + re-parse drops that leading newline).
type preservedString struct{ s string }

func (p preservedString) MarshalYAML() (interface{}, error) {
	var n yaml.Node
	n.Kind = yaml.ScalarNode
	n.Tag = "!!str"
	n.Style = yaml.DoubleQuotedStyle
	n.Value = p.s
	return &n, nil
}

// preserveLeadingNewlines returns a deep copy of v with strings that start with '\n'
// replaced by preservedString so ToYaml round-trips them through yaml.v3 Encode.
func preserveLeadingNewlines(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		if strings.HasPrefix(x, "\n") {
			return preservedString{s: x}
		}
		return x
	case map[string]any:
		out := make(map[string]any, len(x))
		for k, val := range x {
			out[k] = preserveLeadingNewlines(val)
		}
		return out
	case []any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = preserveLeadingNewlines(x[i])
		}
		return out
	default:
		return preserveLeadingNewlinesReflect(reflect.ValueOf(v))
	}
}

func preserveLeadingNewlinesReflect(rv reflect.Value) any {
	if !rv.IsValid() {
		return nil
	}
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		return preserveLeadingNewlines(rv.Elem().Interface())
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		t := rv.Type()
		if t.Key().Kind() != reflect.String {
			out := reflect.MakeMapWithSize(t, rv.Len())
			for _, mk := range rv.MapKeys() {
				out.SetMapIndex(mk, reflect.ValueOf(preserveLeadingNewlines(rv.MapIndex(mk).Interface())))
			}
			return out.Interface()
		}
		out := reflect.MakeMapWithSize(t, rv.Len())
		for _, mk := range rv.MapKeys() {
			out.SetMapIndex(mk, reflect.ValueOf(preserveLeadingNewlines(rv.MapIndex(mk).Interface())))
		}
		return out.Interface()
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		if rv.Type().Elem().Kind() == reflect.Uint8 {
			return rv.Interface()
		}
		// Use []any so elements can become preservedString (cannot mix into []string).
		n := rv.Len()
		out := make([]any, n)
		for i := 0; i < n; i++ {
			out[i] = preserveLeadingNewlines(rv.Index(i).Interface())
		}
		return out
	default:
		return rv.Interface()
	}
}
