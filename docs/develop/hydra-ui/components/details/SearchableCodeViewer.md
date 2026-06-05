# SearchableCodeViewer

**File:** `src/components/SearchableCodeViewer.tsx`

## Function

A feature-rich code viewer built on CodeMirror 6. Provides:

- **Syntax highlighting** — YAML and Go template languages
- **Search** — Built-in Ctrl+F search with match highlighting
- **YAML breadcrumb** — Shows the current cursor position as a dot-separated key path; clicking a segment scrolls to that key
- **Provenance gutter** — Coloured gutter markers indicating the source of each YAML line (cluster, context, group, app, chart defaults)
- **Hover tooltips** — Shows provenance details, override chain, and value differences on hover
- **Line decorations** — Visual markers for unnecessary overrides, new keys, global clones, and overridden values

The simpler `CodeViewer` variant provides basic code viewing without provenance features.

## Exports

- `SearchableCodeViewer` — Full-featured viewer with provenance support
- `CodeViewer` / `CodeViewerHandle` — Simpler viewer with imperative handle for content updates
- `parseYamlLineKeys(content)` — Parses YAML content into line/key/indent tuples
- `buildBreadcrumb(lineKeys, line)` — Builds a breadcrumb path for a given line

## Props (SearchableCodeViewer)

| Prop             | Type                            | Description                                  |
| ---------------- | ------------------------------- | -------------------------------------------- |
| `content`        | `string \| null`                | YAML/template content to display             |
| `language`       | `"yaml" \| "gotpl"`             | Syntax highlighting language                 |
| `isDark`         | `boolean`                       | Current dark mode state                      |
| `loading`        | `boolean`                       | Show loading indicator                       |
| `error`          | `string \| null`                | Error message to display                     |
| `headerLeft`     | `React.ReactNode`               | Custom content for the header left area      |
| `lineProvenance` | `LineProvenance[]`              | Per-line provenance data for gutter/tooltips |
| `fileColorMap`   | `Map<string, ProvenanceColors>` | File → colour mapping for provenance display |

## Used by

- `src/components/EntityPage.tsx` — `CodeViewer` for manifest/template display
- `src/components/ValuesView.tsx` — `SearchableCodeViewer` for merged values preview with provenance; `CodeViewer` for individual file display
