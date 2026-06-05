# DebouncedSearch

**File:** `src/components/DebouncedSearch.tsx`

## Function

A text input field with debounced output. The `onFilterChange` callback fires after the configured debounce delay, or immediately when the user presses Enter. This prevents excessive re-renders during fast typing.

## Props

| Prop             | Type                      | Default     | Description                                                    |
| ---------------- | ------------------------- | ----------- | -------------------------------------------------------------- |
| `onFilterChange` | `(value: string) => void` | —           | Called with the current input value after debounce or on Enter |
| `placeholder`    | `string`                  | `undefined` | Placeholder text for the input                                 |
| `debounceMs`     | `number`                  | `undefined` | Debounce delay in milliseconds                                 |
| `style`          | `React.CSSProperties`     | `undefined` | Custom styles for the input wrapper                            |

## Used by

- `src/components/ChartsTreeView.tsx` — filter charts by name (500ms debounce)
- `src/components/EntityListPanel.tsx` — global search in entity table
- `src/components/FilesTreeView.tsx` — filter files by path
- `src/components/TreePanel.tsx` — filter search tree nodes
