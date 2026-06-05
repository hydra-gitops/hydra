# ClonePage

**File:** `src/components/ClonePage.tsx`

## Function

Displays a mini Cytoscape graph for a clone node and all its sibling clones. Shows the selected clone and its immediate neighbours (connected via edges) alongside all other sibling clones with their neighbours. Allows navigating to any node in the graph by clicking.

## Props

| Prop           | Type                       | Description                                          |
| -------------- | -------------------------- | ---------------------------------------------------- |
| `cloneNodeId`  | `string`                   | The ID of the clone node to display                  |
| `siblingIds`   | `Set<string>`              | IDs of all sibling clones (same original entity)     |
| `nodes`        | `GraphNode[]`              | All graph nodes (used to find neighbours)            |
| `edges`        | `GraphEdge[]`              | All graph edges (used to find connections)           |
| `isDark`       | `boolean`                  | Current dark mode state                              |
| `onNavigateTo` | `(nodeId: string) => void` | Called when the user clicks a node to navigate to it |
| `onClose`      | `() => void`               | Called when the clone page overlay is closed         |

## Used by

- `src/components/GraphPanel.tsx` — rendered as an overlay when a clone node's "Show clones" action is triggered
