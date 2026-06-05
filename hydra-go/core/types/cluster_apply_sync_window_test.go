package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseClusterApplySyncWindow(t *testing.T) {
	m, err := ParseClusterApplySyncWindow("keep-or-prevent")
	require.NoError(t, err)
	assert.Equal(t, ClusterApplySyncWindowKeepOrPrevent, m)

	m, err = ParseClusterApplySyncWindow("manual")
	require.NoError(t, err)
	assert.Equal(t, ClusterApplySyncWindowManual, m)

	m, err = ParseClusterApplySyncWindow("deny")
	require.NoError(t, err)
	assert.Equal(t, ClusterApplySyncWindowManual, m)

	m, err = ParseClusterApplySyncWindow("keep-or-manual")
	require.NoError(t, err)
	assert.Equal(t, ClusterApplySyncWindowKeepOrManual, m)

	m, err = ParseClusterApplySyncWindow("keep-or-deny")
	require.NoError(t, err)
	assert.Equal(t, ClusterApplySyncWindowKeepOrManual, m)

	_, err = ParseClusterApplySyncWindow("invalid")
	require.Error(t, err)
}
