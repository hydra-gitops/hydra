# Hydra UI Component Documentation

Detailed documentation for each React component in the Hydra UI frontend.

## Components

| Component                                               | Source                                    | Description                                                                     |
| ------------------------------------------------------- | ----------------------------------------- | ------------------------------------------------------------------------------- |
| [App](details/App.md)                                   | `src/App.tsx`                             | Root application shell — orchestrates state, navigation, filtering, and layout  |
| [ChartPage](details/ChartPage.md)                       | `src/components/ChartPage.tsx`            | Chart details page with versions, dependencies, parents, Chart.yaml, and values |
| [ChartsTreeView](details/ChartsTreeView.md)             | `src/components/ChartsTreeView.tsx`       | Sidebar tree/flat view of charts with search                                    |
| [ClonePage](details/ClonePage.md)                       | `src/components/ClonePage.tsx`            | Mini-graph view for clone nodes and their siblings                              |
| [DebouncedSearch](details/DebouncedSearch.md)           | `src/components/DebouncedSearch.tsx`      | Search input with configurable debounce                                         |
| [EntityListPanel](details/EntityListPanel.md)           | `src/components/EntityListPanel.tsx`      | Sortable, paginated entity table                                                |
| [EntityPage](details/EntityPage.md)                     | `src/components/EntityPage.tsx`           | Entity detail overlay with multiple tabs                                        |
| [FilterChip](details/FilterChip.md)                     | `src/components/FilterChip.tsx`           | Tri-state filter chip                                                           |
| [FilterExprTreeEditor](details/FilterExprTreeEditor.md) | `src/components/FilterExprTreeEditor.tsx` | Drag-and-drop filter expression tree editor                                     |
| [FilterRow](details/FilterRow.md)                       | `src/components/FilterRow.tsx`            | Single filter row with field/value selection                                    |
| [FilterSlotsPanel](details/FilterSlotsPanel.md)         | `src/components/FilterSlotsPanel.tsx`     | Manages multiple filter slots with selection pinning                            |
| [FilesTreeView](details/FilesTreeView.md)               | `src/components/FilesTreeView.tsx`        | Sidebar file tree grouped by cluster/app/directory                              |
| [GraphPanel](details/GraphPanel.md)                     | `src/components/GraphPanel.tsx`           | Cytoscape graph with entity/group nodes, edges, and overlays                    |
| [IncomingRbacPanel](details/IncomingRbacPanel.md)       | `src/components/IncomingRbacPanel.tsx`    | Incoming RBAC permissions analysis                                              |
| [NumberInput](details/NumberInput.md)                   | `src/components/NumberInput.tsx`          | Numeric input with +/- buttons                                                  |
| [RbacInfoPanel](details/RbacInfoPanel.md)               | `src/components/RbacInfoPanel.tsx`        | Outgoing RBAC rules display with verb coverage                                  |
| [SearchableCodeViewer](details/SearchableCodeViewer.md) | `src/components/SearchableCodeViewer.tsx` | CodeMirror editor with search, breadcrumbs, and provenance                      |
| [SecretsPanel](details/SecretsPanel.md)                 | `src/components/SecretsPanel.tsx`         | Secret keys, producers, and consumers display                                   |
| [SettingsPage](details/SettingsPage.md)                 | `src/components/SettingsPage.tsx`         | Settings page with edit-copy pattern and multiple tabs                          |
| [SourceTag](details/SourceTag.md)                       | `src/components/SourceTag.tsx`            | Coloured tag for value file source types                                        |
| [StateEditor](details/StateEditor.md)                   | `src/components/StateEditor.tsx`          | Full YAML state editor with CodeMirror 6                                        |
| [TreePanel](details/TreePanel.md)                       | `src/components/TreePanel.tsx`            | Grouping tree, search tree, graph/search settings                               |
| [ValuesView](details/ValuesView.md)                     | `src/components/ValuesView.tsx`           | Split-panel values view with file tree and merged preview                       |
| [YamlHighlighter](details/YamlHighlighter.md)           | `src/components/YamlHighlighter.tsx`      | Read-only YAML viewer with syntax highlighting                                  |

Each component file contains props, behavior, layout, and source file references. For architecture-level documentation, see the parent [agent.md](../agent.md).
