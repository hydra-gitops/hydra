package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterValuesCommandRegisters(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	v := childCommandWithUse(cmd, "values")
	require.NotNil(t, v, "expected hydra gitops values to exist")
	assert.Contains(t, v.Use, "values")

	assert.NotNil(t, v.Flags().Lookup("hydra-context"))
	assert.NotNil(t, v.Flags().Lookup("helm-network-mode"))
}
