package values

import (
	"fmt"
	"maps"
	"os"

	"hydra-gitops.org/hydra/hydra-go/base/log"

	"hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/core/types"
	"hydra-gitops.org/hydra/hydra-go/core/yaml"
)

// LoadValuesFile loads a YAML values file from the given path
// Returns an empty map if the file doesn't exist, or an error if the file cannot be read/parsed
func LoadValuesFile(l log.Logger, path string) (types.ValuesMap, error) {
	vals := make(types.ValuesMap)

	// Return empty map if file doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		l.DebugLog(logIdValues, "Values file does not exist, returning empty map",
			log.String("path", path))
		return vals, nil
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, log.CreateError(
			errors.ErrValuesFailed,
			"could not read values file '{path}': {err}",
			log.String("path", path),
			log.Err(err))
	}

	// Parse the YAML
	vals, err = ParseValuesString(types.YamlString(data))
	if err != nil {
		return nil, log.CreateError(
			errors.ErrValuesFailed,
			"could not parse values file '{path}': {err}",
			log.String("path", path),
			log.Err(err))
	}

	l.DebugLog(logIdValues, "Loaded values file",
		log.String("path", path))

	return vals, nil
}

// MergeValues performs a deep merge of two maps, with right values taking precedence
// Returns a new map with the merged values without modifying the input maps
func MergeValues(left, right types.ValuesMap) types.ValuesMap {
	// Create a copy of left to avoid modifying the original
	result := make(types.ValuesMap)
	maps.Copy(result, left)

	// Merge right into result
	for key, rightValue := range right {
		if leftValue, exists := result[key]; exists {
			// If both values are maps, merge them recursively
			if leftMap, ok := leftValue.(types.ValuesMap); ok {
				if rightMap, ok := rightValue.(types.ValuesMap); ok {
					result[key] = MergeValues(leftMap, rightMap)
					continue
				}
			}
		}
		// Otherwise, use the right value (overwrite or new key)
		result[key] = rightValue
	}

	return result
}

// LoadAndMergeValuesFile loads a YAML values file from the given path and merges it with the provided values map
// The loaded file values take precedence over the provided values (right overwrites left)
// Returns the merged map or an error if the file cannot be read/parsed
func LoadAndMergeValuesFile(l log.Logger, filePath string, existingValues types.ValuesMap) (types.ValuesMap, error) {
	// Load values from file
	fileValues, err := LoadValuesFile(l, filePath)
	if err != nil {
		return nil, err
	}

	// Merge file values into existing values (file values override existing)
	mergedValues := MergeValues(existingValues, fileValues)

	return mergedValues, nil
}

// ParseValuesString parses YAML values from a string content
func ParseValuesString(content types.YamlString) (types.ValuesMap, error) {
	vals, err := yaml.FromYaml[types.ValuesMap](content)
	if err != nil {
		return nil, err
	}

	return vals, nil
}

// MergeGlobalValues merges values with app-specific values if present
func MergeGlobalValues(values types.ValuesMap, appKey string) (types.ValuesMap, error) {
	// Merge with app-specific values if present
	if appKey == "" {
		return values, nil
	}
	if childValues, ok := values[appKey]; ok {
		if childMap, ok := childValues.(types.ValuesMap); ok {
			return MergeValues(values, childMap), nil
		}
	}
	return values, nil
}

// Lookup traverses a map using variadic keys and returns the value nil-safe.
// Example: Lookup(m, "global", "hydra", "values-cleanup")
func Lookup(m map[string]any, keys ...string) any {
	current := any(m)

	for _, key := range keys {
		// Check if current is a map
		if mapVal, ok := current.(map[string]any); !ok {
			return nil
		} else {
			// Safe retrieval from map
			current, ok = mapVal[key]
			if !ok {
				return nil
			}
		}
	}

	return current
}

func LookupMap(m map[string]any, keys ...string) (map[string]any, error) {
	if len(keys) == 0 {
		return m, nil
	}
	value := Lookup(m, keys[0])
	if value == nil {
		return nil, nil
	}

	if result, ok := value.(map[string]any); ok {
		return LookupMap(result, keys[1:]...)
	}
	return nil, log.CreateError(errors.ErrValuesFailed,
		"value at keys {keys} is not a map[string]any, it is {type}",
		log.String("keys", fmt.Sprintf("%v", keys)),
		log.String("type", fmt.Sprintf("%T", value)))
}
