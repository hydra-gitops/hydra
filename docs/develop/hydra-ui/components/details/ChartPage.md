# ChartPage

**File:** `src/components/ChartPage.tsx`

## Function

Displays Helm chart metadata for the selected chart. The page has the following tabs:

- **Details** (default) — Version overview with appId tags, dependency tree (expanded by default), used-by/parent tree (expanded by default), and Chart.yaml with YAML syntax highlighting.
- **Values** — Unified values view (delegates to `ValuesView`).
- **Manifests** — Rendered Kubernetes manifests for the chart.
- **Templates** — Helm template sources from the chart archive.
- **Files** — Hierarchical tree view of all files and directories in the chart `.tgz` archive. See [Files Tab](#files-tab) below.
- **App** (optional) — Application-specific view, shown only when the chart has an associated app entity.

If the selected chart is not found in the entity-filtered results, it is added from the full chart list to ensure dependency charts are always visible when navigated to.

Dependencies and parent charts are clickable — clicking calls `onOpenChart` to navigate with a proper selection filter.

## Files Tab

The Files tab (`id: "files"`, icon: `pi pi-folder-open`) displays a read-only hierarchical tree of every file and directory contained in the chart's `.tgz` archive. The tree is rendered using PrimeReact's `Tree` component (same component used for Dependencies, Parents, and Templates trees).

### Behavior

- All directory nodes are expanded by default. `expandedKeys` is computed by collecting all directory node keys (`isFile === false`) in a `useMemo`, following the same pattern as `childrenExpandedKeys`, `parentExpandedKeys`, and `templateExpandedKeys`.
- Directories are sorted before files at each level; both are sorted alphabetically.
- Clicking a file node does nothing (no file viewer — tree display only).
- Data is loaded **lazily**: the file list is fetched only when the Files tab is first activated, not on page load.
- While loading, a centered "Loading..." text is shown (same pattern as other lazy-loaded content).
- If the tgz fetch fails, a centered message "Could not load chart files." is shown (graceful fallback, no crash).

### Data Flow

```text
Files tab activated
  │
  ▼
ChartPage calls ClusterLoader.listChartFiles(chartName)
  │
  ▼
ClusterLoader fetches chart .tgz (cached) → calls listAllFilesFromTgz(tgzData)
  │
  ▼
listAllFilesFromTgz iterates tar headers → returns string[] of regular file paths
  │
  ▼
ChartPage calls buildTgzFileTree(filePaths) → TgzFileTreeNode[]
  │
  ▼
useMemo converts TgzFileTreeNode[] → PrimeReact TreeNode[] (same pattern as dependency/parent/template trees)
  │
  ▼
Tree rendered via PrimeReact <Tree> component
```

### Types

```typescript
/** A node in the chart file tree (directory or file). */
export type TgzFileTreeNode = {
  name: string; // Display name (e.g. "values.yaml" or "templates")
  path: string; // Full path within the archive (e.g. "chart/templates")
  isFile: boolean; // true for files, false for directories (consistent with ValueFileTreeNode)
  children: TgzFileTreeNode[];
};
```text

### New Functions

| Function              | File                    | Signature                                  | Description                                                                                                                                                                                                                                    |
| --------------------- | ----------------------- | ------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `listAllFilesFromTgz` | `src/tgzExtract.ts`     | `(tgzData: ArrayBuffer): string[]`         | Iterates tar headers in the tgz archive and returns all regular file paths as a flat string array. Does not read file contents — only collects paths from headers.                                                                             |
| `listChartFiles`      | `src/clusterLoader.ts`  | `(chartName: string): Promise<string[]>`   | Public method on `ClusterLoader`. Fetches the chart `.tgz` (uses the existing cache) and delegates to `listAllFilesFromTgz`. Returns a flat array of file paths.                                                                               |
| `buildTgzFileTree`    | `src/chartPageLogic.ts` | `(filePaths: string[]): TgzFileTreeNode[]` | Converts a flat list of file paths into a hierarchical `TgzFileTreeNode[]` tree. Directories are sorted before files at each level, and both groups are sorted alphabetically. Follows the same pattern as the existing `buildValueFilesTree`. |

### Unit Tests

#### `listAllFilesFromTgz` tests (`src/__tests__/tgzExtract.test.ts`)

| Test                                  | Verifies                                                                                                 |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| Empty/minimal tgz returns empty array | An empty or minimal tgz with no file entries returns `[]`                                                |
| Returns all regular file paths        | A tgz with multiple files returns all regular file paths                                                 |
| Excludes directory entries            | Only regular files (type flag `0` / `0x30`) are returned; directory entries (type flag `5`) are excluded |

#### `buildTgzFileTree` tests (`src/__tests__/chartPageLogic.test.ts`)

| Test                            | Verifies                                                                                                                                              |
| ------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| Empty input returns empty array | `buildTgzFileTree([])` returns `[]`                                                                                                                   |
| Single file at root level       | A single path like `"Chart.yaml"` produces one file node with no children                                                                             |
| Nested directory structure      | Paths like `["chart/templates/deploy.yaml", "chart/values.yaml", "chart/Chart.yaml"]` produce the correct hierarchy with intermediate directory nodes |
| Directories sorted before files | At each level, directory nodes (`isFile === false`) appear before file nodes (`isFile === true`)                                                      |
| Alphabetical sorting            | Within directories and within files, nodes are sorted alphabetically by name                                                                          |
| Deep nesting (3+ levels)        | Paths with 3+ directory levels (e.g. `"chart/templates/hooks/pre-install.yaml"`) produce the correct deeply nested tree                               |

## Props

| Prop                             | Type                                 | Description                                                 |
| -------------------------------- | ------------------------------------ | ----------------------------------------------------------- |
| `filteredEntities`               | `HydraEntity[]`                      | Entities matching the current filter pipeline               |
| `charts`                         | `HydraChart[]`                       | All available chart entries                                 |
| `isDark`                         | `boolean`                            | Current dark mode state                                     |
| `selectedChartName`              | `string \| null`                     | Externally controlled selected chart name                   |
| `onSelectedChartNameChange`      | `(name: string \| null) => void`     | Called when the user selects a different chart              |
| `onOpenChart`                    | `(chartName: string) => void`        | Called when navigating to a dependency/parent chart         |
| `valueFiles`                     | `HydraValueFile[]`                   | Value file metadata for the values tab                      |
| `appValues`                      | `HydraAppValues[]`                   | App values entries                                          |
| `fallbackValues`                 | `HydraAppValues[]`                   | Fallback (hydra defaults) values                            |
| `loader`                         | `ClusterLoader`                      | Loader for on-demand data (Chart.yaml, merged values, etc.) |
| `activeTabId`                    | `string`                             | Active tab ID from URL hash                                 |
| `onTabChange`                    | `(tab: string \| undefined) => void` | Called when the active tab changes                          |
| `gitRemote`                      | `string`                             | Git remote URL for file links                               |
| `gitRepoPrefix`                  | `string`                             | Git repo prefix for value file paths                        |
| `gitBranch`                      | `string`                             | Git branch for file links                                   |
| `valuesDisabledSources`          | `string[]`                           | Disabled source types in values preview                     |
| `onValuesDisabledSourcesChange`  | `(sources: string[]) => void`        | Callback for disabled sources change                        |
| `valuesShowUnnecessary`          | `string[]`                           | Source types showing "unnecessary" markers                  |
| `onValuesShowUnnecessaryChange`  | `(sources: string[]) => void`        | Callback for unnecessary markers change                     |
| `valuesShowErrors`               | `string[]`                           | Source types showing "new key" error markers                |
| `onValuesShowErrorsChange`       | `(sources: string[]) => void`        | Callback for error markers change                           |
| `valuesHideGlobalClones`         | `boolean`                            | Whether to hide global clone lines                          |
| `onValuesHideGlobalClonesChange` | `(hide: boolean) => void`            | Callback for global clones toggle                           |
| `valuesDisabledFiles`            | `string[]`                           | Individually disabled value files                           |
| `onValuesDisabledFilesChange`    | `(files: string[]) => void`          | Callback for disabled files change                          |
| `valuesSelectedFile`             | `string`                             | Currently selected file in values tree                      |
| `onValuesSelectedFileChange`     | `(file: string) => void`             | Callback for selected file change                           |
| `onManifestSelect`               | `(path: string) => void`             | Called when a manifest is selected                          |
| `highlightedTemplatePath`        | `string \| null`                     | Path of the currently highlighted template                  |
| `baseFilteredEntities`           | `HydraEntity[]`                      | Base filtered entities before chart-specific filtering      |
| `clusterName`                    | `string`                             | Active cluster name                                         |
| `hydraData`                      | `HydraData`                          | Full hydra data model                                       |
| `editorFontSize`                 | `number`                             | Font size for the Chart.yaml viewer                         |

## Used by

- `src/App.tsx` — rendered when `page === "charts"`
