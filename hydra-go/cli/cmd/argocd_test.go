package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func commandByUsePath(cmd *cobra.Command, prefixes ...string) *cobra.Command {
	current := cmd
	for _, prefix := range prefixes {
		current = childCommandWithUsePrefix(current, prefix)
		if current == nil {
			return nil
		}
	}
	return current
}

func commandSurfaceText(cmd *cobra.Command) string {
	return strings.ToLower(strings.Join([]string{
		cmd.Use,
		cmd.Short,
		cmd.Long,
		cmd.Example,
	}, "\n"))
}

func TestRootCommandContainsArgocdTopLevelCommand(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)
	assert.Contains(t, commandUseNames(rootCmd.Commands()), "argocd")
}

func TestClusterCommandContainsSyncSubcommand(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	clusterCmd := commandByUsePath(rootCmd, "gitops")
	require.NotNil(t, clusterCmd, "expected hydra gitops to exist")

	assert.Contains(t, commandUseNames(clusterCmd.Commands()), "sync")
}

func TestArgocdCommandContainsExpectedSubcommands(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	argocdCmd := commandByUsePath(rootCmd, "argocd")
	require.NotNil(t, argocdCmd, "expected hydra argocd to exist")

	assert.ElementsMatch(t, []string{"status", "sync"}, commandUseNames(argocdCmd.Commands()))
}

func TestArgocdCommandHelpSurfaceShowsOnlyNewSurface(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	argocdCmd := commandByUsePath(rootCmd, "argocd")
	require.NotNil(t, argocdCmd, "expected hydra argocd to exist")

	surface := commandSurfaceText(argocdCmd)
	assert.Contains(t, surface, "hydra argocd status")
	assert.Contains(t, surface, "hydra argocd sync auto")
	assert.Contains(t, surface, "hydra argocd sync manual")
	assert.Contains(t, surface, "hydra argocd sync prevent")
	assert.NotContains(t, surface, "hydra gitops sync")
	assert.NotEmpty(t, strings.TrimSpace(argocdCmd.Example), "hydra argocd help should include examples for the new command family")
}

func TestArgocdStatusCommandHelpSurface(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	statusCmd := commandByUsePath(rootCmd, "argocd", "status")
	require.NotNil(t, statusCmd, "expected hydra argocd status to exist")

	assert.Equal(t, "status [appId...]", statusCmd.Use)
	require.NotNil(t, statusCmd.Args, "status should define explicit arg validation")
	require.NoError(t, statusCmd.Args(statusCmd, []string{}), "status should accept zero app IDs")
	require.NoError(t, statusCmd.Args(statusCmd, []string{"prod.apps.api"}), "status should accept explicit app IDs")

	assert.Contains(t, strings.ToLower(statusCmd.Short), "argocd")
	assert.Contains(t, strings.ToLower(statusCmd.Long), "read-only")
	assert.Contains(t, strings.ToLower(statusCmd.Long), "all visible")
	assert.NotNil(t, statusCmd.Flags().Lookup("hydra-context"))
	assert.NotNil(t, statusCmd.Flags().Lookup("color"))
	assert.NotNil(t, statusCmd.Flags().Lookup("exclude-app"))
	assert.Nil(t, statusCmd.Flags().Lookup("dry-run"))
	assert.Nil(t, statusCmd.Flags().Lookup("helm-network-mode"))
}

func TestArgocdStatusCommandDocumentsReadOnlyRealStatusAndImplicitAllSelection(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	statusCmd := commandByUsePath(rootCmd, "argocd", "status")
	require.NotNil(t, statusCmd, "expected hydra argocd status to exist")

	surface := commandSurfaceText(statusCmd)
	assert.Contains(t, surface, "read-only")
	assert.Contains(t, surface, "real")
	assert.Contains(t, surface, "argocd")
	assert.Contains(t, surface, "status [appid...]")
	assert.Contains(t, surface, "all visible")
	assert.Contains(t, surface, "--exclude-app")
	assert.Contains(t, surface, "hydra argocd status")
	assert.NotContains(t, surface, "hydra gitops sync")
	assert.NotEmpty(t, strings.TrimSpace(statusCmd.Example), "hydra argocd status help should demonstrate the zero-arg and exclude-app surfaces")
}

func TestArgocdSyncCommandHelpSurface(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	syncCmd := commandByUsePath(rootCmd, "argocd", "sync")
	require.NotNil(t, syncCmd, "expected hydra argocd sync to exist")

	assert.Equal(t, "sync", syncCmd.Use)
	assert.Contains(t, strings.ToLower(syncCmd.Long), "appproject sync")
	assert.Contains(t, strings.ToLower(syncCmd.Long), "appproject")
	assert.ElementsMatch(t, []string{"auto", "manual", "prevent"}, commandUseNames(syncCmd.Commands()))
}

func TestArgocdSyncCommandDocumentsCanonicalMutatingSurface(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	syncCmd := commandByUsePath(rootCmd, "argocd", "sync")
	require.NotNil(t, syncCmd, "expected hydra argocd sync to exist")

	surface := commandSurfaceText(syncCmd)
	assert.Contains(t, strings.ToLower(surface), "appproject")
	assert.Contains(t, surface, "appproject")
	assert.Contains(t, surface, "--exclude-app")
	assert.Contains(t, surface, "hydra argocd sync auto")
	assert.Contains(t, surface, "hydra argocd sync manual")
	assert.Contains(t, surface, "hydra argocd sync prevent")
	assert.NotContains(t, surface, "hydra gitops sync")
	assert.NotEmpty(t, strings.TrimSpace(syncCmd.Example), "hydra argocd sync help should include examples for the canonical mutating surface")
}

func TestArgocdSyncLeafCommandsRequireAppIdsAndExposeFlags(t *testing.T) {
	mock := newMockRootCommand()
	rootCmd, _ := newRootCommand(mock.rootCommandParams())

	require.NotNil(t, rootCmd)

	testCases := map[string]string{
		"auto":    "auto <appId> [appId...]",
		"manual":  "manual <appId> [appId...]",
		"prevent": "prevent <appId> [appId...]",
	}

	for name, expectedUse := range testCases {
		t.Run(name, func(t *testing.T) {
			leafCmd := commandByUsePath(rootCmd, "argocd", "sync", name)
			require.NotNil(t, leafCmd, "expected hydra argocd sync %s to exist", name)

			assert.Equal(t, expectedUse, leafCmd.Use)
			require.NotNil(t, leafCmd.Args, "%s should define explicit arg validation", name)
			require.Error(t, leafCmd.Args(leafCmd, []string{}), "%s should require at least one app ID", name)
			require.NoError(t, leafCmd.Args(leafCmd, []string{"prod.apps.api"}), "%s should accept one or more app IDs", name)

			assert.NotNil(t, leafCmd.Flags().Lookup("hydra-context"))
			assert.NotNil(t, leafCmd.Flags().Lookup("color"))
			assert.NotNil(t, leafCmd.Flags().Lookup("dry-run"))
			assert.NotNil(t, leafCmd.Flags().Lookup("exclude-app"))
			assert.Nil(t, leafCmd.Flags().Lookup("helm-network-mode"))
		})
	}
}
