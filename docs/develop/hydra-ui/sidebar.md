# Sidebar

The sidebar is the left panel of Hydra UI, providing filtering, file browsing, chart navigation, entity search, and graph configuration in a vertically stacked accordion layout. The sidebar width is resizable (170–600px, default 340px).

## Key Concepts

- **Header** — Logo, title, and icon row for page navigation (Search, Home, Charts, Graph, Theme toggle, Settings)
- **Accordion sections** — Filter, Charts, Files, Nodes, Groups — multiple can be expanded simultaneously
- **Filter sidebar toggle** — Controls whether sidebar trees show all entities or only those matching active filters
- **Filter section** — `FilterSlotsPanel` manages global filter slots and selection filter
- **Charts section** — `ChartsTreeView` with tree/flat view modes and search
- **Files section** — `FilesTreeView` grouped by cluster/app/directory with entity counts
- **Nodes section** — `SearchTree` with configurable grouping, search, and entity selection
- **Groups section** — `TreePanel` groups tree with expand/collapse checkboxes and filter-on-click
- **Graph panel toolbar** — Layout direction, expand/collapse controls, zoom, dependency highlighting

## Source Files

| File                                  | Purpose                                       |
| ------------------------------------- | --------------------------------------------- |
| `src/App.tsx`                         | Sidebar layout, header, accordion state       |
| `src/components/FilterSlotsPanel.tsx` | Filter slots panel                            |
| `src/components/ChartsTreeView.tsx`   | Charts tree view                              |
| `src/components/FilesTreeView.tsx`    | Files tree view                               |
| `src/components/TreePanel.tsx`        | Nodes tree, groups tree, search configuration |
| `src/components/GraphPanel.tsx`       | Graph panel with toolbar                      |
| `src/components/DebouncedSearch.tsx`  | Reusable debounced search input               |

→ **Full details:** [details/sidebar.md](details/sidebar.md)
