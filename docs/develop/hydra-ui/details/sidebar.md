# Sidebar Architecture

## Overview

The sidebar is the left panel of the Hydra UI. It provides filtering, file browsing, searching, and graph configuration in a vertically stacked accordion layout. The sidebar width is resizable (170–600px, default 340px) via a drag handle.

```text
┌──────────────────────┬───┬──────────────────────────────────┐
│ Header               │   │                                  │
├──────────────────────┤ R │                                  │
│ ▼ Filter             │ e │                                  │
│   [FilterRow]        │ s │                                  │
│   [FilterRow]        │ i │       Graph Panel                │
│   [+ Filter]         │ z │    or State Editor (YAML)        │
│   [Filter Sidebar]   │ e │                                  │
├──────────────────────┤   │                                  │
│ ▼ Charts             │ H │                                  │
│   [ChartsTree]       │ a │                                  │
├──────────────────────┤ n │                                  │
│ ▼ Files              │ d │                                  │
│   [FilesTree]        │ l │                                  │
├──────────────────────┤ e │                                  │
│ ▼ Nodes              │   │                                  │
│   [SearchTree]       │   │                                  │
├──────────────────────┤   │                                  │
│ ▼ Groups             │   │                                  │
│   [TreePanel]        │   │                                  │
└──────────────────────┴───┴──────────────────────────────────┘
```text

## Header

The top section of the sidebar, always visible.

```text
┌──────────────────────────────────────────────┐
│ [🐉] Hydra    [🏠] [📋] [◈] [☀] [⚙]       │
└──────────────────────────────────────────────┘
  Logo  Title   Home Charts Graph Theme Settings
```

**Components:**

- **Logo**: Hydra icon (`/img/hydra.svg`) displayed to the left of the title
- **Title**: "Hydra" text next to the logo
- **Home icon** (`pi-home`): Navigates to list view. Highlighted when list view is active.
- **Graph icon** (`pi-share-alt`): Navigates to graph view. Highlighted when graph view is active.
- **Charts icon** (`pi-list`): Navigates to chart view. Highlighted when chart view is active.
- **Theme toggle** (`pi-sun`/`pi-moon`/`pi-palette`): Cycles through three modes on each click:
  - _Auto_ (`pi-palette`): Follows the operating system's light/dark preference
  - _Light_ (`pi-sun`): Forces light theme
  - _Dark_ (`pi-moon`): Forces dark theme
- **Settings icon** (`pi-cog`): Opens the Settings Page (`page=settings`). Highlighted when the settings page is active. See [navigation.md § Settings Page](navigation.md#settings-page) for tabs and edit-copy pattern.

Note: The cluster name has been removed from the header to free up space. All icons use PrimeReact `Button` components with `size="small"` and fixed 32×32px dimensions.

## Accordion Sections

The sidebar contains four collapsible accordion sections. Multiple sections can be expanded simultaneously. When expanded, the Files, Nodes, and Groups sections grow to fill available vertical space (`flex: 1`).

Section IDs: `"filter"`, `"charts"`, `"files"`, `"nodes"`, `"groups"`. Expansion state is part of the unified YAML state (see **State** section below).

### Filter Sidebar Toggle

At the bottom of the Filter section (below `FilterSlotsPanel`), a **Filter Sidebar** toggle (`InputSwitch`) controls whether the sidebar trees (Files, Charts, Nodes, Groups) show all entities/charts or only those matching the active filters.

| Toggle state      | Sidebar trees show                                    |
| ----------------- | ----------------------------------------------------- |
| **Off** (default) | All entities and all charts                           |
| **On**            | Only entities/charts matching the active filter slots |

**State:** `filterSidebar: boolean` (default: `false`), persisted in `HydraUiState`.

When off, `sidebarEntities` = `allEntitiesArray` and `sidebarCharts` = all charts. When on, `sidebarEntities` = `filteredEntities` and `sidebarCharts` = charts whose `appId` matches filtered entities.

### Filter

**Component:** `FilterSlotsPanel` (in `src/components/FilterSlotsPanel.tsx`)
**Purpose:** Manages global filter slots and the selection filter. Reduces the visible entity set.

The filter panel displays two categories:

1. **Selection filter** (temporary): Created by sidebar selection actions (Files/Groups) **or graph node/group clicks**. Green background, pin/clear actions. When active, the entity list ignores global filters.
2. **Global filter slots** (persistent): Expression-based filter slots (OR-combined). Each slot has view/edit modes with Done/Cancel buttons. Custom filters can be saved as named predefined filters.

**Buttons:**

- `+ Filter`: Adds a new empty custom filter slot with a unified grouped dropdown (Fields / Grouping Keys / Predefined Filters). Selecting a predefined filter immediately populates the slot.

**Header count:** Shows combined count of global + selection filters, e.g. `Filter (3)`.

**State:** Global filters (`activeFilterSlots`) persisted in `HydraUiState`. Selection filter is transient (not persisted).

#### Chip States (Global)

All filter chips (in global filter slots **and** the selection filter) share a single global `chipStates: Record<string, boolean>` managed in `App.tsx`. Clicking any chip toggles it globally — if the same chip key (e.g. `namespace:default`) appears in multiple places, it is toggled everywhere at once.

The chip key format is `field:value` for filter leaves (e.g. `namespace:default`, `gvk:apps/v1/Deployment`) and `group:groupName` for group references.

**Chip modes:**

| Mode       | Badge                    | Meaning                                                   |
| ---------- | ------------------------ | --------------------------------------------------------- |
| `enabled`  | `+` (green)              | Filter condition is active (include)                      |
| `disabled` | `−` (red)                | Filter condition is negated (exclude)                     |
| `inactive` | `○` (grey, line-through) | Filter condition is skipped (ignored during evaluation)   |
| `partial`  | `◐` (orange)             | Composite field: some sub-fields are inactive (see below) |

#### Composite Field ↔ Sub-Field Coupling

Several grouping fields are _composite_, meaning their values are composed of multiple simpler fields joined by `/`. The chip state system couples composite fields with their sub-fields so that toggling a composite chip also affects related standalone chips and vice versa.

| Composite Field | Sub-Fields                              | Example Value                     | Sub-Keys                                                           |
| --------------- | --------------------------------------- | --------------------------------- | ------------------------------------------------------------------ |
| `apiVersion`    | `group`, `version`                      | `apps/v1`                         | `group:apps`, `version:v1`                                         |
| `gvk`           | `group`, `version`, `kind`              | `apps/v1/Deployment`              | `group:apps`, `version:v1`, `kind:Deployment`                      |
| `gvkn`          | `group`, `version`, `kind`, `namespace` | `apps/v1/Deployment/default`      | `group:apps`, `version:v1`, `kind:Deployment`, `namespace:default` |
| `nscs`          | `namespace`                             | `default` / `""` (cluster-scoped) | `namespace:default` / `namespace:`                                 |

**Toggle propagation:** When a composite chip is toggled, all corresponding sub-field chip keys are toggled as well. For example, toggling `gvk:apps/v1/Deployment` off also sets `group:apps`, `version:v1`, and `kind:Deployment` to inactive. Toggling `nscs:default` off also sets `namespace:default` to inactive.

**Partial mode:** When a composite chip itself is active, but some of its sub-field keys (e.g. `kind:Deployment`) are independently deactivated, the composite chip renders in `partial` mode (orange ◐ badge). If _all_ sub-fields are inactive, the composite shows `inactive`.

**Parsing rules:**

- `apiVersion`: `group/version` (e.g. `apps/v1`); core resources without group: single part treated as `version` with empty `group`.
- `gvk`: `group/version/kind`; core: `version/kind` → empty `group`.
- `gvkn`: `group/version/kind/namespace` (4 parts) or `gvk` without namespace for cluster-scoped (3 parts).
- `nscs`: namespace string directly; empty string for cluster-scoped → `namespace:` sub-key.

**Implementation:** `getCompositeSubKeys()`, `parseGvk()`, `getGvkSubKeys()`, `computeChipMode()`, and `toggleChipState()` are exported from `src/filterExprHelpers.ts`. Tests in `src/__tests__/chipStateGvk.test.ts`.

#### Filter Display (View Mode)

Filters and selections are displayed vertically with indentation for nested groups:

```text
chip1
AND
(
  chip2
  OR
  chip3
)
AND
chip4
```text

Each group level is indented by 8px. Parentheses mark sub-groups. The operator label (`AND`/`OR`) appears between children.

→ See [filters.md](filters.md) for full filter architecture.

### Charts

**Component:** `ChartsTreeView` (in `src/components/ChartsTreeView.tsx`)
**Purpose:** Browse Helm charts and their dependencies (including transitive). Navigate to chart details.

**View mode toggle:** A `SelectButton` with two options (Tree / Flat) at the top of the section, next to the search input. State: `chartsViewMode: ChartsViewMode` (default: `"tree"`), persisted in `HydraUiState`.

**Search:** A debounced search input (`DebouncedSearch` component) filters the tree/list by chart/dependency name. Triggers after 500ms inactivity or on Enter.

```text
┌─────────────────────────────────┐
│ ▼ Charts                        │
├─────────────────────────────────┤
│ [Tree|Flat] [Filter charts... ] │
├─────────────────────────────────┤
│  (Tree mode)                    │
│  ├── my-chart (1.0.0)          │  ← chart (click → chart page)
│  │   ├── redis ^7.0.0          │  ← direct dependency (navigable)
│  │   │   └── redis-lib ^1.0.0  │  ← transitive dependency
│  │   └── postgres ^14.0.0      │  ← direct dependency (navigable)
│  ├── redis (2.0.0)             │  ← chart
│  │   └── redis-lib ^1.0.0      │  ← dependency
│  └── other-chart (3 versions)  │  ← chart with multiple versions
│      └── my-chart ^1.0.0       │  ← dependency (with its own sub-deps)
├─────────────────────────────────┤
│  (Flat mode)                    │
│  my-chart (1.0.0)              │  ← all charts + deps as root nodes
│  other-chart (3.0.0)           │
│  postgres (14.0.0)             │
│  redis (2.0.0)                 │
│  redis-lib (1.0.0)             │
└─────────────────────────────────┘
```

**Tree mode** (default):

Charts are grouped by root app ID (first two segments of `appId`). Each root group contains chart nodes with their dependencies as children.

- **Root group nodes**: Top-level folder grouping by root app ID. Icon: `pi pi-folder`.
- **Chart nodes**: Label: `name (version)`. Icon: `pi pi-list`. Clicking opens the chart details page.
- **Dependency nodes**: Children of chart nodes. Label: `name version`. Icon: `pi pi-link` if the dependency exists as a chart (navigable), `pi pi-box` otherwise. Navigable dependencies are rendered as clickable links (underlined, primary color).
- **Transitive dependencies**: If a dependency itself exists as a chart in the data, its own dependencies are recursively expanded as children. Circular dependencies are detected and broken (each chart name is only expanded once per path). This gives a full dependency tree view.

**Flat mode:**

All charts and all their dependencies are collected, deduplicated by name, and shown as a flat alphabetically sorted list of root-level nodes (no children). Each node shows `name (version)` with icon `pi pi-list` for charts that exist in the data, `pi pi-box` for external dependencies. Clicking any node opens the chart details page (if it exists as a chart).

**Data source:** Receives `sidebarCharts` from `App.tsx` — all charts when Filter Sidebar is off, or charts matching filtered entities when on. Also receives `highlightedChartNames` (set of chart names matching the current `selectionSlot`).

**Interaction:**

| Click target             | Action                                                                                                                                                                                                                                            |
| ------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Root group node          | Sets a **selection filter** (single `rootAppId` filter matching the clicked root app ID). Does **not** navigate to the chart details page — only sets the filter.                                                                                 |
| Chart node               | Sets a **selection filter** (`appId` filters for all non-empty appIds of that chart, OR-combined) and opens chart details page (`view=chart`, selected chart set). The selection filter causes the chart dropdown to show only the clicked chart. |
| Navigable dependency     | Same as chart node click — sets selection filter and opens chart details page for that dependency chart                                                                                                                                           |
| Non-navigable dependency | No action                                                                                                                                                                                                                                         |

**Bold highlighting:** Chart nodes whose name matches any chart in the `selectionChartNames` set (derived from `selectionSlot` + `listFilteredEntities`) are displayed in **bold** text. This works for any selection filter source (sidebar click, graph click, etc.), not just chart clicks.

**Auto-expand:** When highlighted charts exist, their parent root-group nodes are automatically expanded via `expandedKeys`.

### Files

**Component:** `FilesTreeView` (in `src/components/FilesTreeView.tsx`)
**Purpose:** Browse entities by their Helm template source file path.

**Search:** A debounced search input (`DebouncedSearch` component) at the top filters the tree by filename/directory name. Triggers after 500ms inactivity or on Enter.

```text
┌─────────────────────────────────┐
│ ▼ Files                         │
├─────────────────────────────────┤
│  ├── prod (8)                  │  ← cluster
│  │   ├── myapp (5)             │  ← rootApp
│  │   │   ├── child (3)         │  ← childApp
│  │   │   │   └── chart/ (3)   │
│  │   │   │       └── templates/ (3)│
│  │   │   │           ├── deploy.yaml (1)│
│  │   │   │           └── svc.yaml (2)│
│  │   │   └── other (2)         │  ← childApp
│  │   │       └── chart/ (2)   │
│  │   └── otherapp (3)          │  ← rootApp (2-seg appId, no child)
│  │       └── chart/ (3)       │
│  │           └── templates/ (3)│
│  └── staging (4)               │  ← cluster
│      └── app1 (4)              │  ← rootApp
│          └── chart/ (4)       │
└─────────────────────────────────┘
```text

**Tree structure:**

The tree is built from the `templatePath` and `appIds` values of all filtered entities by `buildFilesTree()` in `src/filesTreeLogic.ts`. The appId format is `cluster.rootApp.childApp` (dot-separated). Entities are grouped into a 3-level hierarchy derived from their appId segments, followed by the directory/file tree from `templatePath` values. Each entity's `templatePath` (e.g. `my-chart/templates/deployment.yaml`) is split by `/` into directory segments and a filename. The tree has seven node types (`FilesTreeNodeType`):

- **Root node** (`"root"`): Invisible top-level container. Always present, `name: ""`, `entityCount` = total entities in the tree.
- **Cluster nodes** (`"cluster"`): Top-level grouping by the first appId segment (e.g. `prod`). Icon: `pi pi-server`.
- **RootApp nodes** (`"rootApp"`): Second-level grouping by the second appId segment (e.g. `myapp`). Only present for 2+ segment appIds. Icon: `pi pi-box`.
- **ChildApp nodes** (`"childApp"`): Third-level grouping by the third appId segment (e.g. `child`). Only present for 3-segment appIds. Icon: `pi pi-th-large`.
- **Directory nodes** (`"directory"`): Intermediate path segments. Label shows the directory name and the total entity count (recursive) in parentheses, e.g. `templates/ (5)`. Icon: `pi pi-folder`.
- **File nodes** (`"file"`): Leaf path segments (template files). Label shows the filename and the entity count in parentheses, e.g. `configmaps.yaml (2)`. Icon: `pi pi-file`.
- **Entity nodes** (`"entity"`): Individual entities from the template. Displayed as `Kind Name` (e.g. `Deployment demo`). Have `entityId` and `entityLabel` fields. Icon: `pi pi-cog`.

**Hierarchy mapping from appId segments:**

- 3-segment appId `prod.myapp.child` → cluster `prod` → rootApp `myapp` → childApp `child` → paths
- 2-segment appId `prod.myapp` → cluster `prod` → rootApp `myapp` → paths (no childApp level)
- 1-segment appId `prod` → cluster `prod` → paths (no rootApp or childApp levels)
- No appId → `"(no cluster)"` placeholder → paths

Entities with multiple `appIds` appear under each of their corresponding hierarchies.

**Sorting:** Cluster nodes are sorted alphabetically, with `"(no cluster)"` always last. Within each level, hierarchy nodes (rootApp, childApp) sort before directory/file/entity nodes, then alphabetically by name. Entity leaves within a file are sorted by `templateIndex` (ascending) before the alphabetical sort.

**Entities without templatePath** are excluded from the tree.

**Filter info:** Each tree node carries a `filterInfo` array of `{ field, value }` pairs, pre-computed during tree building. The array always contains at most two entries: the hierarchy filter and the most specific path filter for this node. Each hierarchy level uses a progressively more specific filter field:

- **Cluster node**: `[{ field: "clusterName", value: "prod" }]`
- **RootApp node**: `[{ field: "rootAppId", value: "prod.myapp" }]`
- **ChildApp node**: `[{ field: "appId", value: "prod.myapp.child" }]`
- **Directory node**: `[parent's filter, { field: "templatePathPrefix", value: "chart/templates/" }]`
- **File node**: `[parent's filter, { field: "templatePath", value: "chart/templates/deploy.yaml" }]`
- **Entity node**: same filterInfo as its parent file node

Only the most specific path filter is kept — intermediate directory prefixes are not included because the most specific prefix already implies all parent prefixes.

**Interaction:**

Clicking any node in the Files tree creates a **selection filter** from the node's `filterInfo` (AND-combined). Child nodes of the clicked node are ignored — only the path from root to the clicked node determines the filter. Non-entity nodes always open the list view (no chart-specific behavior).

| Click target  | Action                                                                                                                              |
| ------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Cluster node  | Selection filter: `clusterName = "prod"` → switches to list view (or highlights in graph)                                           |
| RootApp node  | Selection filter: `rootAppId = "prod.myapp"` → switches to list view (or highlights in graph)                                       |
| ChildApp node | Selection filter: `appId = "prod.myapp.child"` → switches to list view (or highlights in graph)                                     |
| Directory     | Selection filter: parent filter AND `templatePathPrefix = "chart/templates/"` → switches to list view (or highlights in graph)      |
| File          | Selection filter: parent filter AND `templatePath = "chart/templates/deploy.yaml"` → switches to list view (or highlights in graph) |
| Entity leaf   | Opens the **detail page** for that entity (`page=details&node=<entityId>`)                                                          |

Note: Nodes with `chartInfo` (rootApp/childApp) still show chart tooltips on hover but no longer navigate to the chart page. Use the Charts tab to navigate to chart details.

The filter is set as a selection slot (green background in Filter panel, with pin/clear actions). Each level produces a separate filter chip that can be independently toggled. If the graph is currently visible, the selection is highlighted there instead of switching to list view.

**Filter matching** (in `makeMatchLeaf`):

| Field                | Matching                                                                                                                   |
| -------------------- | -------------------------------------------------------------------------------------------------------------------------- |
| `appId`              | Checks if value is contained in `entity.appIds` array (or matches joined string for backward compat)                       |
| `clusterName`        | Checks if any `entity.appIds` has a first dot-segment matching the value; `"(no cluster)"` matches entities with no appIds |
| `rootAppId`          | Checks if any `entity.appIds` equals the value or starts with `value + "."`                                                |
| `templatePathPrefix` | Checks if `entity.templatePath.startsWith(value)`                                                                          |
| `templatePath`       | Checks if `entity.templatePath === value` (exact match)                                                                    |

Whether dependency highlighting (reachability, distance badges, colored edges) is shown depends on the **Dependencies toggle** in the graph panel toolbar (see [Graph Panel Toolbar](#graph-panel-toolbar)).

### Nodes

**Component:** `SearchTree` (in `TreePanel.tsx`)
**Purpose:** Text-based entity search with a filterable tree view.

```text
┌─────────────────────────────────┐
│ [Search...                    ] │
├─────────────────────────────────┤
│  └── :namespace:default (3)     │
│        ├── Kind name            │
│        │   entity-id            │
│        └── Kind name            │
│            entity-id            │
└─────────────────────────────────┘
```

**Features:**

- **Search input**: Text search with 200ms debounce. Persisted in localStorage.
- **Tree view**: Shows all filtered entities grouped by the configured grouping fields. All groups are auto-expanded. Clicking a leaf entity opens its **detail page** (`page=details&node=<entityId>`).
- **Leaf display**: Each leaf entity shows a configurable name (top line, bold) and description (bottom line, muted). Both are composed from configurable fields and separators.

**Configuration:** Search grouping, search fields, and leaf display settings are configured in the Settings Page (**Search** tab, `#<cluster>?page=settings&tab=search`). See [navigation.md § Settings Page](navigation.md#settings-page).

### Groups

**Component:** `TreePanel` (in `src/components/TreePanel.tsx`)
**Purpose:** Group tree view with expand/collapse controls and group-based filtering.

```text
┌─────────────────────────────────┐
│  [🔍 Filter groups...         ]│
│  └── ☑ default (10)            │
│        ├── ☑ frontend (3)      │
│        └── ☐ backend (2)       │
└─────────────────────────────────┘
```text

**Search:** A debounced search input (`DebouncedSearch` component) at the top filters the tree by group label. Triggers after 500ms inactivity or on Enter.

**Tree view:**

- Shows only **groups** (no leaf entities)
- Groups with only **one entry** are hidden from the tree
- **Always shows all groups** — not affected by global filters (receives `allEntitiesArray`)
- **Collapse in graph checkbox** (only visible when graph is active):
  - Checked = group is expanded in the graph (entities visible)
  - Unchecked = group is collapsed (shown as single node)
- **Click behavior:** Always creates a **selection filter** from the group key's field/value pairs (AND-combined). The filter is set as a selection slot (green background in Filter panel). When the graph is visible, matching entities are highlighted in the graph. Bold text indicates groups with at least one entity matching the current selection filter.
- Entity count shown in parentheses
- Tree expansion (which groups show their sub-groups) is managed separately from graph expansion
- Parent groups (those with sub-groups) are auto-expanded in the Tree on grouping change

**Configuration:** Grouping levels, clone rules, color rules, labels, and grouping keys are all configured in the Settings Page. The relevant tabs are **Graph** (`#<cluster>?page=settings&tab=graph`) and **Grouping Keys** (`#<cluster>?page=settings&tab=groupingKeys`). See [navigation.md § Settings Page](navigation.md#settings-page) for the full tab listing and edit-copy pattern. See [layout.md](layout.md) for the data models (grouping, cloning, color rules, labels) and [grouping-keys.md](grouping-keys.md) for grouping key details.

## Graph Panel Toolbar

The graph panel has a floating toolbar in the bottom-right corner with five icon buttons. All icons use `fontSize="medium"` and `size="medium"`.

```text
┌──────────────────────────────────────────────────┐
│                                                  │
│                   Graph Panel                    │
│                                                  │
│                                                  │
│              [🔲] [⊕] [🔍+] [🔍-] [🌿]         │
└──────────────────────────────────────────────────┘
```

| Button       | Icon                | Action                                            |
| ------------ | ------------------- | ------------------------------------------------- |
| Fit Screen   | `FitScreen`         | Zoom to show all nodes                            |
| Center Focus | `CenterFocusStrong` | Zoom to selected node (disabled if none selected) |
| Zoom In      | `ZoomIn`            | Incremental zoom in (centered on viewport)        |
| Zoom Out     | `ZoomOut`           | Incremental zoom out (centered on viewport)       |
| Dependencies | `AccountTree`       | Toggle dependency highlighting on/off (see below) |

### Dependencies Toggle

The Dependencies toggle button controls whether reachability highlighting (distance badges and highlighted reachability edges) is shown for selected nodes.

- **Active** (default): The button is visually highlighted (primary color). Dependency highlighting is shown for all selected nodes — regardless of whether it is a single-select or multi-select.
- **Inactive**: The button is dimmed. Only the selected nodes themselves are visually marked, without any dependency highlighting.

State: `showDependencies: boolean` (default: `true`), persisted in `HydraUiState`.

**Mouse interaction** (graph panel):

| Input                    | Action                                                                                       |
| ------------------------ | -------------------------------------------------------------------------------------------- |
| Left click on node/group | Creates a **selection filter** matching the clicked node/group (replaces existing selection) |
| Ctrl/Cmd + left click    | Adds an **OR condition** to the current selection filter (multi-select)                      |
| Left click on background | Clears the selection filter                                                                  |
| Left drag on node/group  | Move (drag & drop) the node/group                                                            |
| Left drag on background  | Rubber band zoom: draws a rectangle, zooms to fit that area on release (min 10px)            |
| Right drag on background | Pan (scroll) the entire view                                                                 |
| Mouse wheel              | Zoom in/out (centered on cursor)                                                             |

## NumberInput Component

A reusable component for numeric input fields with increment/decrement buttons.

```text
[-] [  10  ] [+]
```text

| Component        | Description                                                               |
| ---------------- | ------------------------------------------------------------------------- |
| **`[-]` button** | Decrements the value by 1. Disabled when at minimum value (if specified). |
| **Number input** | Editable text field showing the current value. Accepts only integers.     |
| **`[+]` button** | Increments the value by 1. Disabled when at maximum value (if specified). |

**Props:** `value`, `onChange`, `min?`, `max?`, `step?` (default 1).

**Usage:** All numeric input fields in the UI use this component:

- Auto-Clone thresholds (`thresholdIn`, `thresholdOut`)
- Grouping Key path level (when field is `Template Path`)
- Any future numeric inputs

## Resize Handle

A 6px wide invisible drag handle between the sidebar and the graph panel. Highlights in the primary color on hover and during drag. Constrains width to 170–600px.

## State Editor

The State Editor is accessible via the Settings Page → **State (YAML)** tab (`#<cluster>?page=settings&tab=state`). It provides a CodeMirror 6 YAML editor with syntax highlighting, live sync, and dirty tracking.

→ **Full documentation:** [state-editor.md](state-editor.md)

→ **State type and sub-state details:** [state.md](state.md)

## State Management

All sidebar state is managed in `App.tsx` and passed down via props. Components are stateless with respect to application data — they only manage transient internal UI state (Tree expansion).

→ **Full state documentation:** [state.md](state.md) — types, defaults, data flow, persistence, and how to add new settings.

### State Flow (Summary)

All configurable settings are combined into `HydraUiState` (defined in `src/state.ts`), serialized to YAML, and persisted in a single localStorage key (`hydra-state`). There are no other localStorage keys. Defaults are stripped from the YAML.

**Transient state** (not persisted):

- `selectionSlot` (`ActiveFilterSlot | null`) → current selection filter (derived: `selectionMatchIds`)
- `graphExpandedGroups` (`Set<string>`) → which groups are expanded in the graph
- `zoom callbacks` → graph viewport control

### Selection Model

All selection state is driven through a single `selectionSlot` (`ActiveFilterSlot | null`) in `App.tsx`. There is no separate `selectedNodeIds` state. Graph highlighting, dependency calculation, list filtering, and sidebar bold labels are all derived from `selectionSlot`.

**Derived state:** `selectionMatchIds` (`Set<string>`) is computed in `App.tsx` by evaluating the `selectionSlot` filter against all cloned entities and mapping matched entity IDs through `entityToNodeId` (to include collapsed group node IDs). This set is passed to `GraphPanel` for highlighting and dependency calculation, and to `TreePanel` for bold group labels.

**Selection sources:**

| Source                         | Filter creation                                                                                             |
| ------------------------------ | ----------------------------------------------------------------------------------------------------------- |
| Graph click (entity)           | `namespace` + `kind` + `name` (AND-combined)                                                                |
| Graph click (group)            | Parsed from group key (e.g. `:namespace:default:kind:Deployment` → `namespace=default AND kind=Deployment`) |
| Graph Ctrl+click               | Adds OR condition to existing selection filter                                                              |
| Graph background click         | Clears selection filter                                                                                     |
| Files tree click               | `filterInfo` from tree node (cluster/rootApp/appId + path filter)                                           |
| Charts tree click (chart/dep)  | `appId` filters for all appIds of the clicked chart name (OR-combined)                                      |
| Charts tree click (root group) | Single `rootAppId` filter matching the clicked root app ID                                                  |
| Groups tree click              | Parsed from group key                                                                                       |
| SearchTree entity click        | Opens detail page (no filter change)                                                                        |

### Interaction with Graph

| User Action                                            | Sidebar Effect                                                                  | Graph Effect                                             |
| ------------------------------------------------------ | ------------------------------------------------------------------------------- | -------------------------------------------------------- |
| Toggle filter                                          | Updates `filters` state                                                         | Entities re-filtered, layout recalculated                |
| Click cluster/rootApp/childApp/directory/file in Files | Sets `selectionSlot`, switches to list view (or highlights in graph if visible) | Matching entities highlighted via `selectionMatchIds`    |
| Click root group in Charts tab                         | Sets `selectionSlot` (appId filters for all charts under root)                  | Matching entities highlighted via `selectionMatchIds`    |
| Click chart in Charts tab                              | Sets `selectionSlot` (appId filters), opens chart details page (`view=chart`)   | Matching entities highlighted via `selectionMatchIds`    |
| Click entity in Files                                  | Opens detail page                                                               | —                                                        |
| Search + click entity in Nodes                         | Opens detail page                                                               | —                                                        |
| Toggle group checkbox                                  | Updates `graphExpandedGroups`                                                   | Group expanded/collapsed, layout recalculated            |
| Click group label                                      | Sets `selectionSlot`                                                            | Matching entities highlighted via `selectionMatchIds`    |
| Change grouping/layout                                 | Updates `graphGroupingConfig`                                                   | Tree rebuilt, layout recalculated                        |
| Change color rules                                     | Updates `colorRules`                                                            | Graph re-rendered with new colors                        |
| Change clone rules                                     | Updates `cloneRules`                                                            | Entities re-cloned, tree rebuilt, layout recalculated    |
| Toggle Auto-Clone / change thresholds                  | Updates `autoClone`                                                             | Auto-rules regenerated, entities re-cloned, tree rebuilt |
| Toggle Dependencies button                             | Updates `showDependencies`                                                      | Reachability highlighting toggled                        |
| Edit YAML in State Editor                              | All relevant state updated at once                                              | Depends on what changed                                  |
| Zoom controls                                          | —                                                                               | Graph viewport adjusted                                  |
| Left click node in graph                               | Sets `selectionSlot`                                                            | Node highlighted, dependencies shown                     |
| Ctrl+click node in graph                               | Adds OR condition to `selectionSlot`                                            | Multiple nodes highlighted                               |
| Left drag node in graph                                | —                                                                               | Node repositioned (drag & drop)                          |
| Right drag in graph                                    | —                                                                               | View panned (scrolled)                                   |

## Components

### Component Tree

```text
App.tsx
├── Header (logo, Home, Charts, Graph, theme toggle, settings icon)
├── Filter Accordion
│   ├── FilterSlotsPanel
│   │   ├── Selection filter (pin/clear)
│   │   └── Global filter slots (view/edit, Done/Cancel/Save As)
│   └── Filter Sidebar toggle (InputSwitch)
├── Charts Accordion
│   └── ChartsTreeView
│       ├── SelectButton (Tree / Flat toggle)
│       ├── DebouncedSearch (filter charts)
│       └── PrimeReact Tree (tree mode: charts → dependencies; flat mode: all as root nodes)
├── Files Accordion
│   └── FilesTreeView
│       ├── DebouncedSearch (filter files)
│       └── PrimeReact Tree (directories → files → entities)
├── Nodes Accordion
│   └── SearchTree
│       ├── Search input (200ms debounce)
│       └── Tree view (entities grouped by configured fields)
├── Groups Accordion
│   └── TreePanel
│       ├── DebouncedSearch (filter groups)
│       └── Group tree with expand/collapse checkboxes
├── Resize Handle
└── Main Panel (right side)
    ├── SettingsPage (when page=settings)
    │   ├── Tabs: Grouping Keys, Predefined Filters, Graph, Search, State (YAML), Reset
    │   └── Edit-copy pattern (Save / Apply / Revert / Cancel)
    ├── ChartPage (when view=chart)
    ├── GraphPanel (when view=graph)
    │   ├── Toolbar: [FitScreen] [CenterFocus] [ZoomIn] [ZoomOut] [Dependencies]
    │   ├── EntityPage (when page=details/relations/rbac/secrets)
    │   └── ClonePage (when page=clone)
    └── EntityListPanel (when view=list)
        ├── DebouncedSearch (1s debounce, pre-filters data)
        └── PrimeReact DataTable (paginated, sortable, column visibility)
```

### NumberInput

**File:** `src/components/NumberInput.tsx`

Reusable numeric input with `[-]` and `[+]` buttons. See [NumberInput Component](#numberinput-component) above.

### DebouncedSearch

**File:** `src/components/DebouncedSearch.tsx`

Reusable search input component with configurable debounce delay (default 1000ms). Isolated via `React.memo` to prevent parent re-renders on keystroke. Fires `onFilterChange` after debounce timeout or on Enter. Used in EntityListPanel, TreePanel (Groups), and FilesTreeView (Files).

### FilterSlotsPanel

**File:** `src/components/FilterSlotsPanel.tsx`

Manages the filter UI in the sidebar. Renders selection filter (temporary, green) and global filter slots (persistent). Supports view/edit modes with Done/Cancel/Save As actions. Edit snapshots enable Cancel to revert changes.

### FilesTreeView

**File:** `src/components/FilesTreeView.tsx`

Builds a directory tree from entity `templatePath` values. Each node shows its child count in parentheses. Clicking applies a selection filter for the clicked path/hierarchy node (list view); clicking an entity leaf opens its details page. Includes DebouncedSearch for filtering. Does not navigate to chart pages — chart navigation is handled by `ChartsTreeView`.

### ChartsTreeView

**File:** `src/components/ChartsTreeView.tsx`

Displays Helm charts in two modes: **Tree** (hierarchical, grouped by root app ID with dependency sub-trees) or **Flat** (all charts and dependencies as alphabetically sorted root nodes). A `SelectButton` toggle switches between modes; the choice is persisted as `chartsViewMode` in `HydraUiState`. Clicking a chart or navigable dependency opens the chart details page (`view=chart`). Includes DebouncedSearch for filtering.

### SearchTree

**File:** `src/components/TreePanel.tsx`

Search tree with debounced text filtering and a tree view that shows entities as leaves. Search configuration (grouping, search fields, leaf display) is managed via the Settings Page (Search tab).

### TreePanel

**File:** `src/components/TreePanel.tsx`

Groups tree panel with group expand/collapse checkboxes and group-based selection filtering. Always shows all groups (unfiltered by global filters). Clicking a group always creates a selection filter (via `onFilterByGroup`). Groups with at least one entity matching the current selection are displayed in bold (via `selectionMatchIds`). Includes DebouncedSearch for filtering groups. Graph configuration is managed via the Settings Page (Graph tab).

### SettingsPage

**File:** `src/components/SettingsPage.tsx`

Consolidated settings page with tabs: Grouping Keys, Predefined Filters, Graph, Search, State (YAML), Reset. Uses the edit-copy pattern — changes are committed to live state via Save/Apply. See [navigation.md § Settings Page](navigation.md#settings-page).

### StateEditor

**File:** `src/components/StateEditor.tsx`

YAML editor component used within the Settings Page (State tab). → See [state-editor.md](state-editor.md) for full documentation.

## Tests

### Chip State / Composite Field Tests (`src/__tests__/chipStateGvk.test.ts`)

Tests for `parseGvk()`, `getGvkSubKeys()`, `getCompositeSubKeys()`, `toggleChipState()`, and `computeChipMode()` in `src/filterExprHelpers.ts`.

| Test group            | Verifies                                                                                                                                                                                                        |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `parseGvk`            | Parses 3-part (`apps/v1/Deployment`), 2-part (`v1/Pod`), dotted groups, and rejects invalid inputs                                                                                                              |
| `getGvkSubKeys`       | Produces correct `group:`, `version:`, `kind:` chip keys from a GVK value                                                                                                                                       |
| `getCompositeSubKeys` | Returns correct sub-keys for all composite fields: `apiVersion` (group+version), `gvk` (group+version+kind), `gvkn` (group+version+kind+namespace), `nscs` (namespace). Returns empty for non-composite fields. |
| `toggleChipState`     | Simple toggle, composite→sub-field propagation for gvk/apiVersion/gvkn/nscs, re-enable from inactive, non-composite isolation                                                                                   |
| `computeChipMode`     | enabled/disabled/inactive for simple chips; partial/inactive for composite fields with mixed sub-field states (gvk, apiVersion, gvkn, nscs)                                                                     |

### Files Tree Tests (`src/__tests__/filesTreeLogic.test.ts`)

Tests for `buildFilesTree()` and `getEntityIdsForNode()` in `src/filesTreeLogic.ts`.

| Test                                        | Verifies                                                                                                    |
| ------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Empty entity list                           | `buildFilesTree([])` returns an empty root with `entityCount: 0`                                            |
| No entities with templatePath               | Entities without `templatePath` produce an empty tree                                                       |
| Single entity with 3-segment appId          | Builds correct cluster → rootApp → childApp → directory → file → entity hierarchy                           |
| 2-segment appId (no childApp)               | Paths go directly under rootApp node (no childApp level)                                                    |
| 1-segment appId (cluster only)              | Paths go directly under cluster node (no rootApp or childApp levels)                                        |
| Multiple entities in same file              | Two entities sharing the same `templatePath` and `appIds` are grouped under one file node                   |
| Multiple files in same directory            | Two entities with different filenames in the same directory produce sibling file nodes                      |
| Recursive entity count propagation          | `entityCount` on directory nodes equals the total count of all descendant entities (recursive)              |
| Skip entities without templatePath          | Entities with empty `templatePath` are excluded from the tree; entity count reflects only included entities |
| Hierarchy — different clusters              | Entities with different cluster segments produce separate top-level cluster groups                          |
| Hierarchy — rootApps under same cluster     | Entities with same cluster but different rootApps are grouped under one cluster                             |
| Hierarchy — childApps under same rootApp    | Entities with same cluster.rootApp but different childApps are grouped under one rootApp                    |
| Hierarchy — multiple appIds per entity      | Entity with multiple appIds appears under each corresponding hierarchy                                      |
| Hierarchy — no appIds                       | Entities without appIds are grouped under `"(no cluster)"`                                                  |
| Hierarchy — sort (no cluster) last          | `"(no cluster)"` sorts after all named clusters                                                             |
| Hierarchy — entity counts propagate         | `entityCount` on cluster/rootApp/childApp nodes equals total descendant entities                            |
| filterInfo — cluster nodes                  | Cluster nodes carry `[{ field: "clusterName", value: "<cluster>" }]`                                        |
| filterInfo — rootApp nodes                  | RootApp nodes carry `[{ field: "rootAppId", value: "<cluster>.<rootApp>" }]`                                |
| filterInfo — childApp nodes                 | ChildApp nodes carry `[{ field: "appId", value: "<full appId>" }]`                                          |
| filterInfo — directory (most specific only) | Directory nodes carry parent's filter + only their own `templatePathPrefix`                                 |
| filterInfo — file nodes                     | File nodes carry parent's filter + `{ field: "templatePath", value }` (exact path)                          |
| filterInfo — entity propagation             | Entity leaves inherit the same filterInfo as their parent file                                              |
| filterInfo — (no cluster) entities          | Entities without appIds get `{ field: "clusterName", value: "(no cluster)" }`                               |
| filterInfo — 2-segment appId directories    | Directories under rootApp carry `rootAppId` filter + path filter                                            |
| filterInfo — root node                      | Root node has empty filterInfo                                                                              |
| makeMatchLeaf — appId containment           | `appId` filter checks array containment, not exact joined string                                            |
| makeMatchLeaf — appId joined string         | Backward-compatible matching for joined appId string                                                        |
| makeMatchLeaf — (no app)                    | Entities with empty appIds match `appId = "(no app)"`                                                       |
| makeMatchLeaf — clusterName                 | `clusterName` checks first dot-segment of any appId; `"(no cluster)"` matches no-appId entities             |
| makeMatchLeaf — rootAppId                   | `rootAppId` checks if any appId equals or starts with the value                                             |
| makeMatchLeaf — templatePathPrefix          | `templatePathPrefix` filter uses `startsWith` matching                                                      |
| makeMatchLeaf — templatePath                | `templatePath` filter uses exact string matching                                                            |
| `getEntityIdsForNode` root click            | Clicking the root returns all entity IDs in the tree                                                        |
| `getEntityIdsForNode` cluster click         | Clicking a cluster node returns all entity IDs under that cluster (recursive)                               |
| `getEntityIdsForNode` rootApp click         | Clicking a rootApp node returns all entity IDs under that rootApp (recursive)                               |
| `getEntityIdsForNode` directory click       | Clicking a directory returns all entity IDs under it (recursive)                                            |
| `getEntityIdsForNode` subdirectory click    | Clicking a subdirectory returns only the entities in that subtree                                           |
| `getEntityIdsForNode` file click            | Clicking a file returns all entity IDs defined in that template                                             |
| `getEntityIdsForNode` entity leaf click     | Clicking an entity leaf returns exactly that one entity ID                                                  |
