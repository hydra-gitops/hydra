package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHydraCloneTargets_CEL_Roundtrip(t *testing.T) {
	original := HydraCloneRule{
		Predicate: `id == "v1/Secret/ns/name"`,
		Targets:   HydraCloneTargets{CEL: "managedNamespaces()"},
		Tag:       "bootstrap",
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var roundtripped HydraCloneRule
	require.NoError(t, yaml.Unmarshal(data, &roundtripped))
	require.Equal(t, "managedNamespaces()", roundtripped.Targets.CEL)
}

func TestHydraCloneTargets_MapRoundtrip(t *testing.T) {
	// Simulates the HelmHydraMapFromValues → extractClonesFromMergedMap path:
	// struct → YAML → map[string]any → YAML → struct
	original := map[string]HydraCloneRule{
		"mirror": {
			Predicate: `id == "v1/Secret/ns/name"`,
			Targets:   HydraCloneTargets{CEL: "managedNamespaces()"},
			Tag:       "bootstrap",
		},
	}
	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var asMap map[string]any
	require.NoError(t, yaml.Unmarshal(data, &asMap))

	data2, err := yaml.Marshal(asMap)
	require.NoError(t, err)

	var result map[string]HydraCloneRule
	require.NoError(t, yaml.Unmarshal(data2, &result))
	require.Equal(t, "managedNamespaces()", result["mirror"].Targets.CEL)
	require.Equal(t, "bootstrap", result["mirror"].Tag)
}

func TestHydraCloneTargets_IsEmpty(t *testing.T) {
	require.True(t, HydraCloneTargets{}.IsEmpty())
	require.False(t, HydraCloneTargets{CEL: "managedNamespaces()"}.IsEmpty())
}
