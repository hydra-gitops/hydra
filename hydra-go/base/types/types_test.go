package types

import (
	"testing"
)

func TestYamlString(t *testing.T) {
	t.Run("type conversion", func(t *testing.T) {
		yaml := YamlString("key: value")

		if string(yaml) != "key: value" {
			t.Errorf("expected 'key: value', got %q", yaml)
		}
	})

	t.Run("empty string", func(t *testing.T) {
		yaml := YamlString("")

		if string(yaml) != "" {
			t.Errorf("expected empty string, got %q", yaml)
		}
	})

	t.Run("multiline yaml", func(t *testing.T) {
		yaml := YamlString(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test`)

		if len(yaml) == 0 {
			t.Error("YamlString should preserve multiline content")
		}
	})
}

func TestValuesMap(t *testing.T) {
	t.Run("create and access", func(t *testing.T) {
		values := ValuesMap{
			"key1": "value1",
			"key2": 42,
			"nested": map[string]any{
				"innerKey": "innerValue",
			},
		}

		if values["key1"] != "value1" {
			t.Errorf("expected 'value1', got %v", values["key1"])
		}
		if values["key2"] != 42 {
			t.Errorf("expected 42, got %v", values["key2"])
		}
	})

	t.Run("nested access", func(t *testing.T) {
		values := ValuesMap{
			"global": map[string]any{
				"hydra": map[string]any{
					"cluster": "production",
				},
			},
		}

		global, ok := values["global"].(map[string]any)
		if !ok {
			t.Fatal("expected nested map")
		}
		hydra, ok := global["hydra"].(map[string]any)
		if !ok {
			t.Fatal("expected nested map")
		}
		if hydra["cluster"] != "production" {
			t.Errorf("expected 'production', got %v", hydra["cluster"])
		}
	})

	t.Run("empty map", func(t *testing.T) {
		values := ValuesMap{}

		if len(values) != 0 {
			t.Errorf("expected empty map, got %d entries", len(values))
		}
	})

	t.Run("nil value", func(t *testing.T) {
		values := ValuesMap{
			"nilKey": nil,
		}

		if values["nilKey"] != nil {
			t.Error("expected nil value")
		}

		// Check key exists
		_, exists := values["nilKey"]
		if !exists {
			t.Error("key should exist even with nil value")
		}
	})
}

// testEnum for testing EnumType interface
type testEnum int

const (
	testEnumA testEnum = iota
	testEnumB
	testEnumC
)

// mockEnumType implements EnumType[testEnum] for testing
type mockEnumType struct{}

func (m *mockEnumType) Stringify(v testEnum) (string, error) {
	switch v {
	case testEnumA:
		return "A", nil
	case testEnumB:
		return "B", nil
	case testEnumC:
		return "C", nil
	}
	return "", nil
}

func (m *mockEnumType) Parse(s string) (testEnum, error) {
	switch s {
	case "A":
		return testEnumA, nil
	case "B":
		return testEnumB, nil
	case "C":
		return testEnumC, nil
	}
	return testEnumA, nil
}

func (m *mockEnumType) Values() []testEnum {
	return []testEnum{testEnumA, testEnumB, testEnumC}
}

func (m *mockEnumType) Valid(v testEnum) bool {
	return v >= testEnumA && v <= testEnumC
}

// Verify interface compliance
var _ EnumType[testEnum] = (*mockEnumType)(nil)

func TestEnumType_Interface(t *testing.T) {
	impl := &mockEnumType{}

	t.Run("Stringify", func(t *testing.T) {
		s, err := impl.Stringify(testEnumB)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if s != "B" {
			t.Errorf("expected 'B', got %q", s)
		}
	})

	t.Run("Parse", func(t *testing.T) {
		v, err := impl.Parse("C")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if v != testEnumC {
			t.Errorf("expected testEnumC, got %v", v)
		}
	})

	t.Run("Values", func(t *testing.T) {
		values := impl.Values()
		if len(values) != 3 {
			t.Errorf("expected 3 values, got %d", len(values))
		}
	})

	t.Run("Valid", func(t *testing.T) {
		if !impl.Valid(testEnumA) {
			t.Error("testEnumA should be valid")
		}
		if impl.Valid(testEnum(99)) {
			t.Error("testEnum(99) should not be valid")
		}
	})
}
