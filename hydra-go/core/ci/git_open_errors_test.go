package ci

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeParentConfigNoGitApps(t *testing.T) (cfgPath string) {
	t.Helper()
	dir := t.TempDir()
	appsDir := filepath.Join(dir, "apps")
	require.NoError(t, os.MkdirAll(appsDir, 0o755))
	cfgPath = filepath.Join(dir, ConfigFileName)
	const hydraCI = `ci:
  rootAppsPath: apps
  environments:
    - dev
    - stage
  appGroups:
    - name: demo
      path: apps/demo
  registry: "oci://registry/helm"
  promote:
    promotableRootApps: []
`
	require.NoError(t, os.WriteFile(cfgPath, []byte(hydraCI), 0o644))
	return cfgPath
}

func TestRunAuto_OpenRepoFailsWithoutGitInApps(t *testing.T) {
	cfgPath := writeParentConfigNoGitApps(t)
	err := RunAuto(cfgPath, ModeDryRun, "", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open charts repo")
}

func TestRunRelease_OpenRepoFailsWithoutGitInApps(t *testing.T) {
	cfgPath := writeParentConfigNoGitApps(t)
	_, err := RunRelease(cfgPath, ModeDryRun, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}

func TestRunPromote_OpenRepoFailsWithoutGitInApps(t *testing.T) {
	cfgPath := writeParentConfigNoGitApps(t)
	_, err := RunPromote(cfgPath, ModeDryRun, &dryRunPromoteActions{}, "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open repo")
}
