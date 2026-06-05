# NumberInput

**File:** `src/components/NumberInput.tsx`

## Function

A simple numeric input with increment/decrement buttons. Wraps PrimeReact's `InputNumber` with horizontally arranged +/- buttons for easy value adjustment.

## Props

| Prop       | Type                      | Default     | Description                   |
| ---------- | ------------------------- | ----------- | ----------------------------- |
| `value`    | `number`                  | —           | Current numeric value         |
| `onChange` | `(value: number) => void` | —           | Called when the value changes |
| `min`      | `number`                  | `undefined` | Minimum allowed value         |
| `max`      | `number`                  | `undefined` | Maximum allowed value         |
| `step`     | `number`                  | `undefined` | Increment/decrement step size |

## Used by

- `src/components/TreePanel.tsx` — used in `GraphSettings` for configuring auto-clone thresholds
