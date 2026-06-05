# FilesTreeView

**File:** `src/components/FilesTreeView.tsx`

## Function

Sidebar tree view that groups entities by their source file path. The tree hierarchy represents the directory structure of the cluster's manifests and templates. Clicking a file node sets a filter to show only entities from that file. Clicking an entity leaf node opens the entity details page.

Includes a debounced search field to filter nodes by path. Supports chart info nodes (childApp/rootApp) that create appId-based selection filters.

## Props

| Prop              | Type                                                    | Description                                               |
| ----------------- | ------------------------------------------------------- | --------------------------------------------------------- |
| `entities`        | `HydraEntity[]`                                         | Entities to display in the tree                           |
| `charts`          | `HydraChart[]`                                          | Chart metadata for chart info nodes                       |
| `onFilterByFiles` | `(filters: { field: string; value: string }[]) => void` | Called when a file/folder node is clicked to set a filter |
| `onSelectEntity`  | `(entityId: string) => void`                            | Called when an entity leaf node is clicked                |

## Used by

- `src/App.tsx` — rendered inside the "Files" accordion tab in the sidebar
