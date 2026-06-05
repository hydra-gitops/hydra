package git

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewChart_Builder(t *testing.T) {
	chart := NewChart("ingress-nginx").
		Version("4.11.8-dev").
		Dep("ingress-nginx", "4.11.8", "https://kubernetes.github.io/ingress-nginx")

	require.Equal(t, "ingress-nginx", chart.GetName())
	require.Equal(t, "4.11.8-dev", chart.GetVersion())
	require.Equal(t, "4.11.8", chart.GetDepVersion("ingress-nginx"))
}

func TestNewChart_DefaultType(t *testing.T) {
	chart := NewChart("app").Version("1.0.0")
	yaml := chart.marshalChartYAML()
	require.Contains(t, yaml, "type: application")
}

func TestNewChart_CustomType(t *testing.T) {
	chart := NewChart("lib").Version("1.0.0").Type("library")
	yaml := chart.marshalChartYAML()
	require.Contains(t, yaml, "type: library")
}

func TestChart_GetDepVersion_NotFound(t *testing.T) {
	chart := NewChart("app").Version("1.0.0")
	require.Equal(t, "", chart.GetDepVersion("nonexistent"))
}

func TestChart_PrimaryDependencyVersion_NameMatch(t *testing.T) {
	chart := NewChart("ingress-nginx").
		Version("1.0.0-dev").
		Dep("other", "9.9.9", "oci://x").
		Dep("ingress-nginx", "4.11.8", "oci://y")
	require.Equal(t, "4.11.8", chart.PrimaryDependencyVersion())
}

func TestChart_PrimaryDependencyVersion_FirstDep(t *testing.T) {
	chart := NewChart("wrapper").
		Version("1.0.0").
		Dep("upstream", "3.4.5", "oci://z")
	require.Equal(t, "3.4.5", chart.PrimaryDependencyVersion())
}

func TestChart_PrimaryDependencyVersion_IgnoresWildcard(t *testing.T) {
	chart := NewChart("unit-test-app").
		Version("1.0.0-dev").
		Dep("upstream-lib", "*", "file://charts/upstream").
		Dep("common", "*", "file://../../../../shared/common/dev")
	require.Equal(t, "", chart.PrimaryDependencyVersion())
}

func TestChart_PrimaryDependencyVersion_SkipsWildcardFirstDep(t *testing.T) {
	chart := NewChart("wrapper").
		Version("1.0.0").
		Dep("a", "*", "file://a").
		Dep("b", "2.0.0", "oci://z")
	require.Equal(t, "2.0.0", chart.PrimaryDependencyVersion())
}

func TestChart_PrimaryDependencyVersion_NameMatchSkipsWildcard(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0").
		Dep("app", "*", "file://charts/app").
		Dep("other", "5.0.0", "oci://x")
	require.Equal(t, "5.0.0", chart.PrimaryDependencyVersion())
}

func TestChart_Values_Raw(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0").
		Values("replicaCount: 2\nimage:\n  tag: \"1.0.0\"\n")

	val, err := chart.GetValue("replicaCount")
	require.NoError(t, err)
	require.Equal(t, "2", val)

	val, err = chart.GetValue("image.tag")
	require.NoError(t, err)
	require.Equal(t, "1.0.0", val)
}

func TestChart_SetValue(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0").
		Values("apps:\n  nginx:\n    version: \"4.11.8-dev\"\n").
		SetValue("apps.ingress-nginx.version", "4.11.9-dev")

	val, err := chart.GetValue("apps.ingress-nginx.version")
	require.NoError(t, err)
	require.Equal(t, "4.11.9-dev", val)
}

func TestChart_SetValue_CreatesPath(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0").
		SetValue("new.nested.key", "value")

	val, err := chart.GetValue("new.nested.key")
	require.NoError(t, err)
	require.Equal(t, "value", val)
}

func TestChart_GetValue_NoValues(t *testing.T) {
	chart := NewChart("app").Version("1.0.0")
	_, err := chart.GetValue("any.key")
	require.Error(t, err)
}

func TestChart_GetValue_KeyNotFound(t *testing.T) {
	chart := NewChart("app").Version("1.0.0").Values("key: value\n")
	_, err := chart.GetValue("nonexistent")
	require.Error(t, err)
}

func TestChart_SaveTo(t *testing.T) {
	repo := Init(t.TempDir()).
		Commit("init", "placeholder", "x")
	require.NoError(t, repo.Err)

	chart := NewChart("ingress-nginx").
		Version("4.11.8-dev").
		Dep("ingress-nginx", "4.11.8", "https://kubernetes.github.io/ingress-nginx").
		Values("replicaCount: 2\n")

	chart.repoRoot = repo.Path()
	err := chart.SaveTo("apps/demo/ingress-nginx/dev")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(repo.Path(), "apps/demo/ingress-nginx/dev/Chart.yaml"))
	require.FileExists(t, filepath.Join(repo.Path(), "apps/demo/ingress-nginx/dev/values.yaml"))
}

func TestChart_SaveTo_NoValues(t *testing.T) {
	repo := Init(t.TempDir()).
		Commit("init", "placeholder", "x")
	require.NoError(t, repo.Err)

	chart := NewChart("app").Version("1.0.0")
	chart.repoRoot = repo.Path()
	err := chart.SaveTo("charts/app/dev")
	require.NoError(t, err)

	require.FileExists(t, filepath.Join(repo.Path(), "charts/app/dev/Chart.yaml"))
	require.NoFileExists(t, filepath.Join(repo.Path(), "charts/app/dev/values.yaml"))
}

func TestLoadChart_AndSave(t *testing.T) {
	repo := Init(t.TempDir()).
		CommitFS("init", NewFS().
			Add("apps/demo/ingress-nginx/dev",
				NewChart("ingress-nginx").
					Version("4.11.8-dev").
					Dep("ingress-nginx", "4.11.8", "https://kubernetes.github.io/ingress-nginx").
					Values("replicaCount: 2\n"),
			),
		)
	require.NoError(t, repo.Err)

	chart, err := repo.LoadChart("apps/demo/ingress-nginx/dev")
	require.NoError(t, err)

	require.Equal(t, "ingress-nginx", chart.GetName())
	require.Equal(t, "4.11.8-dev", chart.GetVersion())
	require.Equal(t, "4.11.8", chart.GetDepVersion("ingress-nginx"))

	val, err := chart.GetValue("replicaCount")
	require.NoError(t, err)
	require.Equal(t, "2", val)

	chart.Version("4.11.9-dev")
	require.NoError(t, chart.Save())

	reloaded, err := repo.LoadChart("apps/demo/ingress-nginx/dev")
	require.NoError(t, err)
	require.Equal(t, "4.11.9-dev", reloaded.GetVersion())
}

func TestLoadChart_NotFound(t *testing.T) {
	repo := Init(t.TempDir()).
		Commit("init", "a.txt", "hello")
	require.NoError(t, repo.Err)

	_, err := repo.LoadChart("nonexistent")
	require.Error(t, err)
}

func TestFS_File(t *testing.T) {
	fs := NewFS().
		File("a.txt", "hello").
		File("b.txt", "world")

	files := fs.files()
	require.Equal(t, "hello", files["a.txt"])
	require.Equal(t, "world", files["b.txt"])
}

func TestFS_Add_Chart(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0-dev").
		Dep("app", "1.0.0", "oci://registry/charts").
		Values("key: value\n")

	fs := NewFS().Add("charts/app/dev", chart)

	files := fs.files()
	require.Contains(t, files, "charts/app/dev/Chart.yaml")
	require.Contains(t, files, "charts/app/dev/values.yaml")
	require.Contains(t, files["charts/app/dev/Chart.yaml"], "name: app")
}

func TestFS_Add_ChartWithoutValues(t *testing.T) {
	chart := NewChart("app").Version("1.0.0-dev")

	fs := NewFS().Add("charts/app/dev", chart)
	files := fs.files()

	require.Contains(t, files, "charts/app/dev/Chart.yaml")
	require.NotContains(t, files, "charts/app/dev/values.yaml")
}

func TestChart_MarshalChartYAML(t *testing.T) {
	chart := NewChart("ingress-nginx").
		Version("4.11.8-dev").
		Dep("ingress-nginx", "4.11.8", "https://kubernetes.github.io/ingress-nginx")

	yamlStr := chart.marshalChartYAML()

	require.Contains(t, yamlStr, "apiVersion: v2")
	require.Contains(t, yamlStr, "name: ingress-nginx")
	require.Contains(t, yamlStr, "version: 4.11.8-dev")
	require.Contains(t, yamlStr, "type: application")
	require.Contains(t, yamlStr, "version: 4.11.8")
	require.Contains(t, yamlStr, "repository: https://kubernetes.github.io/ingress-nginx")
}

func TestChart_Save_WithoutDir_ReturnsError(t *testing.T) {
	chart := NewChart("app").Version("1.0.0")
	require.Error(t, chart.Save())
}

func TestChart_SetValue_PreservesExisting(t *testing.T) {
	chart := NewChart("app").
		Version("1.0.0").
		Values("existing: keep\nnested:\n  a: 1\n").
		SetValue("nested.b", "2")

	val, err := chart.GetValue("existing")
	require.NoError(t, err)
	require.Equal(t, "keep", val)

	val, err = chart.GetValue("nested.a")
	require.NoError(t, err)
	require.Equal(t, "1", val)

	val, err = chart.GetValue("nested.b")
	require.NoError(t, err)
	require.Equal(t, "2", val)
}

func TestRewriteChartYAMLVersion_PreservesComments(t *testing.T) {
	raw := []byte("apiVersion: v2\nname: fluent-bit\n# This is an important comment\nversion: 0.55.0-dev\ntype: application\n")
	result, err := rewriteChartYAMLVersion(raw, "0.55.0-stage")
	require.NoError(t, err)
	require.Contains(t, result, "# This is an important comment")
	require.Contains(t, result, "version: 0.55.0-stage")
	require.NotContains(t, result, "0.55.0-dev")
}

func TestRewriteChartYAMLVersion_PreservesBlankLines(t *testing.T) {
	raw := []byte("apiVersion: v2\nname: fluent-bit\ndescription: Fluent Bit DaemonSet for Kubernetes logging\ntype: application\nversion: 0.55.0-dev\n\ndependencies:\n  - name: fluent-bit\n    repository: oci://ghcr.io/fluent/helm-charts\n    version: 0.55.0\n")
	result, err := rewriteChartYAMLVersion(raw, "0.55.0-stage")
	require.NoError(t, err)
	require.Contains(t, result, "version: 0.55.0-stage")
	require.Contains(t, result, "description: Fluent Bit DaemonSet for Kubernetes logging")
	require.Contains(t, result, "\n\ndependencies:")
}

func TestRewriteChartYAMLVersion_PreservesDescription(t *testing.T) {
	raw := []byte("apiVersion: v2\nname: fluent-bit\ndescription: Fluent Bit DaemonSet\ntype: application\nversion: 0.55.0-dev\n")
	result, err := rewriteChartYAMLVersion(raw, "0.55.0-stage")
	require.NoError(t, err)
	require.Contains(t, result, "description: Fluent Bit DaemonSet")
	require.Contains(t, result, "version: 0.55.0-stage")
}

func TestRewriteChartYAMLVersion_PreservesFieldOrder(t *testing.T) {
	raw := []byte("apiVersion: v2\nname: ingress-nginx\ntype: application\nversion: 4.11.8-dev\ndependencies:\n  - name: ingress-nginx\n    repository: https://kubernetes.github.io/ingress-nginx\n    version: 4.11.8\n")
	result, err := rewriteChartYAMLVersion(raw, "4.11.8-stage")
	require.NoError(t, err)

	nameIdx := strings.Index(result, "name: ingress-nginx")
	typeIdx := strings.Index(result, "type: application")
	versionIdx := strings.Index(result, "version: 4.11.8-stage")
	depsIdx := strings.Index(result, "dependencies:")

	require.Greater(t, typeIdx, nameIdx, "type should come after name")
	require.Greater(t, versionIdx, typeIdx, "version should come after type")
	require.Greater(t, depsIdx, versionIdx, "dependencies should come after version")
}

func TestSave_PreservesFormatting(t *testing.T) {
	rawChartYAML := "apiVersion: v2\nname: fluent-bit\ndescription: Fluent Bit DaemonSet for Kubernetes logging\ntype: application\nversion: 0.55.0-dev\n\ndependencies:\n  - name: fluent-bit\n    repository: oci://ghcr.io/fluent/helm-charts\n    version: 0.55.0\n"

	repo := Init(t.TempDir()).
		CommitFS("init", NewFS().
			File("apps/cluster-infra/fluent-bit/dev/Chart.yaml", rawChartYAML),
		)
	require.NoError(t, repo.Err)

	chart, err := repo.LoadChart("apps/cluster-infra/fluent-bit/dev")
	require.NoError(t, err)
	require.Equal(t, "0.55.0-dev", chart.GetVersion())

	chart.Version("0.55.0-stage")
	require.NoError(t, chart.Save())

	saved, err := os.ReadFile(filepath.Join(repo.Path(), "apps/cluster-infra/fluent-bit/dev/Chart.yaml"))
	require.NoError(t, err)

	expected := "apiVersion: v2\nname: fluent-bit\ndescription: Fluent Bit DaemonSet for Kubernetes logging\ntype: application\nversion: 0.55.0-stage\n\ndependencies:\n  - name: fluent-bit\n    repository: oci://ghcr.io/fluent/helm-charts\n    version: 0.55.0\n"
	require.Equal(t, expected, string(saved))
}

func TestRenderChartYAML_FallbackForNewChart(t *testing.T) {
	chart := NewChart("app").Version("1.0.0").Dep("app", "1.0.0", "oci://registry/charts")
	result := chart.renderChartYAML()
	require.Contains(t, result, "name: app")
	require.Contains(t, result, "version: 1.0.0")
}

func TestCommitFS_ThenLoadChart(t *testing.T) {
	repo := Init(t.TempDir()).
		CommitFS("init", NewFS().
			File(".hydra-ci.yaml", "ci:\n  rootAppsPath: apps\n").
			Add("apps/demo/ingress-nginx/dev",
				NewChart("ingress-nginx").
					Version("4.11.8-dev").
					Dep("ingress-nginx", "4.11.8", "https://kubernetes.github.io/ingress-nginx").
					Values("replicaCount: 2\n"),
			).
			Add("apps/demo/root/dev",
				NewChart("demo").
					Version("200.22.0-dev").
					Dep("libchart", "1.0.0", "file://../../../../shared/libchart/dev").
					Values("apps:\n  ingress-nginx:\n    version: \"4.11.8-dev\"\n"),
			),
		)
	require.NoError(t, repo.Err)

	root, err := repo.LoadChart("apps/demo/root/dev")
	require.NoError(t, err)
	require.Equal(t, "demo", root.GetName())

	root.SetValue("apps.ingress-nginx.version", "4.11.9-dev").
		Version("200.23.0-dev")
	require.NoError(t, root.Save())

	reloaded, err := repo.LoadChart("apps/demo/root/dev")
	require.NoError(t, err)
	require.Equal(t, "200.23.0-dev", reloaded.GetVersion())

	val, err := reloaded.GetValue("apps.ingress-nginx.version")
	require.NoError(t, err)
	require.Equal(t, "4.11.9-dev", val)

	// Ensure .hydra-ci.yaml still exists
	_, err = os.ReadFile(filepath.Join(repo.Path(), ".hydra-ci.yaml"))
	require.NoError(t, err)
}
