# Hydra UI Architecture Documentation

Hydra UI is a React/TypeScript web frontend for visualizing GitOps deployments managed by Hydra Go. It renders Kubernetes objects as interactive graphs, provides filtering, RBAC analysis, Helm chart inspection, and fulltext search.

## Topics

| Topic                                 | Description                                                                |
| ------------------------------------- | -------------------------------------------------------------------------- |
| [State](state.md)                     | Application state management, persistence, localStorage, defaults cascade  |
| [Layout](layout.md)                   | Graph layout algorithm, data model, Cytoscape rendering, color/clone rules |
| [Navigation](navigation.md)           | Page-based routing, URL hash, tabs, settings edit-copy pattern             |
| [Sidebar](sidebar.md)                 | Accordion layout, filter/charts/files/nodes/groups sections, resize        |
| [Filters](filters.md)                 | Predefined filters, expression trees, active filter slots, chip states     |
| [Grouping Keys](grouping-keys.md)     | User-defined entity categories, resolution algorithm, integration          |
| [Chart Page](chart-page.md)           | Helm chart metadata, five tabs, values provenance, sub-chart export        |
| [Cluster Dump](cluster-dump.md)       | CLI export, ClusterLoader, manifest/template tabs, cross-cluster loading   |
| [RBAC Display](rbac-display.md)       | Entity page, outgoing/incoming RBAC analysis, secrets tab                  |
| [Click Flow](click-flow.md)           | User click paths, selection state, handler mapping                         |
| [Values Preview](values-preview.md)   | YAML viewer, breadcrumb, search, provenance annotations                    |
| [Fulltext Search](search-fulltext.md) | Global text search across manifests, templates, and values                 |
| [State Editor](state-editor.md)       | YAML state editor, live sync, dirty tracking, defaults viewer              |

## Components

Detailed documentation for all 24 React components: [components/agent.md](components/agent.md)

Each topic file contains a summary with key concepts and source file references. For full implementation details, see the `details/` subdirectory linked from each topic.
