package helm

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChartSourceTemplatesMultidocFromChartDirectory(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "templates", "a.yaml"), []byte("a: {{ .Values.x }}\n"), 0o644))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "charts", "sub", "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "charts", "sub", "templates", "b.yaml"), []byte("b: 1\n"), 0o644))

	out, err := ChartSourceTemplatesMultidocFromChartDirectory(root, nil)
	require.NoError(t, err)
	assert.Contains(t, out, "# Source: templates/a.yaml")
	assert.Contains(t, out, "{{ .Values.x }}")
	assert.Contains(t, out, "# Source: charts/sub/templates/b.yaml")
}

func TestChartSourceTemplatesMultidocFromChartDirectory_PrefixFilter(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "templates"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "templates", "keep.yaml"), []byte("k: 1\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "templates", "skip.yaml"), []byte("s: 1\n"), 0o644))

	out, err := ChartSourceTemplatesMultidocFromChartDirectory(root, []string{"templates/keep.yaml"})
	require.NoError(t, err)
	assert.Contains(t, out, "# Source: templates/keep.yaml")
	assert.NotContains(t, out, "skip.yaml")
}

func TestDisplayPathMatchesAnySourcePrefix_PathBoundary(t *testing.T) {
	assert.True(t, displayPathMatchesAnySourcePrefix("templates/deploy/x.yaml", []string{"templates/deploy"}))
	assert.False(t, displayPathMatchesAnySourcePrefix("templates/deployment.yaml", []string{"templates/deploy"}))
	assert.True(t, displayPathMatchesAnySourcePrefix("templates/deploy", []string{"templates/deploy"}))
}

func TestDisplayPathMatchesAnySourcePrefix_HelmParentPrefix(t *testing.T) {
	helmPath := "kube-prometheus-stack/charts/kube-prometheus-stack/templates/prometheus/clusterrole.yaml"
	short := "charts/kube-prometheus-stack/templates/prometheus"
	assert.True(t, displayPathMatchesAnySourcePrefix(helmPath, []string{short}))
	assert.True(t, displayPathMatchesAnySourcePrefix(helmPath, []string{
		"charts/kube-prometheus-stack/templates/prometheus/clusterrole.yaml",
	}))
	assert.False(t, displayPathMatchesAnySourcePrefix("templates/deployment.yaml", []string{"deployment"}),
		"single-segment suffix must not match foo-deployment style paths")
}
