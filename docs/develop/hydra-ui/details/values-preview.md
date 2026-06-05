# Values Preview

## Overview

The Values Preview system provides YAML content viewing with syntax highlighting, full-text search, and interactive YAML breadcrumb navigation. It is used across three locations in the UI:

1. **Entity Detail Page → Values Tab** — merged values for a specific entity
2. **Chart Page → Values Tab (Tree mode)** — raw content of individual value files (split-panel preview, right pane)
3. **Chart Page → Values Tab (Preview mode)** — fully merged values for the selected chart

All three locations share the same reusable components from `SearchableCodeViewer.tsx`.

## Architecture

```text
SearchableCodeViewer
├── Header bar
│   ├── headerLeft slot (description text, file path, etc.)
│   ├── Search input (synced via global codeSearchTerm)
│   └── Match count + Prev/Next buttons
├── YAML Breadcrumb bar
│   └── Clickable path segments (e.g. global . hydra . cluster)
└── CodeViewer (CodeMirror 6)
    ├── YAML syntax highlighting
    ├── Go template highlighting (for .gotpl files)
    ├── Search integration (highlight matches)
    ├── Mouse hover tracking (for breadcrumb)
    └── Scroll-to-line (for breadcrumb clicks)
```

## Components

### `CodeViewer`

Low-level read-only CodeMirror 6 editor wrapper.

**Props:**

- `content: string` — document text
- `language: "yaml" | "gotpl"` — syntax mode
- `isDark: boolean` — light/dark theme
- `searchTerm?: string` — drives CodeMirror's built-in search highlighting
- `onMatchCount?: (count: number) => void` — reports total search matches
- `handleRef?: (handle: CodeViewerHandle | null) => void` — exposes imperative actions
- `onHoverLine?: (line: number) => void` — reports the 1-based line number under the mouse cursor; `0` means the mouse left the editor

**Handle API (`CodeViewerHandle`):**

- `findNext()` — jump to next search match
- `findPrevious()` — jump to previous search match
- `scrollToLine(line)` — scroll to and select the given line (centered)

**Mouse hover detection:** Uses `EditorView.domEventHandlers` with `mousemove`. To ensure the breadcrumb only reacts to actual text (not empty space on a line), the handler:

1. Gets the document position via `posAtCoords()`
2. Checks `coordsAtPos(line.to)` to verify the mouse X is within the rendered text bounds
3. Checks `coordsAtPos(line.from)` to exclude the gutter area
4. Verifies the character at the cursor column is non-whitespace

On `mouseleave`, reports `0` so pending updates can be cancelled.

### `SearchableCodeViewer`

High-level wrapper around `CodeViewer` that adds a search bar and YAML breadcrumb.

**Props:**

- `content: string | null` — YAML content (null = not yet loaded)
- `language?: "yaml" | "gotpl"` — syntax mode (default: `"yaml"`)
- `isDark: boolean` — theme
- `searchTerm: string` — global search term (shared across all viewers via state)
- `onSearchTermChange?: (term: string) => void` — updates global search term
- `loading?: boolean` — shows loading indicator
- `error?: string | null` — shows error message
- `headerLeft?: React.ReactNode` — custom content left of the search box

**Breadcrumb:** Only displayed for `language === "yaml"`. Shows the YAML key hierarchy at the mouse position. Each segment is clickable and scrolls the editor to that key's line. The breadcrumb persists when the mouse leaves the editor, so segments remain clickable.

## YAML Breadcrumb

### Parsing (`parseYamlLineKeys`)

Scans every line of the YAML content and extracts key entries:

```text
Line: "    cluster: prod"
  → indent: 4, key: "cluster", line: 42
```

For array items (`- key: value`), the effective indent includes the `-` prefix width (+2), so that array item keys and their sibling keys share the same indent level:

```text
Line: "  - host: example.com"    → indent: 4 (2 + 2), key: "host"
Line: "    port: 443"            → indent: 4,           key: "port"
```

This correctly identifies `host` and `port` as siblings under the same parent.

Skipped lines: blank lines, comments (`#`), value-only lines (no `:`), quoted keys.

### Path resolution (`buildBreadcrumb`)

Given parsed line keys and a hover line number:

1. Find the last key entry at or before the hover line
2. Walk backwards through the entries, collecting each entry with a strictly smaller indent
3. Stop when indent 0 is reached

Result: an ordered array of `BreadcrumbSegment` objects, each with a `label` and source `line` number.

**Example:** For content:

```yaml
global:
  hydra:
    cluster: prod
```

Hovering over line 3 produces: `[{label: "global", line: 1}, {label: "hydra", line: 2}, {label: "cluster", line: 3}]`

Rendered as: `global . hydra . cluster` with each segment clickable.

### Hover behaviour

- Updates **only** when the mouse is directly over a non-whitespace character (not on empty space to the right of text)
- **Persists** when the mouse moves to whitespace, the breadcrumb bar, or outside the editor — so segments remain clickable
- **Resets** when content changes (e.g. switching to a different file)

## Search

### Global search term

A single `codeSearchTerm` string is stored in the application state (`HydraUiState` in `state.ts`) and persisted in `localStorage`. All `SearchableCodeViewer` instances share this term, so searching in one viewer automatically highlights matches in others.

### Search flow

```text
User types in search box
        │
        ▼
onSearchTermChange(term)  →  App.tsx setState  →  localStorage
        │
        ▼
searchTerm prop flows to all SearchableCodeViewer instances
        │
        ▼
CodeViewer receives searchTerm
        │
        ▼
setSearchQuery effect  →  CodeMirror highlights matches
        │
        ▼
onMatchCount(count)  →  SearchableCodeViewer shows "N matches"
```

### Navigation

- **Next match:** `findNext()` via CodeMirror search API
- **Previous match:** `findPrevious()` via CodeMirror search API
- Both buttons are disabled when match count is 0

## Go Template Highlighting

For template files (`language: "gotpl"`), a custom CodeMirror `ViewPlugin` provides syntax highlighting on top of the YAML base language:

- **Delimiters** (`{{ }}`, `{{- -}}`) — purple/bold
- **Keywords** (`if`, `else`, `end`, `range`, `with`, etc.) — red/bold
- **Functions** (`toYaml`, `indent`, `default`, etc.) — purple
- **Variables** (`.Values.x`, `$var`) — blue
- **Strings** — dark blue
- **Comments** (`{{/* */}}`) — grey/italic

Both light and dark theme variants are defined.

## Git Repository Integration

hydra-go exports git repository metadata in `hydra.yaml` as top-level fields:

| Field           | Example                       | Description                                     |
| --------------- | ----------------------------- | ----------------------------------------------- |
| `gitRemote`     | `git@gitlab.com:org/repo.git` | Git remote origin URL                           |
| `gitRepoPrefix` | `environments/production/`    | Path from repo root to context parent directory |
| `gitBranch`     | `main`                        | Current branch name                             |

### Tree Structure

When `gitRepoPrefix` is present, the Value Files tree prepends it to all exported file paths so the tree mirrors the full git repository directory structure. File loading still uses the original relative path (prefix is stripped before calling `ClusterLoader.loadValueFile`).

### Per-File Git Links

Each exported value file in the tree shows a clickable external-link icon that opens the file directly in the git web UI (GitLab or GitHub).

URL construction:

- GitLab: `{webUrl}/-/blob/{branch}/{repoPrefix}{filePath}`
- GitHub: `{webUrl}/blob/{branch}/{repoPrefix}{filePath}`

The git remote URL is converted to an HTTPS web URL via `gitRemoteToWebUrl()`:

- `git@gitlab.com:org/repo.git` → `https://gitlab.com/org/repo`
- `ssh://git@gitlab.com/org/repo.git` → `https://gitlab.com/org/repo`
- `https://gitlab.com/org/repo.git` → `https://gitlab.com/org/repo`

## Value File Sources

### Exported value files (from hydra-go)

Loaded via `ClusterLoader.loadValueFile(relativePath)` from the `values/files/` directory in the cluster dump. Types:

| Type      | Prio | Tag colour | Hex (light / dark)    | Description                                                                     |
| --------- | ---- | ---------- | --------------------- | ------------------------------------------------------------------------------- |
| `cluster` | 1    | rose       | `#e11d48` / `#fb7185` | Values specific to this target cluster                                          |
| `context` | 2    | amber      | `#f59e0b` / `#fbbf24` | Values shared across all apps in this ArgoCD installation                       |
| `group`   | 3    | green      | `#22c55e` / `#4ade80` | Values shared across all apps managed by all ArgoCD installations in this group |
| `app`     | 4    | blue       | `#3b82f6` / `#60a5fa` | Values specific to this application                                             |

### Chart archive values (from .tgz)

Loaded via `ClusterLoader.loadChartFile(chartName, filePath)` from the chart's `.tgz` archive. These are the `values.yaml` files bundled inside Helm charts, including nested sub-chart dependencies. Sub-charts are stored as unpacked directories within the outer `.tgz` (not as nested archives), so `listValuesFilesFromTgz()` finds values from all dependency levels including transitive ones.

| Type           | Tag colour | Hex (light / dark)    | Description                                     |
| -------------- | ---------- | --------------------- | ----------------------------------------------- |
| `chart values` | violet     | `#8b5cf6` / `#a78bfa` | Default values bundled inside the chart archive |

Discovery: `ClusterLoader.listChartValuesFiles(chartName)` uses `listValuesFilesFromTgz()` to scan the `.tgz` archive for all files named `values.yaml`.

### Merged values

Loaded via `ClusterLoader.loadMergedValues(appId)` from the `values/merged/` directory. Represents the final result of the entire values hierarchy as computed by hydra-go.

## Values Provenance

The Values Provenance system analyses merged values to determine, for each YAML key, which source file it originates from. It provides visual annotations in the CodeMirror editor (gutter markers, line decorations, hover tooltips) and works in both the Values tab (merged values) and the Value Files tab (individual source files).

### Colour Scheme

Each source type has a unique colour used consistently across treeview tags, gutter markers, and the legend. Colours follow a warm→cool gradient matching the priority hierarchy (warm = highest priority):

| Source         | Prio | Colour name | Hex (light / dark)    | Meaning                                                         |
| -------------- | ---- | ----------- | --------------------- | --------------------------------------------------------------- |
| Cluster        | 1    | rose        | `#e11d48` / `#fb7185` | Specific to the target cluster (highest priority)               |
| Context        | 2    | amber       | `#f59e0b` / `#fbbf24` | Shared across all apps in one ArgoCD installation               |
| Group          | 3    | green       | `#22c55e` / `#4ade80` | Shared across all ArgoCD installations in this group            |
| App            | 4    | blue        | `#3b82f6` / `#60a5fa` | Specific to this application                                    |
| Chart Defaults | 5+   | violet      | `#8b5cf6` / `#a78bfa` | Default values from the Helm chart's bundled `values.yaml`      |
| Hydra Defaults | 99   | slate       | `#475569` / `#94a3b8` | Hydra fallback values (e.g. `global.hydra.*`) — lowest priority |

Two additional statuses override the source colour:

| Status      | Colour | Hex                   | Meaning                                                          |
| ----------- | ------ | --------------------- | ---------------------------------------------------------------- |
| Unnecessary | grey   | `#9ca3af` / `#6b7280` | Override with an identical value — the line could be removed     |
| New key     | red    | `#ef4444` / `#f87171` | Key does not exist in any lower-priority source (chart defaults) |

### Provenance Priority

Sources are compared in ascending priority order. The highest-priority source that defines a key "wins":

1. **Cluster** (highest — specific to the target cluster)
2. **Context** — shared across all apps in one ArgoCD installation
3. **Group** — shared across all apps in all ArgoCD installations of this group
4. **App** — specific to this application
5. **Chart Defaults** — from `.tgz` `values.yaml` (shallower nesting = higher priority)
6. **Hydra Defaults** (lowest — fallback values from `infra_library`)

Within chart defaults, nesting depth determines sub-priority: a root chart's `values.yaml` wins over a sub-chart's `values.yaml`. The sort key is computed by `getSourceSortKey(sourceType, sourceFile)` which counts `/charts/` segments in the file path to determine depth.

The "Prio" column in the treeview shows the ArgoCD priority number as read from the Application manifest. Only tree nodes that have an associated ArgoCD source (file-based or inline) display a priority. Nodes without ArgoCD source information (e.g. chart tgz archives, hydra defaults) have no priority number. The priority reflects the actual ArgoCD Helm value resolution order: lowest number = lowest priority (overridden first), highest number = highest priority (wins).

### Alias Resolution

Helm charts can declare dependency aliases in `Chart.yaml`:

```yaml
dependencies:
  - name: altinity-clickhouse-operator
    alias: operator
    version: 0.23.7
```

In user-provided value files, the alias is used (`operator.metrics.image.repository`), but the merged output uses the original chart name (`altinity-clickhouse-operator.metrics.image.repository`). The provenance system resolves this mapping by:

1. Extracting `Chart.yaml` from the chart's `.tgz` archive via `ClusterLoader.loadChartYaml()`
2. Parsing the `dependencies` array — if a dependency has an `alias` field, map `alias → name`; otherwise `name → name`
3. Recursively parsing `Chart.yaml` at each sub-chart level to handle transitive dependencies (e.g. `charts/operator/Chart.yaml`)

The resulting `aliasMap` is a `Map<string, string>` mapping alias prefixes to original chart names. During provenance matching, key path prefixes from source files are translated through this map before comparison with merged values keys.

### Analysis Algorithm (`buildProvenance`)

For each dot-path key in the merged values:

1. Look up the key in each source (chart defaults, group, context, cluster, app), applying alias prefix translation
2. Record which sources define the key and with what value
3. The highest-priority source becomes the `sourceType` / `sourceFile`
4. `isUnnecessary = true` if the winning source's value equals the value from a lower-priority source (the override had no effect)
5. `isNew = true` if the key does not exist in any lower-priority source (e.g. an app-level key with no chart default)
6. `overrides` array lists all other sources that also define this key

### Line Mapping

`mapProvenanceToLines()` uses the existing `parseYamlLineKeys()` function to associate each YAML line with a dot-path key, then looks up the provenance for that key. Lines without a key association (comments, blank lines, block scalar content) receive `null` provenance.

### Editor Annotations

When `lineProvenance` data is provided, three CodeMirror extensions are added:

1. **Gutter markers** (`provenanceGutter`): A narrow gutter column to the left of line numbers. Each line with provenance data shows a 10×10px coloured square matching the source type colour. Unnecessary lines show grey; new-key lines show red.

2. **Line decorations** (`provenanceLineHighlight`): `Decoration.line()` applied to the entire line:
   - Unnecessary overrides: reduced opacity (0.45) on the line content
   - New keys: subtle red background tint

3. **Hover tooltips**: Shown on both the **gutter marker** (custom DOM tooltip via `mouseenter`/`mouseleave`) and the **line content** (CodeMirror `hoverTooltip()`), displaying:
   - Source file path and type
   - "Also defined in:" list with values from other sources
   - Status text ("Unnecessary override — same value as chart defaults" or "New key — not defined in chart defaults")

### Parent Provenance Propagation

Object keys (structural YAML keys that have children but no leaf value of their own) inherit the source type colour of their children when **all** direct children share the same source type. This is computed bottom-up by `propagateParentProvenance()`:

1. Group lines by their parent key path (all but the last dot-segment)
2. For each parent that also has a line entry, check if all children share the same `sourceType`
3. If uniform, override the parent's `sourceType` to match the children
4. Iterate until stable (handles deeply nested uniform subtrees)

This ensures that structural keys like `operator:` or `global:` display the correct colour when all their children come from the same source.

### TreeTable Checkbox Columns

The Values tab's Tree mode provides per-file filtering through three checkbox columns in the TreeTable:

| Column                                 | State field                                     | Description                                                               |
| -------------------------------------- | ----------------------------------------------- | ------------------------------------------------------------------------- |
| **Visible** (eye icon)                 | `valuesDisabledFiles` / `valuesDisabledSources` | Controls whether lines from this source appear in the merged preview      |
| **Errors** (exclamation-triangle icon) | `valuesShowErrors`                              | Controls whether "new key" markers are shown for this source              |
| **Unneeded** (minus-circle icon)       | `valuesShowUnnecessary`                         | Controls whether "unnecessary override" markers are shown for this source |

For folder nodes, checkboxes display tri-state (all/some/none checked) computed from children. Toggling a folder propagates to all descendant file nodes.

### Source Filtering

When files/sources are hidden via the "Visible" checkbox, the system computes which lines to hide using `computeHiddenLines()`:

1. **Direct hiding:** Lines whose `sourceType` is in the `disabledSources` set or whose `sourceFile` is in `disabledFiles` are marked hidden
2. **Parent collapse:** If ALL children of a structural parent key are hidden, the parent is hidden too (iterates until stable)
3. **Orphan cleanup:** Non-key lines (comments, blank lines) between two hidden key lines are also hidden

The hidden lines are then removed from the YAML content and the `lineProvenance` array is remapped to the new (compressed) line numbers via `filterContentAndProvenance()`. The filtered content and remapped provenance are passed to `SearchableCodeViewer`, so the editor only displays the visible lines with correct provenance annotations.

**State management:** `disabledSources` / `disabledFiles` are `Set` values derived from persisted state arrays in `ChartPage`. Toggling checkboxes in Tree mode triggers a `useMemo` recomputation of `filteredMergedContent` and `filteredMergedProvenance` used by Preview mode.

## Data Flow

```text
hydra-go cluster dump
├── hydra.yaml
│   ├── valueFiles[]         metadata: path, type, appId
│   ├── appValues[]          metadata: appId → merged file path
│   ├── gitRemote            git remote origin URL
│   ├── gitRepoPrefix        path from repo root to context parent
│   └── gitBranch            current branch name
├── values/
│   ├── files/               raw value files from GitOps repo
│   └── merged/              merged values per app
└── charts/
    └── *.tgz                chart archives with embedded values.yaml

        │
        ▼

ClusterLoader
├── loadValueFile(path)          → fetch from values/files/
├── loadMergedValues(appId)      → fetch from values/merged/
├── loadChartFile(chart, path)   → extract from .tgz cache
└── listChartValuesFiles(chart)  → scan .tgz for values.yaml
```

## Files

| File                                      | Description                                                                                                                                                                                                                                                                              |
| ----------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/components/SearchableCodeViewer.tsx` | `CodeViewer`, `SearchableCodeViewer`, breadcrumb helpers, provenance gutter/tooltips                                                                                                                                                                                                     |
| `src/components/ChartPage.tsx`            | Unified Values tab with tree/preview toggle, TreeTable with checkbox columns, provenance loading                                                                                                                                                                                         |
| `src/components/EntityPage.tsx`           | Entity Values tab (`ValuesTabContent`)                                                                                                                                                                                                                                                   |
| `src/valuesProvenance.ts`                 | `flattenYaml`, `buildAliasMap`, `buildProvenance`, `mapProvenanceToLines`, `propagateParentProvenance`, `computeHiddenLines`, `filterContentAndProvenance`                                                                                                                               |
| `src/clusterLoader.ts`                    | `loadValueFile`, `loadMergedValues`, `loadChartFile`, `loadChartYaml`, `listChartValuesFiles`                                                                                                                                                                                            |
| `src/tgzExtract.ts`                       | `extractFileFromTgz`, `listValuesFilesFromTgz`                                                                                                                                                                                                                                           |
| `src/chartPageLogic.ts`                   | `buildValueFilesTree` — flat file list → nested tree; `gitRemoteToWebUrl`, `gitFileUrl` — git URL helpers                                                                                                                                                                                |
| `src/argocdValuesOrder.ts`                | `extractValuesSourceOrder` — parses ArgoCD Application manifest and returns ordered value sources with resolved repo paths; `resolveValueFilePath` — normalizes relative valueFile paths against chart source.path into absolute repo paths; `argocdSourceLabel`, `ARGOCD_SOURCE_COLORS` |
| `src/components/ArgocdAppView.tsx`        | `AppSpec`, `AppSource`, `AppSourceHelm` types — parsed ArgoCD Application spec including `valuesObject`, `parameters`                                                                                                                                                                    |
| `src/model.ts`                            | `HydraValueFile`, `HydraAppValues`, `HydraData` (incl. `gitRemote`, `gitRepoPrefix`, `gitBranch`) types                                                                                                                                                                                  |
| `src/state.ts`                            | `codeSearchTerm`, `valuesViewMode`, `valuesShowErrors` in `HydraUiState`                                                                                                                                                                                                                 |

## Tests

| Test File                                 | Coverage                                                                                                                                                                                                                                                                                                                                                                                |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/__tests__/yamlBreadcrumb.test.ts`    | `parseYamlLineKeys`, `buildBreadcrumb` — nested paths, siblings, arrays, comments                                                                                                                                                                                                                                                                                                       |
| `src/__tests__/tgzExtract.test.ts`        | `listValuesFilesFromTgz` — simple, empty, deeply nested archives                                                                                                                                                                                                                                                                                                                        |
| `src/__tests__/chartPageLogic.test.ts`    | `buildValueFilesTree` — directory hierarchy, sorting, repoPrefix; `gitRemoteToWebUrl` — SSH/HTTPS conversion; `gitFileUrl` — GitLab/GitHub URL construction                                                                                                                                                                                                                             |
| `src/__tests__/clusterLoader.test.ts`     | `loadValueFile`, `loadMergedValues`                                                                                                                                                                                                                                                                                                                                                     |
| `src/__tests__/valuesProvenance.test.ts`  | `flattenYaml`, `buildAliasMap`, `buildProvenance`, `mapProvenanceToLines`, `propagateParentProvenance`, `computeHiddenLines`, `filterContentAndProvenance`, `getSourceSortKey` — aliases, overrides, new keys, transitive deps, parent propagation, line filtering, priority ordering (full chain: cluster > context > group > app > chart > hydra), chart nesting depth sub-priorities |
| `src/__tests__/argocdValuesOrder.test.ts` | `resolveValueFilePath` — simple relative, sibling top-level dir via `../../../../`, trailing `..`, absolute paths, shallow chart path; `extractValuesSourceOrder` — invalid YAML, missing spec, valueFiles with resolved paths, valuesObject priority, values (string), parameters, multi-source apps, real-world manifest with path resolution; `argocdSourceLabel`                    |

## Related Documentation

- `../../shared/details/values.md` — how Helm values are composed (valueFiles sources, valuesObject composition)
