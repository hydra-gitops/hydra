# GraphPanel

**File:** `src/components/GraphPanel.tsx`

## Function

The main graph visualisation panel using Cytoscape.js. Renders entity nodes, group boundary boxes, and edges (references/dependencies). Supports:

- **Node rendering** — Entity nodes with header/description, group nodes with labels
- **Selection** — Click to select, Ctrl+click to add to selection, click background to deselect
- **Drag** — Drag individual nodes or entire groups
- **Zoom/Pan** — Mouse wheel zoom, drag to pan, zoom-to-fit buttons
- **Dependencies** — Toggle dependency edge visibility
- **Context menu** — Right-click on nodes for actions (jump to graph, show info, show clones)
- **Overlays** — Renders `EntityPage` (for entity details) and `ClonePage` (for clone views) as overlays on top of the graph

The panel also manages the toolbar with zoom controls, dependency toggle, and navigation breadcrumbs.

## Props

| Prop                   | Type                                                      | Description                                 |
| ---------------------- | --------------------------------------------------------- | ------------------------------------------- |
| `nodes`                | `GraphNode[]`                                             | Graph nodes to render                       |
| `edges`                | `GraphEdge[]`                                             | Graph edges to render                       |
| `selectionMatchIds`    | `Set<string>`                                             | IDs of nodes matching the current selection |
| `reachability`         | `ReachabilityMap`                                         | Reachability data for RBAC analysis         |
| `hydraEntities`        | `Map<string, HydraEntity>`                                | All entity data                             |
| `hydraReferences`      | `HydraReference[]`                                        | All references                              |
| `entityToNodeId`       | `Map<string, string>`                                     | Entity ID → graph node ID mapping           |
| `cloneIds`             | `Set<string>`                                             | IDs of clone entities                       |
| `cloneSiblingMap`      | `Map<string, Set<string>>`                                | Clone ID → sibling clone IDs                |
| `onNodeSelect`         | `(nodeId: string, addToSelection: boolean) => void`       | Called on node click                        |
| `onNodeMove`           | `(nodeId: string, x: number, y: number) => void`          | Called on node drag end                     |
| `zoomCommand`          | `ZoomCommand \| null`                                     | Programmatic zoom command                   |
| `onSyncComplete`       | `(cy: cytoscape.Core) => void`                            | Called after Cytoscape sync                 |
| `onZoomToAll`          | `() => void`                                              | Zoom to fit all nodes                       |
| `onZoomToSelection`    | `() => void`                                              | Zoom to fit selection                       |
| `onZoomIn`             | `() => void`                                              | Zoom in                                     |
| `onZoomOut`            | `() => void`                                              | Zoom out                                    |
| `showDependencies`     | `boolean`                                                 | Whether dependency edges are visible        |
| `onToggleDependencies` | `() => void`                                              | Toggle dependency edge visibility           |
| `page`                 | `PageType`                                                | Current page type (for overlay routing)     |
| `nodeId`               | `string`                                                  | Selected entity ID (for details overlay)    |
| `tab`                  | `string`                                                  | Active tab in entity details                |
| `onSetPage`            | `(page: PageType, nodeId?: string, tab?: string) => void` | Navigate to a page                          |
| `onGoBack`             | `() => void`                                              | Navigate back                               |
| `groupingKeyMaps`      | `GroupingKeyMaps`                                         | Resolved grouping key maps                  |
| `groupingKeys`         | `GroupingKeyDefinition[]`                                 | Grouping key definitions                    |
| `onBreadcrumbFilter`   | `(filters: BreadcrumbFilter[]) => void`                   | Called on breadcrumb click                  |
| `loader`               | `ClusterLoader`                                           | Loader for on-demand data                   |
| `valuesConfig`         | `ValuesViewConfig`                                        | Config for values views in entity details   |

## Exported Types

- `GraphNode` — Node data with position, display config, and metadata
- `GraphEdge` — Edge data with source/target IDs
- `ZoomCommand` — Zoom command descriptor (`"fit-all"`, `"fit-selection"`, `"zoom-in"`, `"zoom-out"`)

## Exported Functions

- `fitToSelection(cy, padding)` — Programmatic zoom-to-fit on a Cytoscape instance

## Used by

- `src/App.tsx` — rendered when `page === "graph"` or `page === "details"`
