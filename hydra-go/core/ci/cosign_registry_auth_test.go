package ci

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	orasauth "oras.land/oras-go/v2/registry/remote/auth"
	orascredentials "oras.land/oras-go/v2/registry/remote/credentials"
)

func TestResolveCosignRegistryOptions_UsesHelmRegistryConfig(t *testing.T) {
	helmConfig := filepath.Join(t.TempDir(), "helm", "registry", "config.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(helmConfig), 0o755))
	writeRegistryCredential(t, helmConfig, "harbor.example.com", orasauth.Credential{
		Username: "helm-user",
		Password: "helm-pass",
	})

	t.Setenv("HELM_REGISTRY_CONFIG", helmConfig)
	t.Setenv("DOCKER_CONFIG", filepath.Join(t.TempDir(), "docker-empty"))

	opts, err := resolveCosignRegistryOptions("oci://harbor.example.com/project/chart")
	require.NoError(t, err)
	assert.Equal(t, "helm-user", opts.AuthConfig.Username)
	assert.Equal(t, "helm-pass", opts.AuthConfig.Password)
}

func TestResolveCosignRegistryOptions_FallsBackToDockerConfig(t *testing.T) {
	helmConfig := filepath.Join(t.TempDir(), "helm", "registry", "config.json")
	dockerDir := t.TempDir()
	dockerConfig := filepath.Join(dockerDir, "config.json")
	writeRegistryCredential(t, dockerConfig, "harbor.example.com", orasauth.Credential{
		Username: "docker-user",
		Password: "docker-pass",
	})

	t.Setenv("HELM_REGISTRY_CONFIG", helmConfig)
	t.Setenv("DOCKER_CONFIG", dockerDir)

	opts, err := resolveCosignRegistryOptions("harbor.example.com/project/chart@sha256:deadbeef")
	require.NoError(t, err)
	assert.Equal(t, "docker-user", opts.AuthConfig.Username)
	assert.Equal(t, "docker-pass", opts.AuthConfig.Password)
}

func TestRegistryServerAddress_PreservesPort(t *testing.T) {
	serverAddress, err := registryServerAddress("harbor.example.com:8443/project/chart@sha256:deadbeef")
	require.NoError(t, err)
	assert.Equal(t, "harbor.example.com:8443", serverAddress)
}

func writeRegistryCredential(t *testing.T, configPath string, server string, cred orasauth.Credential) {
	t.Helper()

	store, err := orascredentials.NewStore(configPath, orascredentials.StoreOptions{
		AllowPlaintextPut:        true,
		DetectDefaultNativeStore: false,
	})
	require.NoError(t, err)
	require.NoError(t, store.Put(context.Background(), server, cred))
}
