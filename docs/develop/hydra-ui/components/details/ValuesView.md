# ValuesView

**File:** `src/components/ValuesView.tsx`

## Function

Split-panel view for Helm chart values. Combines a file tree browser (left) with a code viewer (right) and supports two modes:

### Tree Mode (`valuesViewMode: "tree"`)

- **Left pane (55%)** — PrimeReact `TreeTable` showing the GitOps repository directory structure, chart archive values, and Hydra fallback values. Each file has coloured tags by source type and three checkbox columns (Visible, Errors, Unneeded).
- **Right pane (45%)** — Displays raw YAML of the clicked file with provenance gutter markers.

### Preview Mode (`valuesViewMode: "preview"`)

- Shows fully merged values with per-line provenance annotations (coloured gutter, tooltips, override chain)
- Supports filtering by source type, individual files, unnecessary override display, and global clone detection/hiding

The component handles loading chart archives, building provenance maps, alias resolution for Helm sub-chart dependencies, and the complete filtering/hiding pipeline.

## Exported Types

- `ValuesViewConfig` — Configuration subset for values state (sources, files, toggles)
- `ValuesViewProps` — Full props type

## Props

| Prop                             | Type                          | Description                                 |
| -------------------------------- | ----------------------------- | ------------------------------------------- |
| `isDark`                         | `boolean`                     | Current dark mode state                     |
| `loader`                         | `ClusterLoader`               | Loader for chart archives and merged values |
| `chartName`                      | `string`                      | Selected chart name                         |
| `appIds`                         | `string[]`                    | App IDs for the selected chart              |
| `charts`                         | `HydraChart[]`                | All chart entries                           |
| `valueFiles`                     | `HydraValueFile[]`            | Value file metadata                         |
| `fallbackValues`                 | `HydraAppValues[]`            | Hydra defaults/fallback values              |
| `gitRemote`                      | `string`                      | Git remote URL for file links               |
| `gitRepoPrefix`                  | `string`                      | Git repo prefix                             |
| `gitBranch`                      | `string`                      | Git branch                                  |
| `valuesDisabledSources`          | `string[]`                    | Disabled source types                       |
| `onValuesDisabledSourcesChange`  | `(sources: string[]) => void` | Callback for disabled sources               |
| `valuesShowUnnecessary`          | `string[]`                    | Source types showing "unnecessary" markers  |
| `onValuesShowUnnecessaryChange`  | `(sources: string[]) => void` | Callback for unnecessary toggle             |
| `valuesShowErrors`               | `string[]`                    | Source types showing "new key" markers      |
| `onValuesShowErrorsChange`       | `(sources: string[]) => void` | Callback for errors toggle                  |
| `valuesHideGlobalClones`         | `boolean`                     | Whether to hide global clone lines          |
| `onValuesHideGlobalClonesChange` | `(hide: boolean) => void`     | Callback for global clones toggle           |
| `valuesDisabledFiles`            | `string[]`                    | Individually disabled files                 |
| `onValuesDisabledFilesChange`    | `(files: string[]) => void`   | Callback for disabled files                 |
| `valuesSelectedFile`             | `string`                      | Currently selected file path                |
| `onValuesSelectedFileChange`     | `(file: string) => void`      | Callback for file selection                 |

## Used by

- `src/components/ChartPage.tsx` — Values tab in the chart page
- `src/components/EntityPage.tsx` — Values tab in the entity details overlay
