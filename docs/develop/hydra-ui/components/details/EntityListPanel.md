# EntityListPanel

**File:** `src/components/EntityListPanel.tsx`

## Function

Paginated, sortable data table for entities. Supports multi-column sorting, column visibility toggling (including grouping key columns), global search filtering, and row click navigation to entity details. Displays static fields (id, namespace, kind, name, etc.) and dynamic grouping key columns.

## Props

| Prop                     | Type                                    | Description                                       |
| ------------------------ | --------------------------------------- | ------------------------------------------------- |
| `entities`               | `HydraEntity[]`                         | Entities to display (already filtered)            |
| `totalEntityCount`       | `number`                                | Total entity count (for "showing X of Y" display) |
| `isDark`                 | `boolean`                               | Current dark mode state                           |
| `onEntitySelect`         | `(entityId: string) => void`            | Called when a row is clicked                      |
| `entityGroupMap`         | `Map<string, string>`                   | Entity ID → entity group name mapping             |
| `nscsMap`                | `Map<string, string>`                   | Entity ID → namespace:clusterScope mapping        |
| `groupingKeyMaps`        | `GroupingKeyMaps`                       | Resolved grouping key maps for dynamic columns    |
| `groupingKeys`           | `GroupingKeyDefinition[]`               | Grouping key definitions (for column headers)     |
| `visibleColumns`         | `string[]`                              | List of currently visible column IDs              |
| `onVisibleColumnsChange` | `(columns: string[]) => void`           | Called when column visibility changes             |
| `sortMeta`               | `EntityListSortEntry[]`                 | Current sort configuration                        |
| `onSortChange`           | `(sort: EntityListSortEntry[]) => void` | Called when sort configuration changes            |

## Used by

- `src/App.tsx` — rendered when `page === "list"`
