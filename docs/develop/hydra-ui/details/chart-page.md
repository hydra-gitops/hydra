# Chart Page

## Overview

The Chart Page is a dedicated view in the Hydra UI that displays Helm chart metadata for the currently filtered entities. It is accessible via the "Charts" button in the top navigation bar, or by clicking on a childApp/rootApp node in the FilesTreeView.

The page has five tabs:

- **Details** — Version overview, dependency tree (children), used-by tree (parents), Chart.yaml with syntax highlighting
- **Values** — Unified values view combining the file tree and merged values preview
- **Manifests** — Flat list of all rendered manifests (entities with `manifestPath`) from the filtered entities
- **Templates** — Tree view grouped by `templatePath`, with individual manifests as child nodes
- **App** — ArgoCD Application CR information (structured overview + full YAML manifest); only shown when a matching ArgoCD Application entity exists

## Navigation

- **Top Nav Button:** A "Charts" button (pi-list icon) is placed between the Graph and Theme buttons. Clicking it calls `setPage("charts")`.
- **Charts Sidebar Tab:** The sidebar has a dedicated "Charts" accordion tab (`ChartsTreeView`) that shows all charts with their dependencies. Clicking a chart name or navigable dependency opens the chart details page.
- **URL Hash:** The chart page is persisted as `page=charts` in the URL hash (e.g. `#my-cluster?page=charts`), making it a first-class page type alongside `list`, `graph`, `details`, and `settings`.
- **Inter-chart navigation:** Clicking a dependency or parent chart in the Details tab navigates to that chart via `onOpenChart`, which also creates a matching selection filter. For dependency charts without their own `appId`, the parent chart's `appId`s are used for the selection filter instead.

Note: The FilesTreeView no longer navigates to the chart page. Nodes with `chartInfo` only set a selection filter (list view). Chart navigation is handled exclusively through the Charts sidebar tab and the top nav button.

## Data Flow

```text
listFilteredEntities (from selectionSlot if active, else from active filter slots)
         │
         ▼
 getMatchingCharts(listFilteredEntities, allCharts)
         │
         ├── Collect appIds from filtered entities
         ├── Find charts matching those appIds
         ├── Group by chart name
         └── Merge versions (same name+version → merge appIds)
         │
         ▼
   GroupedChart[]                    Selected chart
         │                                │
         ├── length === 1: label           ▼
         ├── length > 1: dropdown   getParentCharts(name, allCharts)
         │                                │
         ▼                                ▼
   Chart selection              GroupedParentChart[]
```

When a chart is clicked in the sidebar, `handleOpenChart` sets a `selectionSlot` with `appId` filters (OR-combined for all appIds of that chart name). For dependency charts that don't have their own `appId` entries, the function finds parent charts that depend on the target and uses their `appId`s instead. This causes `listFilteredEntities` to contain only entities matching those appIds, so `getMatchingCharts()` returns the relevant charts. The dropdown is hidden when only one chart matches (a simple label is shown instead).

Additionally, if `selectedChartName` is set but the chart doesn't appear in the entity-filtered results, `ChartPage` adds it from the full chart list. This ensures dependency charts are always displayed when navigated to.

## Sub-Chart Export (hydra-go)

The `CollectChartInfo()` function in `hydra-go/core/commands/chart_info.go` exports not only the top-level app charts but also their loaded sub-chart dependencies recursively. Sub-charts are exported as additional `ChartModel` entries with empty `AppId`.

This allows the UI to resolve transitive dependency trees: if chart A depends on chart B, and B depends on C, the UI can show A → B → C because B and C are available as separate chart entries in the data.

Deduplication is handled via a `seen` map (key: `name@version`) to avoid exporting the same sub-chart multiple times.

## Tabs

### Details Tab

Shows for the selected chart:

- **Versions**: Each version with appVersion and appId tags
- **Dependencies**: PrimeReact `Tree` view with version → dependency hierarchy, expanded by default. Clickable dependencies navigate to that chart via `onOpenChart`.
- **Used by**: PrimeReact `Tree` view showing parent charts → versions → appIds, fully expanded by default. Clickable parent chart names navigate to that chart via `onOpenChart`.
- **Chart.yaml**: Content loaded from the `.tgz` archive via `ClusterLoader.loadChartYaml()`. Displayed using the `YamlHighlighter` component (CodeMirror 6 with YAML syntax highlighting, read-only mode, max-height 400px). The font size is controlled by the `editorFontSize` setting. The Chart.yaml content is loaded based on a stable chart name string (`chartYamlName`), not the filtered `selected` object. This ensures the content is **never affected by entity filters** — it persists even when the chart is temporarily filtered out of the dropdown, and does not re-load unnecessarily when filters change. For sub-charts that don't have their own `.tgz` archive, the loader falls back to extracting `{parentChart}/charts/{chartName}/Chart.yaml` from parent chart archives.

### Values Tab

The Values tab unifies the former "Value Files" and "Values" tabs into a single view with two modes, controlled by a toolbar toggle button (`valuesViewMode` in state):

#### Toolbar

A horizontal toolbar sits above the content area, showing:

- **Left:** Chart/app info (e.g. "Merged values for **chart-name** (appId)")
- **Right:** A segmented toggle button to switch between **Tree** and **Preview** modes
- **Right:** A "Hide Global Clones" toggle button

#### Tree Mode (`valuesViewMode: "tree"`)

Split-panel layout:

- **Left pane (55%):** PrimeReact `TreeTable` showing the GitOps repository directory structure, chart archive values, and Hydra fallback values. Each file node displays a custom-coloured tag indicating the value category (see `../../shared/details/values.md` for the full list):
  - `group values` — green (`#22c55e`)
  - `context values` — amber (`#f59e0b`)
  - `cluster values` — rose (`#e11d48`)
  - `app values` — cyan (`#06b6d4`)
  - `root app values` — blue (`#3b82f6`)
  - `root app defaults` — indigo (`#6366f1`)
  - `upstream values` — violet (`#8b5cf6`)
  - `hydra values` — slate (`#475569`)

  The tree has three additional checkbox columns per file node:

  | Column       | Icon                 | State field                                                | Effect on Preview mode                                          |
  | ------------ | -------------------- | ---------------------------------------------------------- | --------------------------------------------------------------- |
  | **Visible**  | eye                  | `valuesDisabledFiles` / `valuesDisabledSources` (inverted) | Hides lines from this source in merged preview                  |
  | **Errors**   | exclamation-triangle | `valuesShowErrors`                                         | Shows "new key" markers for lines from this source              |
  | **Unneeded** | minus-circle         | `valuesShowUnnecessary`                                    | Shows "unnecessary override" markers for lines from this source |

  Checkboxes work per file. For folder/group nodes, a tri-state checkbox is computed from the children (all checked / some checked / none checked). Toggling a folder propagates to all its children.

  Tree ordering (top to bottom):
  1. Merged Values — final merged result
  2. Repo root — all value files in the git repository directory structure:
     - group values, context values, cluster values (from gitops-repository)
     - root app values (type `rootApp` from gitops-repository, included even though parentAppId is not in the main appIds)
     - app values, upstream values (from charts-repository, chart tgz archives inserted at their `chartPath` location)
     - root app defaults (from root app chart tgz, path derived by replacing `<child>` with `root` in the child app's `chartPath`)
     - ArgoCD source annotations on matching nodes, Prio column shows ArgoCD priority number
  3. Hydra Values — fallback values from infra_library
  4. ArgoCD inline sources — `valuesObject`, `values` (string), and `parameters` entries shown as separate top-level nodes.

- **Right pane (45%):** Displays the raw YAML content of the clicked file, loaded on demand. Provenance gutter markers and hover tooltips are shown when available.

**Data model (`valuesTreeModel.ts`):** The tree content is represented as a UI-framework-agnostic data model. The pure function `buildValuesTree(input)` takes all required data sources and returns `ValuesTreeNode[]`. Each node carries a `kind` discriminator that describes what the node represents:

| `kind`              | Description                                                                                                                                                  |
| ------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `merged`            | The "Merged Values" root node                                                                                                                                |
| `repo-root`         | Git repository root container                                                                                                                                |
| `directory`         | Intermediate directory in the file tree                                                                                                                      |
| `exported-file`     | Value file from hydra-go export (group/context/cluster/app/rootApp). Files with `appId === parentAppId` and type `"app"` are auto-reclassified to `rootApp`. |
| `app-values`        | Main chart `values.yaml` from tgz (not in `/charts/` sub-directory). Represents the app's own default values in charts-repository.                           |
| `tgz-file`          | File extracted from a chart `.tgz` archive dependency (upstream values in `/charts/` sub-directory)                                                          |
| `root-app-defaults` | File from the root app chart tgz                                                                                                                             |
| `hydra-defaults`    | Hydra fallback values node                                                                                                                                   |
| `argocd-inline`     | Inline ArgoCD source (valuesObject, values, parameter)                                                                                                       |
| `tgz-root`          | Orphan tgz container (when no `chartPath` mapping exists)                                                                                                    |

`ValuesView.tsx` stores the model as non-persistent React state (`useState<ValuesTreeNode[]>`) and converts it to PrimeReact `TreeNode[]` only for rendering. This separation allows the tree-building logic to be unit-tested without any UI framework dependency.

**Git file links:** File nodes in the tree are clickable and open the corresponding file in the git web UI. The link URL is built from `global.hydra.repository` (git remote) and `global.hydra.revision` (branch), extracted from the merged values. These take precedence over the `gitRemote`/`gitBranch` fields from the hydra.yaml export, which serve as fallback. The stage directory in charts-repository paths (e.g. `dev`, `stage`, `prod`) is derived from `global.hydra.stage`.

#### Preview Mode (`valuesViewMode: "preview"`)

Shows the fully merged values for a selected app:

- **Content:** Loads merged values via `ClusterLoader.loadMergedValues(appId)` and displays as formatted YAML
- **Provenance annotations:** Each line in the editor has a coloured gutter marker indicating which source file it originates from. Hovering shows a tooltip with the full provenance chain. Lines with unnecessary overrides are greyed out; lines introducing new keys have a red tint. Parent/structural keys inherit the colour of their children when all children share the same source type.
- **Filtering:** Lines are filtered based on the checkbox states set in Tree mode. The "Visible" checkboxes control which sources appear; the "Errors" and "Unneeded" checkboxes control which annotation markers are shown.
- **Global clone detection:** Helm propagates top-level `global:` values into each sub-chart as `dep.global:`, including nested sub-chart dependencies (e.g. `parentChart.subChart.global:`). The viewer automatically detects these cloned sections by scanning merged flat keys for any `<prefix>.global.` pattern at any nesting depth. Each cloned leaf key is compared against its `global.` counterpart:
  - **Matching:** Line is dimmed (opacity 0.35, grey gutter marker) to indicate it's a redundant copy.
  - **Divergent:** Line gets a red background tint and red gutter marker, signalling that the sub-chart's copy deviates from the top-level global.
    Parent structural keys inherit clone status from children; a parent is marked divergent if any child diverges.
- **Hide Global Clones button:** A toggle button in the toolbar. When active (`valuesHideGlobalClones: true` in state), all global clone lines are removed from the editor output via `computeHiddenLines()`.

The merged values represent the final result of the entire values hierarchy (group → context → cluster → chart defaults → app overrides) as computed by hydra-go. The provenance analysis loads all source files and chart defaults (including transitive sub-chart dependencies from `.tgz` archives), resolves Helm dependency aliases via `Chart.yaml`, and maps each merged key back to its origin. Parent provenance propagation ensures structural keys display the correct colour when all their children share the same source.

### Manifests Tab

Shows a flat, sorted list of all entities from `filteredEntities` that have a non-empty `manifestPath`.

Each entry displays:

- **Manifest path** (monospace, primary colour) — the rendered manifest file path in the cluster dump
- **Entity info** — `kind/name` and namespace (secondary text)

Clicking an entry:

1. Sets a `selectionSlot` with a `templatePath` filter matching the entity's template, so that all entities from the same template are highlighted
2. Navigates to the entity detail page (`page=details`)

Data is computed via `collectManifestEntries(filteredEntities)` from `chartPageLogic.ts`.

### Templates Tab

Shows a PrimeReact `Tree` view grouped by `templatePath`:

- **Level 1:** Template path with manifest count (e.g. `my-chart/templates/deployment.yaml (3)`), icon `pi-file-edit`
- **Level 2:** Individual manifests showing `kind/name (namespace)`, icon `pi-file`, clickable (primary colour, underlined)

All template nodes are expanded by default. Clicking a manifest node at level 2 triggers the same navigation as the Manifests tab (selection filter + details page).

Entities without a `templatePath` are grouped under `(no template)`.

Data is computed via `buildTemplateGroups(manifestEntries)` from `chartPageLogic.ts`.

### App Tab

Shows the ArgoCD Application Custom Resource (CR) information for the selected chart.

**Availability:** The App tab is only visible when a matching ArgoCD Application entity is found for any of the chart's `appId`s. If no match exists, the tab is hidden from the tab menu entirely.

**Matching Logic:**
The lookup uses `findArgocdApp(appIds, entities)` from `argocdAppLookup.ts`. For each `appId` in the chart's `appId` array, it constructs the expected entity ID `argoproj.io/v1alpha1/Application/argocd/<appId>` and performs a direct `entities.get()` lookup. It then verifies the entity has `kind === "Application"` and `group === "argoproj.io"`. The function returns the first matching entity or `null`. Only the first match is used — multiple ArgoCD Applications are not shown.

**Cross-Cluster Data Loading:**
ArgoCD Application CRs are consolidated in the "in-cluster" (management cluster) export. During `hydra ui` context export, root apps from all clusters are rendered and their Application CR entities are merged into the in-cluster result. This means:

- When the user is viewing **in-cluster**, the Application entities are already available in the current `HydraData`.
- When the user is viewing **a different cluster**, the UI creates a second `ClusterLoader("in-cluster")` to load the in-cluster `hydra.yaml` and obtain the Application entities from there. This second loader is cached (created once, reused across tab switches and chart changes). See [cluster-dump.md](cluster-dump.md) → "Cross-Cluster Loading" for details.

**Tab Content:**
The tab uses the `ArgocdAppView` component and shows two sections:

1. **Structured Overview** (top) — Key fields from the Application spec displayed in a readable layout:
   - `spec.source` / `spec.sources` — repository URL, target revision, chart/path (supports both single-source and multi-source Applications)
   - `spec.destination` — server URL / cluster name, namespace
   - `spec.project` — ArgoCD project name
   - `spec.syncPolicy` — sync automation, sync options, retry policy
2. **Full YAML Manifest** (bottom) — The complete Application CR manifest displayed using `YamlHighlighter` with YAML syntax highlighting.

The YAML content is loaded via `ClusterLoader.loadManifest(entity.manifestPath)` — from the in-cluster loader when viewing a non-in-cluster cluster, or from the current cluster's loader when viewing in-cluster. The loaded YAML is then parsed with `js-yaml` to extract the structured `spec` fields for the overview section.

**Error Handling:**

- In-cluster `hydra.yaml` unavailable (non-in-cluster view): App tab is hidden (treated as no match).
- Manifest load failure (404, network error): Show "Application manifest not available" message in place of the YAML content. The structured overview is not shown since the spec fields cannot be extracted.

**Data Flow:**

```text
chart.appIds
      │
      ▼
findArgocdApp(appIds, entities)    ← entities from current cluster or in-cluster
      │
      ├── no match → App tab hidden
      └── match found
            │
            ▼
      loader.loadManifest(entity.manifestPath)    ← in-cluster loader or current loader
            │
            ├── error → "Application manifest not available"
            └── success → parse YAML
                  ├── Structured overview (spec.source, spec.destination, spec.project, spec.syncPolicy)
                  └── Full YAML manifest via YamlHighlighter
```

## Filter Interaction

The Chart page respects the active filter pipeline:

1. When a `selectionSlot` is active, `listFilteredEntities` contains entities matching the selection (ignoring global filter slots). Otherwise it equals `filteredEntities` (from global filter slots, OR-combined).
2. `getMatchingCharts()` extracts `appIds` from `listFilteredEntities`
3. Charts whose `appId` matches are shown in the dropdown (or as a label if only one)
4. If no entities match any appId (e.g. entities without appIds), all charts are shown
5. When filters or selection change, the dropdown updates in real-time (React reactivity via props)
6. Clicking a chart in the sidebar sets a `selectionSlot` with `appId` filters, causing only that chart to appear

**Chart.yaml is never filtered.** The Chart.yaml content loading depends on a stable chart name string (`chartYamlName = effectiveName ?? selected?.name`), not on the filtered `selected` object. This means filter changes do not trigger re-loads and the content always remains visible as long as a chart has been selected.

Parent charts are computed from ALL charts (not just filtered), since the dependency relationship is global.

The `handleSidebarFilter` function preserves the Chart view when it is active (unlike other page types which close on filter change).

## Components

### `chartPageLogic.ts`

Pure logic module (no React dependencies):

- **`getMatchingCharts(filteredEntities, allCharts)`** — Returns `GroupedChart[]` sorted by name
- **`getParentCharts(chartName, allCharts)`** — Returns `GroupedParentChart[]` — all charts that list `chartName` as a dependency, grouped by parent name
- **`chartExists(name, allCharts)`** — Returns `boolean` — whether a chart with the given name exists
- **`buildValueFilesTree(valueFiles)`** — Converts flat `HydraValueFile[]` to nested `ValueFileTreeNode[]` tree; directories before files, alphabetically sorted
- **`collectManifestEntries(entities)`** — Returns `ManifestListEntry[]` from entities with non-empty `manifestPath`, sorted alphabetically
- **`buildTemplateGroups(entries)`** — Groups `ManifestListEntry[]` by `templatePath` into `TemplateGroup[]`, sorted alphabetically

### `argocdAppLookup.ts`

Pure logic module (no React dependencies):

- **`findArgocdApp(appIds, entities)`** — Takes an array of `appId` strings and a `Map<string, HydraEntity>`. For each `appId`, constructs the expected entity ID `argoproj.io/v1alpha1/Application/argocd/<appId>` and performs a direct `entities.get()` lookup. Verifies the result has `kind === "Application"` and `group === "argoproj.io"`. Returns the first matching `HydraEntity` or `null`. Returns `null` for empty `appIds` arrays.

### `ArgocdAppView.tsx`

React component that renders the ArgoCD Application CR:

- **Props:** `manifestYaml` (raw YAML string of the Application CR, or `null` while loading / on error), `isDark`, `editorFontSize`
- **Structured Overview:** Parses the YAML with `js-yaml` and displays `spec.source`, `spec.destination`, `spec.project`, and `spec.syncPolicy` fields in a readable layout at the top. Hidden when `manifestYaml` is `null`.
- **Full YAML Manifest:** Renders the complete Application CR YAML using `YamlHighlighter` at the bottom. Shows an error/loading message when `manifestYaml` is `null`.

### `ChartPage.tsx`

React component with PrimeReact UI:

- **Props:** `filteredEntities`, `charts`, `isDark`, `valueFiles`, `appValues`, `loader`, `onOpenChart`, `onManifestSelect`, `editorFontSize`
- **Dropdown:** Lists all matching charts with version count. When only one chart matches, a simple label is shown instead of a dropdown.
- **TabMenu:** Five tabs (Details, Values, Manifests, Templates, App). The App tab is dynamically included/excluded — it only appears when `findArgocdApp()` returns a match for the current chart's appIds. The tab list is rebuilt on chart selection changes.
- **Inter-chart navigation:** `navigateToChart(name)` delegates to `onOpenChart` (which creates a selection filter and sets the chart name) or falls back to local selection + tab reset

### `YamlHighlighter.tsx`

Read-only CodeMirror 6 component for YAML syntax highlighting:

- **Props:** `content`, `isDark`, `maxHeight`, `fontSize`
- Uses `@codemirror/lang-yaml` for YAML highlighting, `oneDark` theme for dark mode
- Efficiently updates content without recreating the editor (only recreates when theme or font size changes)

## Files

| File                                      | Description                                                                                                                                                                                                     |
| ----------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/chartPageLogic.ts`                   | Matching, grouping, parent-finding, value file tree logic                                                                                                                                                       |
| `src/argocdAppLookup.ts`                  | `findArgocdApp()` — matches chart appIds to ArgoCD Application entities                                                                                                                                         |
| `src/components/ArgocdAppView.tsx`        | Renders ArgoCD Application CR: structured overview + full YAML manifest                                                                                                                                         |
| `src/components/ChartPage.tsx`            | Chart page component with five tabs (Details, Values, Manifests, Templates, App), dependency/parent trees inline, unified values view                                                                           |
| `src/components/YamlHighlighter.tsx`      | Read-only CodeMirror 6 YAML syntax highlighting component                                                                                                                                                       |
| `src/valuesProvenance.ts`                 | `flattenYaml`, `buildAliasMap`, `buildProvenance`, `mapProvenanceToLines`, `propagateParentProvenance`, `markGlobalCloneLines`, `detectGlobalClonePrefixes`, `computeHiddenLines`, `filterContentAndProvenance` |
| `src/clusterLoader.ts`                    | `loadValueFile()`, `loadMergedValues()`, `loadChartYaml()` for on-demand loading                                                                                                                                |
| `src/model.ts`                            | `HydraValueFile`, `HydraAppValues` types                                                                                                                                                                        |
| `src/parseHydra.ts`                       | Parses `valueFiles` and `appValues` from hydra.yaml                                                                                                                                                             |
| `src/state.ts`                            | `PageType` union includes `"charts"`; `valuesViewMode`, `valuesShowErrors` state fields                                                                                                                         |
| `src/useHashNavigation.ts`                | `parseHash`/`buildHash` handle `page=charts`                                                                                                                                                                    |
| `src/App.tsx`                             | Chart button, ChartPage rendering, handleSidebarFilter adjustment                                                                                                                                               |
| `src/components/ChartsTreeView.tsx`       | Charts sidebar tree with chart/dependency navigation                                                                                                                                                            |
| `src/components/SearchableCodeViewer.tsx` | CodeMirror editor with provenance gutter, tooltips, line decorations                                                                                                                                            |

## Tests

| Test File                                 | Coverage                                                                                                                                                             |
| ----------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `src/__tests__/argocdAppLookup.test.ts`   | `findArgocdApp()` — matching Application entity, no match returns null, empty appIds returns null, ignores non-Application entities, first match for multiple appIds |
| `src/__tests__/chartPageLogic.test.ts`    | `getMatchingCharts()`, `getParentCharts()`, `chartExists()`, `buildValueFilesTree()`                                                                                 |
| `src/__tests__/parseHydra.test.ts`        | `valueFiles` and `appValues` parsing                                                                                                                                 |
| `src/__tests__/useHashNavigation.test.ts` | `page=charts` parsing, building, and roundtrip                                                                                                                       |
| `src/__tests__/valuesProvenance.test.ts`  | `flattenYaml`, `buildAliasMap`, `buildProvenance`, `mapProvenanceToLines`, `propagateParentProvenance`, `computeHiddenLines`, `filterContentAndProvenance`           |
