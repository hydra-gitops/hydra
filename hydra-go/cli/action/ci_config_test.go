package action

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"hydra-gitops.org/hydra/hydra-go/core/ci"
)

func createMinimalAppsDir(t *testing.T, dir string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "placeholder"), 0o755))
}

func TestCiConfigInit_AllDefaults(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	input := strings.Join([]string{
		"y",                   // keep default rootAppsPath
		"y",                   // keep environments (empty, no detection)
		"n",                   // don't keep registry (empty) → enter new
		"oci://test-registry", // new registry value
		"y",                   // keep new registry value
		"y",                   // keep default secretsPath
		"y",                   // keep default webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	defCfg := ci.DefaultConfig()
	assert.Equal(t, defCfg.CI.RootAppsPath, cfg.CI.RootAppsPath)
	assert.Equal(t, defCfg.CI.Environments, cfg.CI.Environments)
	assert.Equal(t, "oci://test-registry", cfg.CI.Registry)
	assert.Empty(t, cfg.CI.AppGroups)
	assert.Empty(t, cfg.CI.Promote.PromotableRootApps)
	assert.Empty(t, cfg.CI.Teams.WebhookURL)
}

func TestCiConfigInit_ExistingFileRoundtrip(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	original := ci.Config{
		CI: ci.CIConfig{
			RootAppsPath: "apps",
			Environments: []string{"dev", "stage", "prod"},
			AppGroups: []ci.AppGroup{
				{Name: "demo", Path: "apps/demo"},
				{Name: "cluster-infra", Path: "apps/cluster-infra"},
			},
			Registry: "oci://ghcr.io/example-org/helm-charts",
			Promote:  ci.Promote{PromotableRootApps: []string{}},
			Teams: ci.Teams{
				WebhookURL: "https://teams.example.com/webhook",
			},
		},
	}

	originalData, err := yaml.Marshal(&original)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, originalData, 0644))

	input := strings.Join([]string{
		"y", // keep rootAppsPath
		"y", // keep environments
		"y", // keep registry
		"y", // keep secretsPath
		"y", // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err = CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var roundtripped ci.Config
	require.NoError(t, yaml.Unmarshal(data, &roundtripped))

	assert.Equal(t, original.CI.RootAppsPath, roundtripped.CI.RootAppsPath)
	assert.Equal(t, original.CI.Environments, roundtripped.CI.Environments)
	assert.Equal(t, original.CI.Registry, roundtripped.CI.Registry)
	assert.Equal(t, original.CI.Teams.WebhookURL, roundtripped.CI.Teams.WebhookURL)
	assert.Equal(t, original.CI.AppGroups, roundtripped.CI.AppGroups)
	assert.Equal(t, original.CI.Promote.PromotableRootApps, roundtripped.CI.Promote.PromotableRootApps)
}

func TestCiConfigInit_ModifyField(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	original := ci.Config{
		CI: ci.CIConfig{
			RootAppsPath: "apps",
			Environments: []string{"dev", "stage", "prod"},
			Registry:     "oci://old-registry",
			Teams: ci.Teams{
				WebhookURL: "https://teams.example.com/webhook",
			},
		},
	}

	originalData, err := yaml.Marshal(&original)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, originalData, 0644))

	input := strings.Join([]string{
		"y",                  // keep rootAppsPath
		"y",                  // keep environments
		"n",                  // don't keep registry → change
		"oci://new-registry", // new registry value
		"y",                  // keep new registry value
		"y",                  // keep secretsPath
		"y",                  // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err = CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "oci://new-registry", cfg.CI.Registry)
	assert.Equal(t, original.CI.RootAppsPath, cfg.CI.RootAppsPath)
	assert.Equal(t, original.CI.Environments, cfg.CI.Environments)
	assert.Equal(t, original.CI.Teams.WebhookURL, cfg.CI.Teams.WebhookURL)
}

func TestCiConfigInit_InvalidInput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ci.ConfigFileName)

	in := strings.NewReader("")
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.Error(t, err)

	_, statErr := os.Stat(path)
	assert.True(t, os.IsNotExist(statErr), "file should not be created on error")
}

func TestCiConfigInit_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	partialYAML := "ci:\n  rootAppsPath: \"apps\"\n  registry: \"oci://existing\"\n"
	require.NoError(t, os.WriteFile(path, []byte(partialYAML), 0644))

	input := strings.Join([]string{
		"y", // keep rootAppsPath
		"y", // keep environments (empty, no detection)
		"y", // keep registry
		"y", // keep secretsPath
		"y", // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "oci://existing", cfg.CI.Registry)
	assert.Equal(t, "apps", cfg.CI.RootAppsPath)
	assert.Empty(t, cfg.CI.Environments)
}

func TestCiConfigInit_DetectsEnvironmentsFromDirs(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "demo", "root", "dev"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "demo", "service-ui", "dev"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "cicd", "root", "dev"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "infra", "cert-manager", "dev"), 0o755))

	path := filepath.Join(dir, ci.ConfigFileName)

	input := strings.Join([]string{
		"y",                   // keep default rootAppsPath
		"y",                   // keep auto-detected environments
		"n",                   // don't keep registry → change
		"oci://test-registry", // registry value
		"y",                   // keep new registry value
		"y",                   // keep secretsPath
		"y",                   // keep default webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, []string{"dev", "stage", "prod"}, cfg.CI.Environments)
	assert.Contains(t, out.String(), "Found root apps:")
	assert.Contains(t, out.String(), "Found environments:")
}

func TestCiConfigInit_EditLoop(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	input := strings.Join([]string{
		"y",                    // keep default rootAppsPath
		"y",                    // keep environments (empty, no detection)
		"n",                    // don't keep registry → change
		"oci://first-attempt",  // first value
		"n",                    // don't keep → change again (shows [y/n/r])
		"oci://second-attempt", // corrected value
		"y",                    // keep
		"y",                    // keep secretsPath
		"y",                    // keep default webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "oci://second-attempt", cfg.CI.Registry)

	output := out.String()
	assert.Contains(t, output, "oci://first-attempt")
	assert.Contains(t, output, "oci://second-attempt")
}

func TestCiConfigInit_DirectoryPath(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)

	input := strings.Join([]string{
		"y",                   // keep default rootAppsPath
		"y",                   // keep environments (empty, no detection)
		"n",                   // don't keep registry → change
		"oci://test-registry", // new registry value
		"y",                   // keep new registry value
		"y",                   // keep secretsPath
		"y",                   // keep default webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(dir, in, &out, false)
	require.NoError(t, err)

	expectedPath := filepath.Join(dir, ci.ConfigFileName)
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))
	assert.Equal(t, "oci://test-registry", cfg.CI.Registry)
}

func TestCiConfigInit_RevertToOriginal(t *testing.T) {
	dir := t.TempDir()
	createMinimalAppsDir(t, dir)
	path := filepath.Join(dir, ci.ConfigFileName)

	original := ci.Config{
		CI: ci.CIConfig{
			RootAppsPath: "apps",
			Environments: []string{"dev", "stage", "prod"},
			Registry:     "oci://original-registry",
		},
	}

	originalData, err := yaml.Marshal(&original)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, originalData, 0644))

	input := strings.Join([]string{
		"y",                 // keep rootAppsPath
		"y",                 // keep environments
		"n",                 // don't keep registry → change
		"oci://wrong-value", // enter wrong value
		"r",                 // revert to original [y/n/r]
		"y",                 // keep reverted value
		"y",                 // keep secretsPath
		"y",                 // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err = CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "oci://original-registry", cfg.CI.Registry)
	assert.Contains(t, out.String(), "[Y/n/r]")
}

func TestCiConfigInit_NoRootAppsForceNewValue(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ci.ConfigFileName)

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "charts", "demo"), 0o755))

	input := strings.Join([]string{
		// rootAppsPath: default "apps" has no root apps → directly "New value:"
		"charts",              // enter correct path (has subdirs)
		"y",                   // keep new rootAppsPath
		"y",                   // keep environments (empty)
		"n",                   // don't keep registry → change
		"oci://test-registry", // registry value
		"y",                   // keep registry
		"y",                   // keep secretsPath
		"y",                   // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, "charts", cfg.CI.RootAppsPath)
	assert.Contains(t, out.String(), "No root apps found")
}

func TestCiConfigInit_AutoDetectEnvironments(t *testing.T) {
	dir := t.TempDir()

	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "demo", "service-ui", "dev"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "demo", "service-ui", "stage"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "apps", "demo", "service-ui", "prod"), 0o755))

	path := filepath.Join(dir, ci.ConfigFileName)

	input := strings.Join([]string{
		"y",                   // keep default rootAppsPath ("apps")
		"y",                   // keep auto-detected environments
		"n",                   // don't keep registry → change
		"oci://test-registry", // registry value
		"y",                   // keep registry
		"y",                   // keep secretsPath
		"y",                   // keep webhookUrl
	}, "\n") + "\n"

	in := strings.NewReader(input)
	var out bytes.Buffer

	err := CiConfigInit(path, in, &out, false)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg ci.Config
	require.NoError(t, yaml.Unmarshal(data, &cfg))

	assert.Equal(t, []string{"dev", "stage", "prod"}, cfg.CI.Environments)
	assert.Contains(t, out.String(), "Found environments:")
}
