# State Architecture

## Overview

All persistent UI state in Hydra UI is managed through a single unified state object (`HydraUiState`). This state is serialized to YAML, stored in **one** localStorage key (`hydra-state`), and auto-saved on every change. Default values are stripped from the YAML to keep it minimal — an empty YAML string means all defaults.

Default values are determined by a three-layer cascade: minimal **code defaults** are overridden by an external **YAML defaults file** (`hydra-ui-defaults.yaml`), producing the **effective defaults**. Only user changes that differ from the effective defaults are persisted in localStorage. See [Settings Defaults Cascade](#settings-defaults-cascade) for details.

**Source file:** `src/state.ts`

```text
HydraUiState
├── theme                    ThemeSetting           UI appearance
├── showDependencies         boolean                Dependency highlighting toggle
├── filterSidebar            boolean                Filter sidebar open/closed
├── searchGrouping           GroupingField[]        Nodes tree grouping
├── searchText               string                 Nodes search input
├── searchConfig             SearchConfig           Nodes search display/matching
├── graphGroupingConfig      GraphGroupingConfig    Graph layout & display
├── groupingKeys             GroupingKeyDefinition[] Custom entity categories
├── filterGroups             FilterGroupDefinition[] Predefined filter definitions (expression trees)
├── activeFilterSlots        ActiveFilterSlot[]     Runtime filter slots (OR-combined)
├── selectionSlot            ActiveFilterSlot|null  Transient selection filter from sidebar/graph clicks
├── globalChipStates         Record<string,boolean> Global chip active/inactive toggle map
├── colorRules               GraphColorRule[]       Color configuration
├── cloneRules               CloneRule[]            Entity cloning (manual)
├── autoClone                AutoCloneConfig        Threshold-based auto-cloning
├── sidebarWidth             number                 Sidebar size
├── expandedAccordions       string[]               Accordion state
├── entityListColumns        string[]               Visible entity list columns
├── entityListSort           EntityListSortEntry[]  Entity list column sort order
├── chartsViewMode           ChartsViewMode         Charts sidebar display mode
├── valuesViewMode           ValuesViewMode         Values tab display mode (tree/preview)
├── valuesDisabledSources    string[]               Hidden provenance source types in Values tab
├── valuesShowUnnecessary    string[]               Source types showing unnecessary-override warnings
├── valuesShowErrors         string[]               Source types showing new-key error markers
├── valuesHideGlobalClones   boolean                Hide dep.global clone sections (incl. nested sub-charts) in Values tab
├── valuesDisabledFiles      string[]               Hidden individual value files in Values tab
├── valuesSelectedFile       string                 Currently selected file in Values tree view
└── editorFontSize           number                 Font size for YAML editors and code panels (8–24px, default 13)
```

### Persistence Rules

1. There is only **one** localStorage key: `hydra-state`
2. No other localStorage keys or cookies are used. Navigation state (page, view, tab) is stored in the URL hash (see [URL-based Navigation State](#url-based-navigation-state) below).
3. Default values are **not** written to the YAML — "default" means the **effective defaults** (code defaults merged with YAML file defaults)
4. Missing fields are filled with effective defaults when loading
5. State is auto-saved on every change via `useEffect` in `App.tsx`
6. The State Editor (see [state-editor.md](state-editor.md)) allows viewing and editing the raw YAML

### Key Functions

| Function                                           | Description                                                                                     |
| -------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| `setExternalDefaults(yaml)`                        | Parses YAML defaults file and merges with code defaults to set the effective defaults           |
| `getEffectiveDefaults()`                           | Returns the current effective defaults (`HydraUiState`)                                         |
| `serializeState(state)`                            | Converts `HydraUiState` → YAML string (stripping effective defaults)                            |
| `serializeFullState(state)`                        | Converts `HydraUiState` → YAML string including all default values (for read-only display)      |
| `deserializeState(yaml)`                           | Parses YAML string → `HydraUiState` (filling effective defaults), returns `null` on parse error |
| `loadStateFromStorage()`                           | Reads from localStorage, returns `HydraUiState` (effective defaults if missing)                 |
| `saveStateToStorage(state)`                        | Writes serialized state to localStorage                                                         |
| `clearStateStorage()`                              | Clears all localStorage (used by "Reset settings")                                              |
| `syncEntityListColumns(columns, oldKeys, newKeys)` | Renames `gk:` columns when keys are renamed, removes orphaned `gk:` columns                     |
| `cleanEntityListColumns(columns, keys)`            | Removes `gk:` columns that don't match any existing key                                         |

### Data Flow

```text
hydra-ui-defaults.yaml        localStorage ("hydra-state")      URL hash (#cluster?page=...&node=...&tab=...)
    │                              │                                │
    ▼ fetch() on mount             │                                │
    │                              │                                │
setExternalDefaults(yaml)          │                                │
    │                              │                                │
    ▼ merges with CODE_DEFAULTS    ▼ loadStateFromStorage()         ▼ useHashNavigation()
    │                              │  (uses effective defaults)     │
    └──► effective defaults ──────►│                                │
                                   │                                │
                              App.tsx (initialState)            App.tsx (page, setPage, setTab)
                                   │                                │
                                   ├── individual useState() ──┐    ├── via props ──────────┐
                                   │   for each field          │    │                       │
                                   ▼                           │    ▼                       │
                              Components (via props)      currentState  GraphPanel      history.pushState()
                                   │                           │    │                       │
                                   ▼                           ▼    ▼                       ▼
                              User interaction          YAML in localStorage  Pages      URL hash


ClusterLoader (per clusterName, created in App.tsx via useMemo)
    │
    ├── loadHydraYaml()       → App.tsx useEffect (on mount / cluster change)
    │     tries /data/{cluster}/hydra.yaml → /{cluster}/hydra.yaml → /{cluster}.hydra.yaml
    │
    ├── loadManifest(path)    → EntityPage ManifestTabContent
    │     fetches /data/{cluster}/manifests/{path}
    │
    └── loadTemplate(path)    → EntityPage TemplateTabContent
          extracts chart name, fetches /data/{cluster}/charts/{chart}.tgz (cached),
          extracts file via extractFileFromTgz()

    All methods use safeFetch() with Content-Type guard (rejects HTML SPA fallback).
    .tgz cache: Map<url, Promise<ArrayBuffer>> on the instance — failed fetches are evicted.
    Passed as prop: App → GraphPanel → EntityPage.
```

#### Settings Page Data Flow (Edit-Copy Pattern)

The settings page receives the current live state as a bundled `EditableSettings` object. All edits happen on a local copy inside the component. Changes are only committed to the live state when the user clicks Save or Apply:

```text
App.tsx (live state)
    │
    ├── settings: EditableSettings ──────────► SettingsPage (edit copy)
    │                                              │
    │                                              ├── local useState() per field
    │                                              │
    │                                              ▼
    │                                          User edits (local only)
    │                                              │
    │   onApply(editState)                         │ Save / Apply
    ◄──────────────────────────────────────────────┘
    │
    ▼
App.tsx setters (setFilters, setGroupingKeys, ...)
```

The `EditableSettings` bundle includes: `groupingKeys`, `filterGroups`, `graphGroupingConfig`, `colorRules`, `cloneRules`, `autoClone`, `searchGrouping`, `searchConfig`.

---

## Settings Defaults Cascade

Default values for all settings are determined by a three-layer cascade. No business-logic defaults (such as grouping keys, color rules, or entity list columns) are hardcoded in the source code. Instead, they are defined in an external YAML file that can be changed without modifying or rebuilding the application.

### Layers

```text
Layer 1: CODE_DEFAULTS (src/state.ts)
    Minimal structural values. Ensures the app can render even if the YAML
    file fails to load.  Contains only type-safe zero values:
    theme: "auto", filters: [], groupingKeys: [], colorRules: [],
    entityListColumns: ["id", "entityGroup", "name", "tags"], etc.

        │  merge (YAML overrides code)
        ▼

Layer 2: YAML File Defaults (public/hydra-ui-defaults.yaml)
    Configurable defaults loaded via fetch() on app startup.
    Contains business-logic defaults such as the "Type" grouping key,
    default color rules, default entity list columns with gk: references.

        = Effective Defaults (code + YAML merged)

        │  merge (localStorage overrides effective defaults)
        ▼

Layer 3: localStorage ("hydra-state")
    User-specific overrides. Only values that DIFFER from effective defaults
    are stored.  An empty YAML string means all effective defaults.
```

### Merge Strategy

- **Scalars** (string, number, boolean): YAML value replaces code value
- **Arrays**: YAML array replaces code array entirely (no element-level merge)
- **Objects**: deep merge — each YAML field overrides the corresponding code field; missing fields keep the code default

The merge uses the same logic as `applyDefaults()`: the YAML file content is treated as a partial `HydraUiState`, and code defaults fill any missing fields.

### YAML Defaults File

**Path:** `public/hydra-ui-defaults.yaml` (served as a static asset, fetched via `fetch("/hydra-ui-defaults.yaml")`)

The file uses the same YAML format as the localStorage state. Only fields that should differ from code defaults need to be specified. Example:

```yaml
groupingKeys:
  - name: Type
    entries:
      - key: Kubernetes
        field: group
        values:
          - ""
          - apps
          - batch
          - autoscaling
          - networking.k8s.io
          - rbac.authorization.k8s.io
          - policy
          - storage.k8s.io
          - apiextensions.k8s.io
          - admissionregistration.k8s.io
          - certificates.k8s.io
          - coordination.k8s.io
          - discovery.k8s.io
          - events.k8s.io
          - scheduling.k8s.io
          - node.k8s.io
          - flowcontrol.apiserver.k8s.io
      - key: ArgoCD
        field: group
        values:
          - argoproj.io
      - key: Cluster Infra
        field: group
        values:
          - cert-manager.io
          - isindir.github.com
          - kyverno.io
          - monitoring.coreos.com
      - key: Demo Infra
        field: group
        values:
          - clickhouse.altinity.com
          - kafka.strimzi.io
    fallbackKey: other

entityListColumns:
  - id
  - "gk:Type"
  - entityGroup
  - name
  - tags

colorRules:
  - field: namespace
    value: ""
    target: group
    mode: color
    color: "#616161"
```

### Initialization Sequence

```text
1. App mounts
2. fetch("/hydra-ui-defaults.yaml")
   ├── success: setExternalDefaults(yamlText)  →  effective defaults = merge(CODE_DEFAULTS, parsed YAML)
   └── failure: effective defaults remain CODE_DEFAULTS (graceful fallback)
3. loadStateFromStorage()  →  merges localStorage with effective defaults
4. App renders with final state
```

A brief loading indicator is shown during step 2. The fetch is fast (local static asset) so the delay is minimal.

### Reset Behavior

"Reset all settings" in the settings page calls `clearStateStorage()` and reloads the page. On reload, the initialization sequence runs again: the YAML defaults file is re-fetched, effective defaults are recomputed, and since localStorage is empty, the user sees the effective defaults (code + YAML merged).

### Key Implementation Details

- `CODE_DEFAULTS` is a private constant in `src/state.ts` — not exported.
- `effectiveDefaults` is a module-level mutable variable in `src/state.ts`, initialized to `CODE_DEFAULTS`.
- `setExternalDefaults(yaml)` parses the YAML string, merges with `CODE_DEFAULTS` via `applyDefaults()`, and assigns the result to `effectiveDefaults`.
- `getEffectiveDefaults()` returns the current `effectiveDefaults` — used by components that need to reference the active defaults.
- `stripDefaults()` and `applyDefaults()` both reference `effectiveDefaults` instead of a hardcoded constant.
- `src/groupingKeyLogic.ts` no longer defines a built-in "Type" grouping key constant. The default key definitions live exclusively in `hydra-ui-defaults.yaml`.

---

## Sub-States in Detail

### `theme` — Theme Setting

```typescript
type ThemeSetting = "light" | "dark" | "auto";
```

| Value     | Behavior                                             |
| --------- | ---------------------------------------------------- |
| `"auto"`  | Follows the operating system's light/dark preference |
| `"light"` | Forces light theme                                   |
| `"dark"`  | Forces dark theme                                    |

**Default:** `"auto"`

Managed by `ThemeWrapper.tsx` (reads initial value from YAML state on mount). The theme toggle in the sidebar header cycles through `auto → light → dark → auto`.

---

### `showDependencies` — Dependency Highlighting Toggle

```typescript
showDependencies: boolean;
```

**Default:** `true`

Controls whether dependency highlighting (reachability highlighting, distance badges, colored edges) is shown for selected nodes. When `true`, selecting nodes shows their dependency chain. When `false`, only the selected nodes themselves are visually marked without any dependency highlighting.

Toggled via the Dependencies button in the graph panel toolbar (see [sidebar.md § Graph Panel Toolbar](sidebar.md#graph-panel-toolbar)).

---

### `filterSidebar` — Filter Sidebar Toggle

```typescript
filterSidebar: boolean;
```

**Default:** `false`

Controls whether the filter sidebar is expanded. When `true`, the Filter accordion section is visible in the sidebar.

---

### `searchGrouping` — Nodes Tree Grouping

```typescript
searchGrouping: GroupingField[];
```

**Default:** `["namespace", "entityGroup"]`

The grouping fields used for the Nodes tree view. Determines how entities are hierarchically nested in the Nodes accordion.

Available values: `"namespace"`, `"entityGroup"`, `"kind"`, `"apiVersion"`, `"group"`, `"gvk"`, `"gvkn"`, `"nscs"`, `"name"`, `"version"`.

---

### `searchText` — Search Input

```typescript
searchText: string;
```

**Default:** `""`

The current text in the search input field. Search filtering is debounced (200ms) for performance, but the raw text is persisted immediately.

---

### `searchConfig` — Search Configuration

```typescript
type SearchConfig = {
  leafNameFields: LeafField[]; // fields composing the leaf node name
  leafNameSeparators: string[]; // separators between name fields
  leafDescriptionFields: LeafField[]; // fields composing the leaf description
  leafDescriptionSeparators: string[]; // separators between description fields
  searchFields: SearchField[]; // fields used for search string matching
  searchSeparators: string[]; // separators between search fields
  startSeparator: string; // prefix for the search string
  endSeparator: string; // suffix for the search string
};
```

**Defaults:**

| Field                       | Default                 |
| --------------------------- | ----------------------- |
| `leafNameFields`            | `["kind", "name"]`      |
| `leafNameSeparators`        | `[" "]`                 |
| `leafDescriptionFields`     | `["id"]`                |
| `leafDescriptionSeparators` | `[]`                    |
| `searchFields`              | `["entityGroup", "id"]` |
| `searchSeparators`          | `[":"]`                 |
| `startSeparator`            | `"^"`                   |
| `endSeparator`              | `"$"`                   |

Controls how leaf entities are displayed in the Nodes tree and how search strings are constructed for matching. Configurable via the search settings panel (⚙ toggle in the Nodes accordion).

**Field types:**

- `LeafField`: `"name"`, `"id"`, `"gvk"`, `"gvkn"`, `"group"`, `"version"`, `"kind"`, `"namespace"`, `"templatePath"`, `"tags"`
- `SearchField`: `"id"`, `"name"`, `"gvk"`, `"gvkn"`, `"path"`, `"leafName"`, `"leafDescription"`, `"entityGroup"`

---

---

### `graphGroupingConfig` — Graph Grouping Configuration

```typescript
type GraphGroupingConfig = {
  topLevelLayout: LayoutDirection;
  levels: Array<{
    field: GroupingField;
    layout: LayoutDirection;
    display: GroupingLevelDisplay;
  }>;
  nodeDisplay: GroupingLevelDisplay;
};

type LayoutDirection = "horizontal" | "vertical";
```

**Default:**

```yaml
topLevelLayout: horizontal
levels:
  - field: nscs
    layout: vertical
    display:
      { header: [label], description: [(, itemCount, Items)], tooltip: [label] }
  - field: entityGroup
    layout: vertical
    display:
      { header: [label], description: [(, itemCount, Items)], tooltip: [label] }
nodeDisplay:
  header: [{ type: field, field: kind }]
  description: [{ type: field, field: name }]
  tooltip:
    [
      { type: field, field: id },
      { type: text, text: "\n" },
      { type: field, field: templatePath },
    ]
```

Controls the graph tree hierarchy, layout direction at each depth, and display labels. Color configuration and entity cloning are managed separately (see `colorRules` and `cloneRules` below). See [layout.md](layout.md) for the full data model documentation.

**Sub-types:**

- **`GroupingLevelDisplay`**: Configures `header`, `description`, and `tooltip` for groups/nodes as arrays of `GroupDisplayFieldEntry` (field, label, text, or itemCount).

---

### `groupingKeys` — Custom Grouping Keys

```typescript
type GroupingKeyEntry = {
  key: string; // resolved output key (e.g. "true", "false")
  field: string; // source field for this entry (e.g. "group", "namespace", "gk:OtherKey")
  values: string[]; // field values that map to this key
  pathLevel?: number; // directory depth (only for field === "templatePath")
};

type GroupingKeyDefinition = {
  name: string; // unique name (e.g. "Type")
  entries: GroupingKeyEntry[]; // key/value pairs — first matching entry wins; each entry has its own field
  fallbackKey: string; // key for entities that match no entry
};
```

**Code default:** `[]` (no keys). The effective default is loaded from `hydra-ui-defaults.yaml` and includes one key named `"Type"` with entries `"Kubernetes"`, `"ArgoCD"`, `"Cluster Infra"`, `"Demo Infra"` (all matching by `field: "group"`), `fallbackKey: "other"`. See [Settings Defaults Cascade](#settings-defaults-cascade) for the full YAML and [grouping-keys.md](grouping-keys.md) for examples and the resolution algorithm.

Each grouping key is evaluated independently for every entity. Each entry specifies its own source field, allowing entries within the same key to match on different entity properties (e.g. one entry by namespace, another by kind). The resolved key string (from `entries` or `fallbackKey`) is available as column `gk:<keyName>` in the entity list.

---

### `filterGroups` — Predefined Filter Definitions

```typescript
type FilterExprNode = FilterExprLeaf | FilterExprRef | FilterExprGroup;

type FilterGroupDefinition = {
  name: string; // unique display name
  description: string; // Markdown description
  root: FilterExprNode; // expression tree root
};
```

**Default:** `[]` (no filter groups)

Predefined filter expression definitions (recursive expression trees) that can be applied as presets. Each definition contains a root expression node that can combine filters with AND/OR operators, negation, and references to other predefined filters. Managed in the Settings page (Predefined Filters tab).

See [filters.md](filters.md) for the full expression tree data model, UI behavior, and algorithm documentation.

---

### `activeFilterSlots` — Active Filter Slots

```typescript
type ActiveFilterSlot = {
  root: FilterExprNode; // local copy of expression tree
  editing: boolean; // false = view mode (chips), true = editor mode
  chipStates: Record<string, boolean>; // per-slot chip active/inactive toggle
};
```

**Default:** `[]` (no active filter slots)

Runtime filter instances displayed in the sidebar's Filter accordion. Each slot holds a local copy of an expression tree. Slots are OR-combined for entity filtering. Each slot has view/edit modes and per-slot `chipStates` for toggling individual filter chips active/inactive.

See [filters.md](filters.md) for full architecture, data flow, and algorithm documentation.

---

### `selectionSlot` — Selection Filter

```typescript
selectionSlot: ActiveFilterSlot | null;
```

**Default:** `null` (no selection active)

A transient filter created by selection actions (sidebar Files/Groups clicks or graph node/group clicks). When active, the Entity List shows only entities matching the selection, ignoring global filter slots. The selection can be pinned (converted to a permanent global slot) or cleared.

See [filters.md](filters.md) for details on selection behavior and the Filter Slots Panel UI.

---

### `globalChipStates` — Global Chip Toggle Map

```typescript
globalChipStates: Record<string, boolean>;
```

**Default:** `{}` (all chips active by default)

A shared map of chip key → active/inactive state. Toggling a chip key (e.g. `namespace:default`) in one filter slot updates its state globally across all slots and the selection filter. A `false` value means the chip is inactive (skipped during evaluation); absent keys are treated as active.

---

### `colorRules` — Color Rules

```typescript
type ColorRuleTarget = "group" | "node" | "all";
type ColorRuleMode = "unchanged" | "color" | "auto";

type GraphColorRule = {
  field: GroupingField | `gk:${string}`; // grouping field or dynamic grouping key (e.g. "gk:Kubernetes Standard API")
  value: string; // value to match (e.g. "default", "kube-system", "Pod")
  target: ColorRuleTarget; // apply to group boxes, entity nodes, or both
  mode: ColorRuleMode; // unchanged / color / auto
  color?: string; // hex color, only used when mode === "color"
};
```

**Code default:** `[]` (no rules). The effective default is loaded from `hydra-ui-defaults.yaml`: `[{ field: "namespace", value: "", target: "group", mode: "color", color: "#616161" }]` (cluster-scoped entity groups are colored dark grey).

A flat list of color rules, decoupled from the grouping hierarchy. Each rule matches elements based on a grouping field value and applies a color mode. Rules are evaluated top-down — first matching rule wins. See [layout.md § Color Configuration](layout.md#color-configuration) for the full data model documentation.

**Color modes:**

| Mode        | Effect                                                     |
| ----------- | ---------------------------------------------------------- |
| `unchanged` | No color change (keeps default). Previously called `none`. |
| `auto`      | Automatic color from palette (hash of value).              |
| `color`     | Manually chosen color (hex). Previously called `fixed`.    |

---

### `cloneRules` — Entity Clone Rules

```typescript
type CloneRule = {
  field: LeafFieldType; // entity field to match (kind, gvk, name, ...)
  value: string; // value to match (e.g. "CustomResourceDefinition")
  per: GroupingField; // determines clone placement and target discovery strategy
};
```

**Default:** `[]` (no cloning)

Configures manual entity clone rules. Each rule clones heavily-linked entities into every tree branch where their entity group has members, replacing the original and eliminating long cross-graph edges. The `per` field determines placement: `entityGroup` places clones inside the entity group (targets from group members' nscs values), `nscs`/`namespace` places clones ungrouped at the namespace level (targets from referencing entities). See [layout.md § Entity Cloning](layout.md#entity-cloning) for the full algorithm and examples.

---

### `autoClone` — Auto-Clone Configuration

```typescript
type AutoCloneConfig = {
  enabled: boolean; // whether auto-clone is active
  thresholdIn: number; // minimum incoming edge count to trigger cloning
  thresholdOut: number; // minimum outgoing edge count to trigger cloning
  per: GroupingField; // grouping level for clone placement
};
```

**Default:** `{ enabled: true, thresholdIn: 10, thresholdOut: 10, per: <last active grouping level> }`

Configures automatic clone rule generation based on edge counts (excluding reverse edges). When enabled, entities with more incoming edges than `thresholdIn` OR more outgoing edges than `thresholdOut` automatically receive clone rules matching by their exact entity ID. The `per` field defaults to the deepest (last) active grouping level — i.e. the level that produces leaf groups with no sub-groups. When the grouping configuration changes and the current `per` value is no longer an active level, `per` falls back to the new deepest level.

Auto-generated rules are **prepended** before manual `cloneRules` when passed to `cloneEntities()`, so they take precedence. Auto-generated rules are displayed in the UI as read-only entries (no edit/delete) and are visually distinguished from manual rules.

See [layout.md § Auto-Clone](layout.md#auto-clone-threshold) for the full algorithm, data flow, and UI description.

---

### `sidebarWidth` — Sidebar Width

```typescript
sidebarWidth: number;
```

**Default:** `340`

Width of the sidebar in pixels. Constrained to 170–600px. Adjusted by dragging the resize handle between the sidebar and the graph panel.

---

### `expandedAccordions` — Expanded Accordion Sections

```typescript
expandedAccordions: string[];
```

**Default:** `[]` (all collapsed)

List of currently expanded accordion section IDs. Valid IDs: `"filter"`, `"files"`, `"nodes"`, `"groups"`. Multiple sections can be expanded simultaneously.

---

### `entityListColumns` — Entity List Columns

```typescript
entityListColumns: string[];
```

**Code default:** `["id", "entityGroup", "name", "tags"]`. The effective default is loaded from `hydra-ui-defaults.yaml`: `["id", "gk:Type", "entityGroup", "name", "tags"]`.

Controls which columns are visible in the Entity List view. Each entry is a column identifier — either a built-in field (`"id"`, `"name"`, `"kind"`, `"namespace"`, `"group"`, `"version"`, `"apiVersion"`, `"gvk"`, `"entityGroup"`, `"tags"`, `"templatePath"`) or a dynamic grouping key column (`"gk:<keyName>"`).

**Automatic sync with groupingKeys:** When grouping keys are renamed, `gk:` column references are automatically updated to match the new name. When a grouping key is deleted or a `gk:` reference becomes invalid (e.g. from persisted state), the stale column is automatically removed. See [grouping-keys.md § entityListColumns ↔ groupingKeys Sync](grouping-keys.md#entitylistcolumns--groupingkeys-sync).

---

### `entityListSort` — Entity List Sort Order

```typescript
type EntityListSortEntry = { field: string; order: 1 | -1 };
```

**Default:** `[]` (no sorting — table shows entities in their natural order)

Stores the active multi-column sort state for the Entity List table. Each entry specifies a column field and its sort direction (`1` for ascending, `-1` for descending). Multiple entries enable multi-column sorting (hold Ctrl/Meta and click column headers). The order of entries determines sort priority.

Passed as `multiSortMeta` to PrimeReact's `DataTable`. Sort changes are persisted immediately via the unified state auto-save.

---

### `chartsViewMode` — Charts Sidebar Display Mode

```typescript
type ChartsViewMode = "tree" | "flat";
```

**Default:** `"tree"`

Controls how charts are displayed in the Charts sidebar accordion.

| Value    | Behavior                                                                                                                   |
| -------- | -------------------------------------------------------------------------------------------------------------------------- |
| `"tree"` | Hierarchical view: charts grouped by root app ID, with dependencies as nested children (including transitive dependencies) |
| `"flat"` | Flat list: all charts and all their dependencies shown as root-level nodes (deduplicated by name), sorted alphabetically   |

Toggled via a `SelectButton` (Tree / Flat) in the Charts accordion, next to the search input.

---

### `valuesViewMode` — Values Tab Display Mode

```typescript
type ValuesViewMode = "tree" | "preview";
```

**Default:** `"tree"`

Controls how values are displayed in the Chart Page Values tab.

| Value       | Behavior                                                                          |
| ----------- | --------------------------------------------------------------------------------- |
| `"tree"`    | Split-panel with file tree (left) and raw YAML viewer (right)                     |
| `"preview"` | Merged values with provenance annotations, coloured gutter markers, and filtering |

Toggled via a segmented button in the Values tab toolbar. See [chart-page.md](chart-page.md) for the full Values tab documentation.

---

### `valuesShowErrors` — Error Marker Source Types

```typescript
valuesShowErrors: string[];
```

**Default:** `["chartDefaults", "group", "context", "cluster", "app"]`

Source types for which "new key" error markers are displayed in the Values preview. Controlled via the "Errors" checkbox column in the Values tree view.

---

### `valuesDisabledFiles` — Hidden Individual Value Files

```typescript
valuesDisabledFiles: string[];
```

**Default:** `[]` (no files hidden)

List of individual value file paths that are hidden in the Values preview. Controlled via the "Visible" checkbox column in the Values tree view. When a file path is in this array, its lines are excluded from the merged preview.

---

### `valuesSelectedFile` — Selected File in Values Tree

```typescript
valuesSelectedFile: string;
```

**Default:** `""` (no file selected)

The currently selected file in the Values tree view (left pane). Clicking a file node in the tree sets this value and displays the raw YAML content in the right pane.

---

### `editorFontSize` — Editor Font Size

```typescript
editorFontSize: number;
```

**Default:** `13`

Font size in pixels for YAML editors, Chart.yaml viewer, and other code panels. Valid range is 8–24px. Configurable in Settings > Appearance tab.

---

## URL-based Navigation State

Navigation state that benefits from browser history (back/forward) is stored in the URL hash instead of localStorage. This enables deep-linking to specific pages and browser back/forward navigation between page views.

**Source file:** `src/useHashNavigation.ts`

### Hash Format

The URL hash encodes both the cluster name and optional navigation parameters:

```text
#<clusterName>?page=<page>&node=<entityId>&tab=<tab>
```

| Parameter     | Description                                                                                                                                                                                                                                          |
| ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `clusterName` | The cluster to load data for (default: `in-cluster`). The part before the first `?`.                                                                                                                                                                 |
| `page`        | Top-level page: `list` (default), `graph`, `charts`, `details`, `settings` (optional)                                                                                                                                                                |
| `node`        | The entity node ID for the details page (optional, only for `page=details`)                                                                                                                                                                          |
| `tab`         | Active sub-tab within a page (optional). For details page: `details` (default), `relations`, `outgoing-rbac`, `incoming-rbac`, `secrets`, `clone`. For settings page: `groupingKeys` (default), `filterGroups`, `graph`, `search`, `state`, `reset`. |

Filters are managed exclusively via `activeFilterSlots` in localStorage (see [filters.md](filters.md)). Old filter URL parameters (`f.`, `x.`, `n.`) are ignored if present.

**Examples:**

| URL                                                                                               | Meaning                                       |
| ------------------------------------------------------------------------------------------------- | --------------------------------------------- |
| `#my-cluster`                                                                                     | List page (default)                           |
| `#my-cluster?page=graph`                                                                          | Graph page                                    |
| `#my-cluster?page=charts`                                                                         | Charts page                                   |
| `#my-cluster?page=details&node=apps/v1/Deployment/default/nginx`                                  | Details page for nginx (default tab: details) |
| `#my-cluster?page=details&node=apps/v1/Deployment/default/nginx&tab=relations`                    | Details page, relations tab                   |
| `#my-cluster?page=details&node=apps/v1/Deployment/default/nginx&tab=outgoing-rbac`                | Details page, outgoing RBAC tab               |
| `#my-cluster?page=details&node=rbac.authorization.k8s.io/v1/ClusterRole//admin&tab=incoming-rbac` | Details page, incoming RBAC tab               |
| `#my-cluster?page=details&node=v1/Secret/default/my-secret&tab=secrets`                           | Details page, secrets tab                     |
| `#my-cluster?page=settings`                                                                       | Settings page (default tab: groupingKeys)     |
| `#my-cluster?page=settings&tab=graph`                                                             | Settings page, graph tab                      |

### PageType

```typescript
type PageType = "list" | "graph" | "charts" | "details" | "settings";
```

Defined in `src/state.ts`. Used by `useHashNavigation` and passed to components via props.

The `page` parameter determines which top-level page is shown. The `tab` parameter determines which sub-tab within that page is active.

The details page uses a unified **Entity Page** component with tabs (see [rbac-display.md § Entity Page](rbac-display.md#entity-page)). The `tab` parameter determines which tab is active (`details`, `relations`, `outgoing-rbac`, `incoming-rbac`, `secrets`, `clone`); the `node` parameter determines which entity is shown. Switching entity tabs uses `setPage("details", nodeId, newTab)` which creates a browser history entry. Clicking entity links changes `node` while keeping the current tab.

The settings page also uses `tab` for sub-tab navigation. Settings tab changes use `replaceState` (no history entry), so switching tabs within settings does not clutter the browser back/forward history.

### Hook API

```typescript
type HashNavState = {
  clusterName: string;
  page: PageType;
  nodeId: string;
  tab?: string;
};

function useHashNavigation(): [
  HashNavState,
  {
    setPage: (page: PageType, nodeId?: string, tab?: string) => void;
    setTab: (tab: string | undefined) => void;
    goBack: () => void;
  },
];
```

| Method      | Behavior                                                                                                                 |
| ----------- | ------------------------------------------------------------------------------------------------------------------------ |
| `setPage()` | Navigates to a page via `pushState` (creates history entry). Pass `nodeId` for details page, `tab` for specific sub-tab. |
| `setTab()`  | Updates the active sub-tab via `replaceState` (no history entry — tab switches don't clutter back/forward)               |
| `goBack()`  | Calls `history.back()` to return to previous state                                                                       |

| Behavior         | Details                                                                           |
| ---------------- | --------------------------------------------------------------------------------- |
| **Read**         | Parses `window.location.hash` on mount and on every `popstate`/`hashchange` event |
| **Write**        | `setPage` uses `pushState`; `setTab` uses `replaceState`                          |
| **Back/Forward** | `popstate` event listener updates the React state to match the URL                |
| **1:1 sync**     | Any change to the URL hash is immediately reflected in the returned state         |

### Pure helpers (exported for testing)

| Function                                     | Description                                                         |
| -------------------------------------------- | ------------------------------------------------------------------- |
| `parseHash(hash)`                            | Parses a hash string into `HashNavState` (including `tab` from URL) |
| `buildHash(clusterName, page, nodeId, tab?)` | Builds a hash string from navigation state (includes `tab` if set)  |

---

## Adding a New Setting

1. Add the field to the appropriate type in `src/state.ts`:
   - `HydraUiState` for top-level settings
   - `GraphGroupingConfig` for graph/grouping settings
   - `SearchConfig` for search settings
   - Or create a new sub-type
2. Add a minimal structural default in `CODE_DEFAULTS` (the private constant in `src/state.ts`). This is the fallback value when the YAML defaults file is unavailable.
3. If the setting should have a non-trivial default (e.g. a pre-configured list), add it to `public/hydra-ui-defaults.yaml`. The YAML value overrides the code default.
4. Update `stripDefaults()` to omit the default value during serialization (comparison is against effective defaults)
5. Update `applyDefaults()` to fill the default value when the field is missing (uses effective defaults)
6. Add a `useState()` in `App.tsx` and wire it to the component via props
7. Include the field in `currentState` (the `useMemo` in `App.tsx`)
8. Update `handleApplyStateYaml` to set the new field from parsed YAML
9. If the setting is editable in the settings page, add it to the `EditableSettings` type in `SettingsPage.tsx` and include it in `currentEditableSettings` and `handleApplySettings` in `App.tsx`
10. **Do NOT** create a new `localStorage.setItem()` / `localStorage.getItem()` call
11. Update this documentation (`STATE.md`) and the relevant architecture docs
12. For **navigation state** (state that should support browser back/forward), use the URL hash via `useHashNavigation` instead of localStorage. Do **not** add navigation-related state to `HydraUiState`.

## Tests

### State Serialization Tests (`src/__tests__/state.test.ts`)

Tests for `serializeState()`, `deserializeState()`, `getEffectiveDefaults()`, and `DEFAULT_NODE_DISPLAY` in `src/state.ts`.

| Test                                                          | Verifies                                                                                      |
| ------------------------------------------------------------- | --------------------------------------------------------------------------------------------- |
| `showDependencies` default                                    | `getEffectiveDefaults().showDependencies` is `true`                                           |
| `showDependencies` not serialized when default                | `serializeState` omits `showDependencies` when `true` (default stripping)                     |
| `showDependencies` serialized when `false`                    | `serializeState` includes `showDependencies: false` for non-default value                     |
| `showDependencies` deserialized to `true` when missing        | `deserializeState("")` fills default `true`                                                   |
| `showDependencies` deserialized `false` correctly             | `deserializeState("showDependencies: false")` returns `false`                                 |
| Node tooltip default includes templatePath                    | `DEFAULT_NODE_DISPLAY.tooltip` contains `[field:id, text:"\n", field:templatePath]`           |
| Node tooltip in effective defaults                            | `getEffectiveDefaults().graphGroupingConfig.nodeDisplay.tooltip` matches the default tooltip  |
| Accordion section IDs serialize correctly                     | New section IDs (`files`, `nodes`, `groups`) are serialized to YAML                           |
| Accordion section IDs deserialize correctly                   | YAML with new section IDs round-trips to the correct `expandedAccordions` array               |
| Accordion section IDs backwards compatibility                 | Old section IDs (`quickSearch`, `graph`) are accepted during deserialization                  |
| `GroupingKeyDefinition` serialize with entry-level field      | Key with `name`, `entries[].field`, `fallbackKey` serialized correctly                        |
| `GroupingKeyDefinition` serialize entry with `pathLevel`      | Key with `field: templatePath` and `pathLevel: 1` is serialized correctly                     |
| `GroupingKeyDefinition` deserialize with entry-level field    | YAML with per-entry `field` is deserialized to the correct `GroupingKeyDefinition`            |
| `GroupingKeyDefinition` migrate old definition-level field    | Old format with `field` on definition level is migrated to entry-level during deserialization |
| `GroupingKeyDefinition` round-trip `pathLevel: 0`             | Key with `pathLevel: 0` survives serialize → deserialize round-trip                           |
| `GroupingKeyDefinition` deserialize entries with mixed fields | Entries using different source fields (`namespace`, `kind`) deserialize correctly             |
| Default state serializes to empty string                      | `serializeState(getEffectiveDefaults())` returns `""` (all defaults stripped)                 |
| Empty string deserializes to default state                    | `deserializeState("")` fills all effective default values correctly                           |

### Settings Defaults Cascade Tests (`src/__tests__/state.test.ts`)

| Test                                                 | Verifies                                                                                                                                                      |
| ---------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `setExternalDefaults` merges YAML with code defaults | After calling `setExternalDefaults` with a YAML containing `groupingKeys`, `getEffectiveDefaults()` includes the keys while other fields retain code defaults |
| `setExternalDefaults` replaces arrays entirely       | YAML `entityListColumns` replaces the code default list, does not append                                                                                      |
| `stripDefaults` uses effective defaults              | After `setExternalDefaults`, values matching the YAML defaults are stripped from serialized output                                                            |
| `deserializeState` fills effective defaults          | After `setExternalDefaults`, deserializing an empty string returns effective defaults including YAML-provided values                                          |
| Fallback on invalid YAML                             | `setExternalDefaults` with invalid YAML keeps effective defaults at code defaults                                                                             |
| `resetEffectiveDefaults` restores code defaults      | After reset, `getEffectiveDefaults()` returns code defaults                                                                                                   |

### entityListColumns ↔ groupingKeys Sync Tests (`src/__tests__/state.test.ts`)

| Test                                    | Verifies                                                                       |
| --------------------------------------- | ------------------------------------------------------------------------------ |
| `syncEntityListColumns` rename          | `gk:` columns are rewritten when a grouping key is renamed at the same index   |
| `syncEntityListColumns` remove          | `gk:` columns referencing deleted keys are removed                             |
| `syncEntityListColumns` non-gk columns  | Non-`gk:` columns are always preserved unchanged                               |
| `syncEntityListColumns` rename + delete | Rename and deletion in a single operation                                      |
| `cleanEntityListColumns` remove invalid | Stale `gk:` references are removed based on current keys                       |
| `cleanEntityListColumns` keep valid     | Valid `gk:` references are preserved                                           |
| `cleanEntityListColumns` empty keys     | All `gk:` columns removed when no keys exist                                   |
| Deserialization cleanup                 | Stale `gk:` references in persisted YAML are cleaned during `deserializeState` |

### Filter Groups State Tests (`src/__tests__/state.test.ts`)

| Test                                       | Verifies                                                                  |
| ------------------------------------------ | ------------------------------------------------------------------------- |
| `filterGroups` not serialized when default | Empty `filterGroups` array is stripped from YAML                          |
| `filterGroups` serialized when non-empty   | Non-empty `filterGroups` appear in serialized YAML                        |
| `filterGroups` deserialized correctly      | YAML with `filterGroups` round-trips to correct `FilterGroupDefinition[]` |
| `filterGroups` fills default when missing  | Missing `filterGroups` in YAML gets default `[]`                          |
| `filterGroups` round-trip                  | `filterGroups` survives serialize/deserialize cycle                       |

### Filter Group Logic Tests (`src/__tests__/filterGroupLogic.test.ts`)

| Test                                                        | Verifies                                                              |
| ----------------------------------------------------------- | --------------------------------------------------------------------- |
| `expandFilterGroup` flat filters only                       | Group with only `filter` members expands to sorted, deduplicated list |
| `expandFilterGroup` with group reference                    | Group referencing another group includes all referenced filters       |
| `expandFilterGroup` nested group references                 | Multi-level nesting resolves correctly                                |
| `expandFilterGroup` deduplication                           | Duplicate filters across members are deduplicated                     |
| `expandFilterGroup` invalid group reference                 | Reference to non-existent group is ignored                            |
| `expandFilterGroup` forward reference ignored               | Reference to a later-defined group is ignored                         |
| `expandFilterGroup` mixed filter types                      | Namespace, kind, tags filters all handled correctly                   |
| `matchFilterGroups` exact match                             | Group whose filters exactly match active filters is returned          |
| `matchFilterGroups` subset match                            | Group whose filters are a subset of active filters is returned        |
| `matchFilterGroups` partial mismatch                        | Group with filters not in active set is not returned                  |
| `matchFilterGroups` disabled filters ignored                | Filters with `mode: "disabled"` don't count                           |
| `matchFilterGroups` inactive filters ignored                | Filters with `mode: "inactive"` don't count                           |
| `matchFilterGroups` no matching filters                     | Unrelated filters match no groups                                     |
| `matchFilterGroups` multiple groups                         | Multiple groups can match simultaneously                              |
| `addFilterGroupFilters` adds missing filters                | Filters not yet present are added as enabled                          |
| `addFilterGroupFilters` no duplicates                       | Already-present filters are not duplicated                            |
| `addFilterGroupFilters` reactivates disabled                | Disabled filters are set to enabled                                   |
| `removeFilterGroupFilters` removes unshared filters         | Filters unique to the removed group are removed                       |
| `removeFilterGroupFilters` preserves shared filters         | Filters shared with other matched groups are kept                     |
| `removeFilterGroupFilters` removes all when no other groups | All group filters removed when no other groups active                 |
| `removeFilterGroupFilters` preserves unrelated filters      | Non-group filters are not affected                                    |
| `removeFilterGroupFilters` unknown group                    | Unknown group name returns unchanged filters                          |
