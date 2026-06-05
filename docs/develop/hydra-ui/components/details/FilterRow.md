# FilterRow

**File:** `src/components/FilterRow.tsx`

## Function

A single row in a tabular filter interface. Provides a field selector dropdown, a multi-select value picker, a toggle for including references, and a delete button. Originally used in an older filter interface before the expression tree editor was introduced.

## Props

| Prop                  | Type                                      | Description                                 |
| --------------------- | ----------------------------------------- | ------------------------------------------- |
| `rowId`               | `string`                                  | Unique row identifier                       |
| `field`               | `FilterField`                             | Currently selected filter field             |
| `values`              | `string[]`                                | Selected filter values                      |
| `includeRefs`         | `boolean`                                 | Whether to include reference matches        |
| `fieldOptions`        | `{ value: FilterField; label: string }[]` | Available fields for the dropdown           |
| `optionsByField`      | `Record<FilterField, Option[]>`           | Available values per field                  |
| `onFieldChange`       | `(value: FilterField) => void`            | Called when the field selection changes     |
| `onValuesChange`      | `(values: string[]) => void`              | Called when values change                   |
| `onIncludeRefsChange` | `(includeRefs: boolean) => void`          | Called when the include-refs toggle changes |
| `onRemove`            | `() => void`                              | Called when the delete button is clicked    |

## Used by

Not currently imported by any other component. This component predates the `FilterExprTreeEditor` and may be used in future interfaces or removed.
