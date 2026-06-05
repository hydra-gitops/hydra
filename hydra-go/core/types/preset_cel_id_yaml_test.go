package types

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestPresetCelItemUnmarshal_Scalar(t *testing.T) {
	t.Parallel()

	var item PresetCelItem
	err := yaml.Unmarshal([]byte(`'gvk == "v1/Node"'`), &item)
	require.NoError(t, err)
	require.Equal(t, `gvk == "v1/Node"`, item.Expr)
	require.False(t, item.Optional)
	require.True(t, item.Selector.IsZero())
}

func TestPresetCelItemUnmarshal_SelectorAndCel(t *testing.T) {
	t.Parallel()

	var item PresetCelItem
	err := yaml.Unmarshal([]byte(`
gvk: metrics.k8s.io/v1beta1/PodMetrics
namespace: kube-system
cel: 'name.matches("^metrics-server-.*")'
optional: true
`), &item)
	require.NoError(t, err)
	require.Equal(t, `name.matches("^metrics-server-.*")`, item.Expr)
	require.True(t, item.Optional)
	require.Equal(t, Group("metrics.k8s.io"), item.Selector.Group)
	require.Equal(t, Version("v1beta1"), item.Selector.Version)
	require.Equal(t, Kind("PodMetrics"), item.Selector.Kind)
	require.Equal(t, Namespace("kube-system"), item.Selector.Namespace)
}

func TestPresetCelItemUnmarshal_RejectsSelectorConflict(t *testing.T) {
	t.Parallel()

	var item PresetCelItem
	err := yaml.Unmarshal([]byte(`
gvk: apps/v1/Deployment
kind: StatefulSet
cel: 'name == "x"'
`), &item)
	require.Error(t, err)
}
