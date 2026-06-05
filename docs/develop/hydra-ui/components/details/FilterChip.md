# FilterChip

**File:** `src/components/FilterChip.tsx`

## Function

A small coloured chip that represents a single filter condition (field + value). Supports multiple visual modes:

- **enabled** — Active include filter (coloured background)
- **disabled** — Active exclude filter (red/strikethrough)
- **inactive** — Greyed out, not applied
- **partial** — Partially active (some children active)

Clicking the chip cycles through modes; the X button removes it. Colour is determined by field type via `filterChipColors()`.

## Exports

- `FilterChip` — The chip component
- `displayFilterValue(field, value)` — Formats a filter value for display
- `filterChipColors(field, isDark)` — Returns colours for a given field type

## Props

| Prop          | Type             | Description                                                              |
| ------------- | ---------------- | ------------------------------------------------------------------------ |
| `field`       | `string`         | Filter field name                                                        |
| `fieldLabel`  | `string`         | Human-readable field label                                               |
| `value`       | `string`         | Filter value                                                             |
| `mode`        | `FilterChipMode` | Current chip mode (`"enabled"`, `"disabled"`, `"inactive"`, `"partial"`) |
| `isDark`      | `boolean`        | Current dark mode state                                                  |
| `onCycleMode` | `() => void`     | Called when the chip body is clicked                                     |
| `onRemove`    | `() => void`     | Called when the X button is clicked                                      |

## Used by

- `src/components/FilterExprTreeEditor.tsx` — renders chips for filter leaf nodes in the expression tree
- `src/components/FilterSlotsPanel.tsx` — renders chips in the filter slot view/edit panels
