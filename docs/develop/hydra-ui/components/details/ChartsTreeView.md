# ChartsTreeView

**File:** `src/components/ChartsTreeView.tsx`

## Function

Sidebar tree view for navigating Helm charts. Supports two display modes:

- **Tree mode** — Charts grouped by root app ID (first two segments of appId), with transitive dependencies shown as nested nodes. Circular dependencies are detected and broken.
- **Flat mode** — All unique chart names listed alphabetically as root nodes.

Includes a debounced search field (500ms) to filter chart nodes by label. Navigable dependencies are shown as clickable links. Charts matching the current selection are highlighted with bold text.

## Props

| Prop                    | Type                             | Description                                            |
| ----------------------- | -------------------------------- | ------------------------------------------------------ |
| `charts`                | `HydraChart[]`                   | All chart entries to display                           |
| `onOpenChart`           | `(chartName: string) => void`    | Called when a chart or navigable dependency is clicked |
| `viewMode`              | `ChartsViewMode`                 | Current display mode (`"tree"` or `"flat"`)            |
| `onViewModeChange`      | `(mode: ChartsViewMode) => void` | Called when the user toggles the view mode             |
| `highlightedChartNames` | `Set<string>`                    | Chart names to highlight (from selection slot)         |

## Used by

- `src/App.tsx` — rendered inside the "Charts" accordion tab in the sidebar
