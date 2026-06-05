package helm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"helm.sh/helm/v4/pkg/chart/common"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

func TestChartSourceTemplatesMultidoc_IncludesDependencyTemplates(t *testing.T) {
	dep := &v2chart.Chart{
		Metadata: &v2chart.Metadata{Name: "sub"},
		Templates: []*common.File{
			{Name: "templates/dep.yaml", Data: []byte("dep: {{ .Values.x }}\n")},
		},
	}
	root := &v2chart.Chart{
		Metadata: &v2chart.Metadata{Name: "root"},
		Templates: []*common.File{
			{Name: "templates/root.yaml", Data: []byte("root: true\n")},
		},
	}
	root.AddDependency(dep)

	out, err := ChartSourceTemplatesMultidoc(root, nil)
	require.NoError(t, err)
	assert.Contains(t, out, "# Source: templates/root.yaml")
	assert.Contains(t, out, "# Source: templates/dep.yaml")
	assert.Contains(t, out, "{{ .Values.x }}")
}

func TestChartSourceTemplatesMultidoc_PrefixFilter(t *testing.T) {
	dep := &v2chart.Chart{
		Metadata: &v2chart.Metadata{Name: "sub"},
		Templates: []*common.File{
			{Name: "templates/keep.yaml", Data: []byte("k: 1\n")},
			{Name: "templates/skip.yaml", Data: []byte("s: 1\n")},
		},
	}
	root := &v2chart.Chart{
		Metadata: &v2chart.Metadata{Name: "root"},
		Templates: []*common.File{
			{Name: "templates/root.yaml", Data: []byte("r: 1\n")},
		},
	}
	root.AddDependency(dep)

	out, err := ChartSourceTemplatesMultidoc(root, []string{"templates/keep.yaml"})
	require.NoError(t, err)
	assert.Contains(t, out, "keep.yaml")
	assert.NotContains(t, out, "skip.yaml")
	assert.NotContains(t, out, "root.yaml")
}
