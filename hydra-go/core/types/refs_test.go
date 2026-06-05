package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestRefAttributesFromParserAttributes(t *testing.T) {
	attrs, err := RefAttributesFromParserAttributes([]RefParserAttribute{
		{"origin:app": "in-cluster.cluster-infra.kyverno"},
		{"origin:workload": "kyverno-background-controller"},
	})
	require.NoError(t, err)
	assert.Equal(t, []RefAttribute{
		{Type: "origin:app", Value: "in-cluster.cluster-infra.kyverno"},
		{Type: "origin:workload", Value: "kyverno-background-controller"},
	}, attrs)
}

func TestRefAttributesFromParserAttributes_RejectsLegacyGeneratedKey(t *testing.T) {
	tests := []struct {
		name      string
		legacyVal string
	}{
		{
			name:      "controller value",
			legacyVal: RefGeneratedController,
		},
		{
			name:      "job value",
			legacyVal: RefGeneratedJob,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := RefAttributesFromParserAttributes([]RefParserAttribute{
				{"generated": tt.legacyVal},
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported legacy key")
			assert.Contains(t, err.Error(), RefAttributeOriginGenerated)
		})
	}
}

func TestRefAttributesFromParserAttributes_RejectsMultiEntryObject(t *testing.T) {
	_, err := RefAttributesFromParserAttributes([]RefParserAttribute{
		{
			"origin:app":      "in-cluster.cluster-infra.kyverno",
			"origin:workload": "kyverno-background-controller",
		},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one key/value pair")
}

func TestRefAttributeYamlMarshalUsesSingleKeyObject(t *testing.T) {
	data, err := yaml.Marshal([]RefAttribute{
		{Type: RefAttributeOriginApp, Value: "in-cluster.cluster-infra.kyverno"},
		{Type: RefAttributeOriginWorkload, Value: "kyverno-background-controller"},
	})
	require.NoError(t, err)
	assert.Equal(t, "- origin:app: in-cluster.cluster-infra.kyverno\n- origin:workload: kyverno-background-controller\n", string(data))
}

func TestRefAttributeYamlUnmarshalSupportsSingleKeyObject(t *testing.T) {
	var attrs []RefAttribute
	err := yaml.Unmarshal([]byte("- origin:app: in-cluster.cluster-infra.kyverno\n- origin:workload: kyverno-background-controller\n"), &attrs)
	require.NoError(t, err)
	assert.Equal(t, []RefAttribute{
		{Type: RefAttributeOriginApp, Value: "in-cluster.cluster-infra.kyverno"},
		{Type: RefAttributeOriginWorkload, Value: "kyverno-background-controller"},
	}, attrs)
}

func TestRefAttributeYamlUnmarshalRejectsLegacyTypeValueObject(t *testing.T) {
	var attrs []RefAttribute
	err := yaml.Unmarshal([]byte("- type: origin:app\n  value: in-cluster.cluster-infra.kyverno\n"), &attrs)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exactly one key/value pair")
}
