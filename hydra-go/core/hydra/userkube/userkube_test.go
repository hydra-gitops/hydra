package userkube

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultConfigFilePath_respectsXDG(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	path, err := DefaultConfigFilePath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "hydra", "config.yaml"), path)
}

func TestDefaultConfigFilePath_fallbackHomeConfig(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := DefaultConfigFilePath()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(home, ".config", "hydra", "config.yaml"), path)
}

func TestReadOptionalFile_missing(t *testing.T) {
	cfg, err := ReadOptionalFile(filepath.Join(t.TempDir(), "nope.yaml"))
	require.NoError(t, err)
	require.Nil(t, cfg)
}

func TestReadOptionalFile_validYAML(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte("contexts:\n  - path: /x\n    config: /k\n    name: c\n"), 0o600))
	cfg, err := ReadOptionalFile(p)
	require.NoError(t, err)
	require.Len(t, cfg.Contexts, 1)
	require.Equal(t, "/x", cfg.Contexts[0].Path)
}

func TestKubeMappingForClusterDir_firstMatchWins(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cluster-a")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	abs, err := filepath.Abs(dir)
	require.NoError(t, err)

	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: abs, Config: "/first", Name: "ctx-first"},
			{Path: abs, Config: "/second", Name: "ctx-second"},
		},
	}
	k, n, ok, via := cfg.KubeMappingForClusterDir(dir)
	require.True(t, ok)
	require.False(t, via)
	require.Equal(t, "/first", k)
	require.Equal(t, "ctx-first", n)
}

func TestKubeMappingForClusterDir_skipsIncompleteEntries(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "c")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: dir, Config: "", Name: "x"},
			{Path: dir, Config: "/k", Name: "good"},
		},
	}
	k, n, ok, via := cfg.KubeMappingForClusterDir(dir)
	require.True(t, ok)
	require.False(t, via)
	require.Equal(t, "/k", k)
	require.Equal(t, "good", n)
}

func TestKubeMappingForClusterDir_noMatch(t *testing.T) {
	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: "/other/cluster", Config: "/k", Name: "c"},
		},
	}
	_, _, ok, via := cfg.KubeMappingForClusterDir(t.TempDir())
	require.False(t, ok)
	require.False(t, via)
}

func TestInvalidKubeconfigPaths_missingAndDir(t *testing.T) {
	tmp := t.TempDir()
	kubeOK := filepath.Join(tmp, "kube.yaml")
	require.NoError(t, os.WriteFile(kubeOK, []byte("apiVersion: v1\n"), 0o600))
	kubeDir := filepath.Join(tmp, "kube-is-dir")
	require.NoError(t, os.MkdirAll(kubeDir, 0o755))

	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: "/x", Config: filepath.Join(tmp, "missing.yaml"), Name: "c"},
			{Path: "/x", Config: kubeDir, Name: "c2"},
			{Path: "/x", Config: kubeOK, Name: "c3"},
		},
	}
	inv := cfg.InvalidKubeconfigPaths()
	require.Len(t, inv, 2)
	require.Equal(t, "does not exist", inv[0].Detail)
	require.Equal(t, "is a directory, expected a kubeconfig file", inv[1].Detail)
}

func TestInvalidContextPaths_missingAndNotDir(t *testing.T) {
	tmp := t.TempDir()
	realDir := filepath.Join(tmp, "ok")
	require.NoError(t, os.MkdirAll(realDir, 0o755))
	fileAsPath := filepath.Join(tmp, "file-not-dir")
	require.NoError(t, os.WriteFile(fileAsPath, []byte("x"), 0o600))

	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: filepath.Join(tmp, "nope"), Config: "/k", Name: "c"},
			{Path: fileAsPath, Config: "/k2", Name: "c2"},
			{Path: realDir, Config: "/k3", Name: "c3"},
		},
	}
	inv := cfg.InvalidContextPaths()
	require.Len(t, inv, 2)
	require.Equal(t, "does not exist", inv[0].Detail)
	require.Equal(t, "not a directory", inv[1].Detail)
}

func TestKubeMappingForClusterDir_parentMatchSetsViaHydraContextFlag(t *testing.T) {
	ctxRoot := t.TempDir()
	clusterDir := filepath.Join(ctxRoot, "in-cluster")
	require.NoError(t, os.MkdirAll(clusterDir, 0o755))

	cfg := &UserKubeConfig{
		Contexts: []ContextMapping{
			{Path: ctxRoot, Config: "/kube", Name: "ctx-a"},
		},
	}
	k, n, ok, via := cfg.KubeMappingForClusterDir(clusterDir)
	require.True(t, ok)
	require.True(t, via)
	require.Equal(t, "/kube", k)
	require.Equal(t, "ctx-a", n)
}
