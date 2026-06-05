package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterSyncStatusCommandExists(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	syncStatusCmd := commandByUsePath(rootCmd, "gitops", "sync", "status")
	require.NotNil(t, syncStatusCmd, "expected hydra gitops sync status to exist")

	assert.Equal(t, "status [appId...]", syncStatusCmd.Use)
}
