package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterSystemCommandRegistersYamlFlag(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	system := childCommandWithUse(cmd, "system")
	require.NotNil(t, system, "expected hydra gitops system to exist")
	assert.Equal(t, "system <cluster>", system.Use)

	assert.NotNil(t, system.Flags().Lookup("yaml"))
	assert.NotNil(t, system.Flags().Lookup("color"))
	assert.NotNil(t, system.Flags().Lookup("parallel"))
	assert.NotNil(t, system.Flags().Lookup("all"))
}
