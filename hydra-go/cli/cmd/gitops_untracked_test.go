package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterUntrackedCommandRegistersFlags(t *testing.T) {
	cmd := NewClusterCommand(ClusterCommandParams{})

	untracked := childCommandWithUse(cmd, "untracked")
	require.NotNil(t, untracked, "expected hydra gitops untracked to exist")
	assert.Equal(t, "untracked <cluster>", untracked.Use)

	assert.NotNil(t, untracked.Flags().Lookup("parallel"))
	assert.NotNil(t, untracked.Flags().Lookup("include"))
}
