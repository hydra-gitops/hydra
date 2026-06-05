package helm

import (
	"path/filepath"
	"slices"
	"strings"

	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

// ChartSourceTemplatesMultidoc returns Helm chart template sources as multi-document text with
// "# Source: <path>" headers (same style as helm template output), but bodies are unrendered.
// Dependency charts are included after the parent chart, sorted by chart name and template path.
// This includes templates from packaged dependencies (charts/*.tgz) after [chart.Charter] load.
// If pathPrefixes is non-empty, only template files whose Helm path name matches
// [displayPathMatchesAnySourcePrefix] for at least one prefix are included (OR).
func ChartSourceTemplatesMultidoc(charter chart.Charter, pathPrefixes []string) (string, error) {
	c, err := convertToV2Chart(charter)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	writeChartTemplatesRecursive(&b, c, pathPrefixes)
	return strings.TrimRight(b.String(), "\n"), nil
}

func writeChartTemplatesRecursive(b *strings.Builder, c *v2chart.Chart, pathPrefixes []string) {
	if c == nil {
		return
	}
	tmpl := append([]*common.File(nil), c.Templates...)
	slices.SortFunc(tmpl, func(a, b *common.File) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return 1
		case b == nil:
			return -1
		default:
			return strings.Compare(a.Name, b.Name)
		}
	})
	for _, f := range tmpl {
		if f == nil || f.Name == "" {
			continue
		}
		display := filepath.ToSlash(f.Name)
		if !displayPathMatchesAnySourcePrefix(display, pathPrefixes) {
			continue
		}
		b.WriteString("---\n# Source: ")
		b.WriteString(f.Name)
		b.WriteString("\n")
		b.Write(f.Data)
		if len(f.Data) == 0 || f.Data[len(f.Data)-1] != '\n' {
			b.WriteByte('\n')
		}
	}
	deps := append([]*v2chart.Chart(nil), c.Dependencies()...)
	slices.SortFunc(deps, func(a, b *v2chart.Chart) int {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return 1
		case b == nil:
			return -1
		default:
			return strings.Compare(a.Name(), b.Name())
		}
	})
	for _, d := range deps {
		writeChartTemplatesRecursive(b, d, pathPrefixes)
	}
}
