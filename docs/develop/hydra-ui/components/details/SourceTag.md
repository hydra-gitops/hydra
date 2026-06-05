# SourceTag

**File:** `src/components/SourceTag.tsx`

## Function

A small inline tag for displaying value file source type labels. Renders a coloured text with a tinted background and border, matching the provenance colour scheme used throughout the values view.

## Props

| Prop    | Type            | Description                                      |
| ------- | --------------- | ------------------------------------------------ |
| `label` | `string`        | Text to display in the tag                       |
| `color` | `string`        | CSS colour for text, background tint, and border |
| `style` | `CSSProperties` | Optional additional styles                       |

## Used by

- `src/components/ValuesView.tsx` — displays source type tags (group, context, cluster, app, chart values, hydra defaults) next to file nodes in the values tree
