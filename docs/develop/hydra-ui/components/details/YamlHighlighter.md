# YamlHighlighter

**File:** `src/components/YamlHighlighter.tsx`

## Function

A lightweight, read-only YAML viewer with CodeMirror 6 syntax highlighting. Used for displaying YAML content (like `Chart.yaml`) without editing capabilities. Supports dark mode via `oneDark` theme and configurable font size and max height.

The component efficiently manages the CodeMirror editor instance:

- Content changes are dispatched as transactions (no editor re-creation)
- Theme and font size changes trigger a full editor re-creation

## Props

| Prop        | Type      | Default     | Description                                                  |
| ----------- | --------- | ----------- | ------------------------------------------------------------ |
| `content`   | `string`  | —           | YAML content to display                                      |
| `isDark`    | `boolean` | —           | Current dark mode state (switches to `oneDark` theme)        |
| `maxHeight` | `number`  | `undefined` | Maximum height in pixels (enables vertical scrolling)        |
| `fontSize`  | `number`  | `undefined` | Font size in pixels (defaults to browser default if not set) |

## Used by

- `src/components/ChartPage.tsx` — displays `Chart.yaml` content with syntax highlighting in the details tab
