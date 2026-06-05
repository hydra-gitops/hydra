# Values Preview

The Values Preview system provides YAML content viewing with syntax highlighting, full-text search, interactive YAML breadcrumb navigation, and provenance annotations. It is used across entity detail pages and the chart page values tab.

## Key Concepts

- **SearchableCodeViewer** — High-level wrapper around CodeMirror 6 with search bar and YAML breadcrumb
- **YAML breadcrumb** — Clickable path segments showing the key hierarchy at mouse position; persists when mouse leaves editor
- **Global search term** — Shared `codeSearchTerm` across all viewer instances via application state
- **Values provenance** — Source-file origin annotations with coloured gutter markers, line decorations, and hover tooltips
- **Provenance priority** — Cluster > Context > Group > App > Chart Defaults > Hydra Defaults
- **Alias resolution** — Helm dependency aliases resolved via `Chart.yaml` parsing for correct provenance mapping
- **Source filtering** — Checkbox columns (Visible, Errors, Unneeded) in tree mode control preview annotations
- **Go template highlighting** — Custom ViewPlugin for `{{}}` delimiters, keywords, functions, variables

## Source Files

| File                                      | Description                                                                          |
| ----------------------------------------- | ------------------------------------------------------------------------------------ |
| `src/components/SearchableCodeViewer.tsx` | `CodeViewer`, `SearchableCodeViewer`, breadcrumb helpers, provenance extensions      |
| `src/valuesProvenance.ts`                 | Provenance analysis: `buildProvenance`, `mapProvenanceToLines`, `computeHiddenLines` |
| `src/components/ChartPage.tsx`            | Values tab with tree/preview toggle and checkbox columns                             |
| `src/clusterLoader.ts`                    | Value file and merged values loading                                                 |
| `src/chartPageLogic.ts`                   | `buildValueFilesTree`, git URL helpers                                               |

→ **Full details:** [details/values-preview.md](details/values-preview.md)
