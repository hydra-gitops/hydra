package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClusterCommandContainsStatusSubcommand(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	statusCmd := commandByUsePath(rootCmd, "gitops", "status")
	require.NotNil(t, statusCmd, "expected hydra gitops status to exist")

	assert.Equal(t, "status <appId...>", statusCmd.Use)
}

func TestClusterStatusCommandHelpSurface(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	statusCmd := commandByUsePath(rootCmd, "gitops", "status")
	require.NotNil(t, statusCmd, "expected hydra gitops status to exist")

	require.NotNil(t, statusCmd.Args, "cluster status should define explicit arg validation")
	require.Error(t, statusCmd.Args(statusCmd, []string{}), "cluster status should require at least one app ID")
	require.NoError(t, statusCmd.Args(statusCmd, []string{"prod.apps.api"}), "cluster status should accept explicit app IDs")

	assert.Contains(t, strings.ToLower(statusCmd.Short), "sync")
	assert.Contains(t, strings.ToLower(statusCmd.Long), "rendered desired state")
	assert.Contains(t, strings.ToLower(statusCmd.Long), "live cluster resources")
	assert.NotNil(t, statusCmd.Flags().Lookup("hydra-context"))
	assert.NotNil(t, statusCmd.Flags().Lookup("helm-network-mode"))
	assert.NotNil(t, statusCmd.Flags().Lookup("exclude-app"))
	assert.Nil(t, statusCmd.Flags().Lookup("cluster"), "cluster status should derive the cluster from the selected app IDs")
}

func TestClusterStatusCommandDocumentsSelectionAndSyncSemantics(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	statusCmd := commandByUsePath(rootCmd, "gitops", "status")
	require.NotNil(t, statusCmd, "expected hydra gitops status to exist")

	surface := commandSurfaceText(statusCmd)
	assert.Contains(t, surface, "status <appid...>")
	assert.Contains(t, surface, "rendered desired state")
	assert.Contains(t, surface, "live cluster resources")
	assert.Contains(t, surface, "in sync")
	assert.Contains(t, surface, "out of sync")
	assert.Contains(t, surface, "--exclude-app")
	assert.Contains(t, surface, "same cluster")
	assert.Contains(t, surface, "hydra gitops status")
	assert.NotContains(t, surface, "hydra gitops sync")
	assert.NotEmpty(t, strings.TrimSpace(statusCmd.Example), "hydra gitops status help should include examples for include and exclude selection")
}
