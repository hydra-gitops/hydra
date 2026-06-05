# Graph Layout

Hydra UI renders Kubernetes objects as a directed graph, grouped by namespace and entity group. The layout algorithm computes positions for entities and groups in a hierarchical tree structure, with dependencies determining the reading direction.

## Key Concepts

- **Single source of truth** — All positions, sizes, and group membership live in the React model; Cytoscape is a pure rendering target
- **Processing order** — Filters applied first, then entities grouped into tree hierarchy using configured grouping fields
- **Recursive layout** — Hierarchical Sugiyama-style layout computed recursively for each group level
- **Node types** — Entity nodes (leaf), group nodes (compound), collapsed group nodes (represent hidden subtree)
- **Color rules** — Field/value-based color rules for groups and nodes with auto-color, manual color, and unchanged modes
- **Clone rules** — Entity duplication to reduce edge crossings for high-fanout nodes (manual + threshold-based auto-cloning)
- **Graph model** — `computeGraphNodes`, `computeGraphEdges` transform tree + layout into Cytoscape elements

## Source Files

| File                            | Description                                                                                 |
| ------------------------------- | ------------------------------------------------------------------------------------------- |
| `src/model.ts`                  | Core types: `GroupingField`, `GraphColorRule`, `CloneRule`, `HydraEntity`, `HydraReference` |
| `src/layoutLogic.ts`            | `computeNodeLayout`, `computeRecursiveLayout`, `computeEntityNodeSize`                      |
| `src/graphModel.ts`             | `computeGraphNodes`, `computeGraphEdges`, `computeVisibleEntities`                          |
| `src/treeLogic.ts`              | `buildTree`, `getAllGroups`, `getCollapsedGroups`, grouping field helpers                   |
| `src/colorLogic.ts`             | `autoColor`, `resolveColorFromRules`                                                        |
| `src/cloneLogic.ts`             | `cloneEntities`, `buildAutoCloneRules`, `getEffectiveCloneRules`                            |
| `src/filterLogic.ts`            | `applyFilters`, `expandWithRefs`                                                            |
| `src/components/GraphPanel.tsx` | Cytoscape rendering, interaction, context menu                                              |

→ **Full details:** [details/layout.md](details/layout.md)
