# FilterSlotsPanel

**File:** `src/components/FilterSlotsPanel.tsx`

## Function

Manages the filter pipeline displayed in the sidebar "Filter" accordion. Contains:

- **Active filter slots** — Multiple independently configurable filter slots (OR-combined). Each slot can be toggled between view mode (compact chip display) and edit mode (full expression tree editor).
- **Selection slot** — A transient filter created by clicking on graph nodes, chart names, or sidebar items. Can be pinned to become a permanent active slot or cleared.
- **Chip states** — Global and per-slot enabled/disabled states for individual filter chips.

Supports saving the current filter as a named filter group, adding new empty slots, reordering slots, and duplicating/deleting existing slots.

## Props

| Prop                        | Type                                            | Description                                        |
| --------------------------- | ----------------------------------------------- | -------------------------------------------------- |
| `activeFilterSlots`         | `ActiveFilterSlot[]`                            | Current active filter slots                        |
| `onActiveFilterSlotsChange` | `(slots: ActiveFilterSlot[]) => void`           | Called when slots are added/removed/modified       |
| `filterGroups`              | `FilterGroupDefinition[]`                       | Named filter groups for reference resolution       |
| `entities`                  | `HydraEntity[]`                                 | All entities (for populating add-filter dropdowns) |
| `entityGroupMap`            | `Map<string, string>`                           | Entity group mapping                               |
| `nscsMap`                   | `Map<string, string>`                           | Namespace:clusterScope mapping                     |
| `isDark`                    | `boolean`                                       | Current dark mode state                            |
| `selectionSlot`             | `ActiveFilterSlot \| null`                      | Transient selection filter                         |
| `onPinSelection`            | `() => void`                                    | Called to pin the selection as a permanent slot    |
| `onClearSelection`          | `() => void`                                    | Called to clear the selection slot                 |
| `onSaveAsFilterGroup`       | `(name: string, root: FilterExprNode) => void`  | Called to save a slot as a named filter group      |
| `groupingKeys`              | `GroupingKeyDefinition[]`                       | Grouping key definitions for field options         |
| `groupingKeyMaps`           | `GroupingKeyMaps`                               | Resolved grouping key maps                         |
| `chipStates`                | `Record<string, boolean>`                       | Global chip enabled/disabled states                |
| `onChipStatesChange`        | `(chipStates: Record<string, boolean>) => void` | Called when chip states change                     |

## Used by

- `src/App.tsx` — rendered inside the "Filter" accordion tab in the sidebar
