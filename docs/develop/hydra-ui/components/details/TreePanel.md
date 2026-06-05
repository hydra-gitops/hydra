# TreePanel

**File:** `src/components/TreePanel.tsx`

## Function

Multi-purpose file that exports several related components for the sidebar and settings:

### TreePanel

Sidebar grouping tree that mirrors the graph's group hierarchy. When the graph is visible, shows checkboxes to expand/collapse individual groups. Supports clicking group labels to set a filter. Displays selection match highlighting.

### SearchTree

Sidebar search panel with a debounced text input and a hierarchical tree of entities grouped by configurable fields (namespace, entity group, etc.). Clicking an entity leaf navigates to entity details. Supports quick-add for search config fields.

### GraphSettings

Settings component for graph grouping configuration. Allows configuring:

- Top-level layout direction (horizontal/vertical)
- Grouping levels with field, layout direction, and display config
- Node display configuration (header, description, tooltip)
- Colour rules (field/value → colour mapping)
- Clone rules (manual entity cloning)
- Auto-clone configuration (threshold-based)

### SearchSettings

Settings component for search tree configuration. Allows configuring grouping fields, leaf name/description fields, search fields, and separators.

## Exports

- `TreePanel` — Grouping tree for sidebar
- `SearchTree` — Search tree for sidebar
- `GroupingConfig` — Grouping level editor (used internally)
- `GraphSettings` — Graph settings editor
- `SearchSettings` — Search settings editor
- `GroupingField` — Type alias (re-exported from `treeLogic`)

## Props (TreePanel)

| Prop                    | Type                                                    | Description                            |
| ----------------------- | ------------------------------------------------------- | -------------------------------------- |
| `entities`              | `HydraEntity[]`                                         | Entities to group                      |
| `groups`                | `HydraGroup[]`                                          | Group definitions                      |
| `references`            | `HydraReference[]`                                      | References (for edge counts)           |
| `groupingConfig`        | `GraphGroupingConfig`                                   | Current graph grouping configuration   |
| `edgeCounts`            | `EdgeCounts`                                            | Edge counts (for auto-clone display)   |
| `selectionMatchIds`     | `Set<string>`                                           | IDs of matching nodes for highlighting |
| `expandedItems`         | `string[]`                                              | Currently expanded group keys          |
| `onExpandedItemsChange` | `(items: string[]) => void`                             | Called when expanded items change      |
| `isGraphVisible`        | `boolean`                                               | Whether the graph is currently visible |
| `onFilterByGroup`       | `(filters: { field: string; value: string }[]) => void` | Called on group label click            |

## Props (SearchTree)

| Prop                   | Type                                | Description                           |
| ---------------------- | ----------------------------------- | ------------------------------------- |
| `entities`             | `HydraEntity[]`                     | Entities to search/group              |
| `groups`               | `HydraGroup[]`                      | Group definitions                     |
| `grouping`             | `GroupingField[]`                   | Grouping field hierarchy              |
| `onGroupingChange`     | `(fields: GroupingField[]) => void` | Called when grouping changes          |
| `onSelectEntity`       | `(entityId: string) => void`        | Called when an entity is clicked      |
| `searchText`           | `string`                            | Current search text                   |
| `onSearchTextChange`   | `(text: string) => void`            | Called when search text changes       |
| `searchConfig`         | `SearchConfig`                      | Search display/matching configuration |
| `onSearchConfigChange` | `(config: SearchConfig) => void`    | Called when search config changes     |

## Used by

- `src/App.tsx` — `TreePanel` in the "Groups" accordion tab; `SearchTree` in the "Nodes" accordion tab
- `src/components/SettingsPage.tsx` — `GraphSettings` in the "Graph" tab; `SearchSettings` in the "Search" tab
