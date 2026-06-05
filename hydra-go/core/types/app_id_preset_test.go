package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppId_IsPresetApp(t *testing.T) {
	assert.True(t, AppId("in-cluster.preset.local-path-provisioner").IsPresetApp())
	assert.False(t, AppId("in-cluster.legacy.local-path-provisioner").IsPresetApp())
	assert.False(t, AppId("in-cluster.myapp").IsPresetApp())
	assert.False(t, AppId("in-cluster.myapp.child").IsPresetApp())
}

func TestNewPresetAppId(t *testing.T) {
	id, err := NewPresetAppId(InCluster, "local-path-provisioner")
	require.NoError(t, err)
	assert.Equal(t, AppId("in-cluster.preset.local-path-provisioner"), id)
	assert.True(t, id.IsPresetApp())

	_, err = NewPresetAppId(InCluster, "")
	require.Error(t, err)
	_, err = NewPresetAppId(InCluster, "a.b")
	require.Error(t, err)
	_, err = NewPresetAppId(InCluster, "Bad")
	require.Error(t, err)
}
