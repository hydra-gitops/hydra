# FilterExprTreeEditor

**File:** `src/components/FilterExprTreeEditor.tsx`

## Function

Visual drag-and-drop editor for building recursive filter expressions. Supports:

- **AND/OR groups** — Click the operator label to toggle between AND and OR
- **NOT negation** — Toggle negation on any group or leaf node
- **Filter leaves** — Individual `field = value` conditions rendered as `FilterChip`s
- **Reference leaves** — References to named filter groups
- **Drag and drop** — Drag chips onto each other to create nested groups; drag to reorder within groups
- **Add menu** — Dropdown to add new filter conditions by field/value

## Props

| Prop                   | Type                                | Description                                   |
| ---------------------- | ----------------------------------- | --------------------------------------------- |
| `root`                 | `FilterExprNode`                    | Root node of the expression tree              |
| `onRootChange`         | `(newRoot: FilterExprNode) => void` | Called when the tree is modified              |
| `entities`             | `HydraEntity[]`                     | Entities for populating field value dropdowns |
| `entityGroupMap`       | `Map<string, string>`               | Entity group mapping for field resolution     |
| `nscsMap`              | `Map<string, string>`               | Namespace:clusterScope mapping                |
| `fieldOptionGroups`    | `FieldOptionGroup[]`                | Custom field option groups for the add menu   |
| `extraFieldValues`     | `Record<string, Set<string>>`       | Additional values per field                   |
| `forceAddOpen`         | `boolean`                           | Force the add dropdown to be open initially   |
| `onSpecialFieldSelect` | `(value: string) => void`           | Callback for special field selection          |
| `specialFieldPrefix`   | `string`                            | Prefix for identifying special fields         |

## Used by

- `src/components/FilterSlotsPanel.tsx` — edit mode for individual filter slots
- `src/components/SettingsPage.tsx` — filter group editor in settings
