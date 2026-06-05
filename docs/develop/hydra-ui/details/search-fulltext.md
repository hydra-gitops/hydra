# Fulltext Search Architecture

## Overview

The fulltext search page provides a global, client-side text search across five resource types in the current cluster:

- **Manifests** — rendered Kubernetes manifests (plain YAML files served from `/data/<cluster>/manifests/`)
- **Templates** — Helm chart template files extracted from `.tgz` chart archives
- **Local Values** — values files served from `/data/<cluster>/values/files/`
- **Merged Values** — merged values per app served from `/data/<cluster>/values/merged/<appId>.yaml`
- **Helm Chart Values** — `values.yaml` files extracted from chart `.tgz` archives

All five sources are enabled by default. Users can toggle each source independently via checkboxes. The search is case-insensitive substring matching. Results are grouped by file and show matching lines with 2 lines of context above and below.

## Data Flow

```text
┌──────────────────────────────────────────────────────────────┐
│  SearchPage                                                  │
│                                                              │
│  ┌──────────────┐  ┌──────────────────────────────────────┐  │
│  │ Search Input │  │ Source Filters                                          │  │
│  │ (debounced)  │  │ [x] Manifests [x] Templates [x] Local Values           │  │
│  │              │  │ [x] Merged Values [x] Helm Chart Values                 │  │
│  └──────┬───────┘  └──────────────┬───────────────────────┘  │
│         │                         │                          │
│         └─────────┬───────────────┘                          │
│                   ▼                                          │
│         loadSearchableContent()                              │
│         (loads files from enabled sources)                   │
│                   │                                          │
│                   ▼                                          │
│         searchInContent()                                    │
│         (pure function, case-insensitive matching)           │
│                   │                                          │
│                   ▼                                          │
│         SearchResult                                         │
│         (matches grouped by file, with line context)         │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

### Content Loading

| Source            | Loader Method                            | Description                                                                                      |
| ----------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Manifests         | `ClusterLoader.loadAllManifests()`       | Fetches all manifest files for entities that have a non-empty `manifestPath`                     |
| Templates         | `ClusterLoader.loadAllChartFiles()`      | Fetches all `.tgz` chart archives and extracts all text files via `extractAllTextFilesFromTgz()` |
| Local Values      | `ClusterLoader.loadAllLocalValueFiles()` | Fetches all value files listed in `HydraData.valueFiles`                                         |
| Merged Values     | `ClusterLoader.loadAllMergedValues()`    | Fetches merged values for all apps listed in `HydraData.appValues`                               |
| Helm Chart Values | `ClusterLoader.loadAllHelmChartValues()` | Extracts all `values.yaml` files from chart `.tgz` archives                                      |

Content is loaded on-demand when the search page opens and cached for the session. Re-loading happens when sources are toggled on.

## Types

### State (`state.ts`)

```typescript
export type SearchSourceType =
  | "manifests"
  | "templates"
  | "values"
  | "mergedValues"
  | "helmChartValues";

export type FulltextSearchState = {
  query: string;
  sources: SearchSourceType[];
};
```

`FulltextSearchState` serves a dual purpose:

1. **Last search (model)**: Persisted in the unified YAML state (`hydra-state` localStorage key) as the last-used search. When the user clicks the search icon, this is used to populate the URL and restore the previous search.
2. **Current search (URL)**: The active search parameters are stored in the URL hash as `q=<query>&sources=<csv>`. The `SearchPage` reads its initial state from the URL and updates it via `replaceState` (no history entries for parameter changes).

**Defaults:**

- `query`: `""`
- `sources`: `["manifests", "templates", "values", "mergedValues", "helmChartValues"]`

### Search Logic (`fullSearchLogic.ts`)

```typescript
export type SearchResultMatch = {
  filePath: string;
  sourceType: SearchSourceType;
  lineNumber: number;
  lineContent: string;
  contextBefore: string[];
  contextAfter: string[];
};

export type SearchFileGroup = {
  filePath: string;
  sourceType: SearchSourceType;
  matches: SearchResultMatch[];
};

export type SearchResult = {
  query: string;
  fileGroups: SearchFileGroup[];
  totalMatches: number;
  totalFiles: number;
  searchedFiles: number;
  durationMs: number;
};
```

### `searchInContent(query, files)` Algorithm

1. If `query` is empty, return empty result immediately.
2. Convert `query` to lowercase.
3. For each file in the input map:
   a. Split content into lines.
   b. For each line, check if `line.toLowerCase()` contains the query.
   c. For each match, capture line number, content, and 2 lines of context above/below.
   d. Merge overlapping context windows.
4. Group matches by file path.
5. Return `SearchResult` with timing information.

## Page Type

`PageType` is extended with `"search"`:

```typescript
export type PageType =
  | "list"
  | "graph"
  | "charts"
  | "details"
  | "settings"
  | "search";
```

URL: `#<cluster>?page=search&q=<query>&sources=<csv>`

- `q` — search query (omitted when empty)
- `sources` — comma-separated source types (omitted when all sources are selected)

## Navigation

The search icon (`pi-search`) is placed as the first icon in the sidebar header icon row (before the Home icon). It navigates to the search page. The icon is highlighted when the search page is active.

The Escape key closes the search page via `goBack()`.

```text
┌──────────────────────────────────────────────────┐
│  🐉 Hydra  cluster  [🔍] [🏠] [📊] [◈]  🌙  ⚙ │
│                       ▲    ▲    ▲    ▲    ▲   ▲  │
│                       │    │    │    │    │   └ Settings
│                       │    │    │    │    └ Theme toggle
│                       │    │    │    └ Graph page
│                       │    │    └ Charts page
│                       │    └ List page (Home)
│                       └ Search page (NEW)
└──────────────────────────────────────────────────┘
```

## Component: `SearchPage.tsx`

### Props

```typescript
type SearchPageProps = {
  hydraData: HydraData;
  loader: ClusterLoader;
  isDark: boolean;
  onClose: () => void;
  initialQuery: string;
  initialSources: SearchSourceType[];
  onSearchChange: (query: string, sources: SearchSourceType[]) => void;
};
```

- `initialQuery` / `initialSources` — read from URL hash params (`q`, `sources`), falling back to the last search from the model.
- `onSearchChange` — called on debounced changes; updates both the URL (`replaceState`) and the model (last search bookmark).

### Layout

```text
┌──────────────────────────────────────────────────┐
│  Fulltext Search                            [✕]  │
├──────────────────────────────────────────────────┤
│  [____Search query____]                          │
│  Sources: [x] Manifests [x] Templates            │
│  [x] Local Values [x] Merged Values              │
│  [x] Helm Chart Values                           │
├──────────────────────────────────────────────────┤
│  42 matches in 15 / 350 files (1.2s)             │
├──────────────────────────────────────────────────┤
│  ▸ manifests/default/nginx-deployment.yaml  (3)  │
│  ▾ templates/my-chart/deployment.yaml       (2)  │
│    11│ ...                                       │
│    12│ replicas: {{ .Values.replicaCount }}  ◀── │
│    13│ ...                                       │
│  ▸ values/files/prod/values.yaml            (1)  │
└──────────────────────────────────────────────────┘
```

- **Search input**: PrimeReact `InputText`, debounced (300ms), autofocused.
- **Source filters**: PrimeReact `Checkbox` for each source type, inline layout.
- **Status bar**: Shows match count, file count, and search duration.
- **Results**: Collapsible file groups. Each group shows the file path, source badge, and match count. Expanding shows matched lines with context and highlighted search terms.
- **Close button**: Top-right `×` button, calls `onClose()` → `goBack()`.

## `tgzExtract.ts` Changes

New function:

```typescript
export function extractAllTextFilesFromTgz(
  tgzData: ArrayBuffer,
): Map<string, string>;
```

Decompresses the `.tgz`, iterates all tar entries, and returns a `Map<filePath, content>` for all regular files (type flag `0` or `0x30`).

## `clusterLoader.ts` Changes

New methods on `ClusterLoader`:

```typescript
async loadAllChartFiles(chartNames: string[]): Promise<Map<string, string>>
async loadAllManifests(entities: Map<string, HydraEntity>): Promise<Map<string, string>>
async loadAllLocalValueFiles(valueFiles: HydraValueFile[]): Promise<Map<string, string>>
async loadAllMergedValues(appValues: HydraAppValues[]): Promise<Map<string, string>>
async loadAllHelmChartValues(chartNames: string[]): Promise<Map<string, string>>
```

All methods return `Map<displayPath, content>`. Failed fetches for individual files are silently skipped (the search should be resilient to missing files).

## Source Files

| File                            | Purpose                                                               |
| ------------------------------- | --------------------------------------------------------------------- |
| `search-fulltext.md`            | This document                                                         |
| `src/state.ts`                  | `SearchSourceType`, `FulltextSearchState` types, `PageType` extension |
| `src/useHashNavigation.ts`      | `"search"` added to valid page types                                  |
| `src/tgzExtract.ts`             | `extractAllTextFilesFromTgz()` function                               |
| `src/clusterLoader.ts`          | Bulk-load methods for search                                          |
| `src/fullSearchLogic.ts`        | `searchInContent()` pure function                                     |
| `src/components/SearchPage.tsx` | Search page React component                                           |
| `src/App.tsx`                   | Search icon, page rendering, state wiring                             |

## Tests

### `src/__tests__/fullSearchLogic.test.ts`

| Test                              | Verifies                                                 |
| --------------------------------- | -------------------------------------------------------- |
| Empty query returns empty result  | No matches when query is empty string                    |
| Finds single match                | Single occurrence in one file                            |
| Case-insensitive matching         | Uppercase query matches lowercase content and vice versa |
| Context lines                     | 2 lines before and after each match are captured         |
| Context merging                   | Adjacent matches share context without duplication       |
| Multiple files                    | Matches across multiple files are grouped correctly      |
| Multiple matches in one file      | All matches in a single file are returned                |
| Source type preserved             | Each match carries its source type                       |
| No partial line matches           | Matching is per-line (full substring)                    |
| Special regex characters in query | Dots, brackets etc. in query are treated as literals     |

### `src/__tests__/tgzExtract.test.ts` (extended)

| Test                                           | Verifies                                                 |
| ---------------------------------------------- | -------------------------------------------------------- |
| `extractAllTextFilesFromTgz` returns all files | All regular files extracted from a `.tgz` archive        |
| Empty archive returns empty map                | No files for an archive with only end-of-archive markers |

### `src/__tests__/useHashNavigation.test.ts` (extended)

| Test                              | Verifies                                        |
| --------------------------------- | ----------------------------------------------- |
| `parseHash` accepts `page=search` | Search page type is valid                       |
| `buildHash` generates search URL  | Correct hash for search page                    |
| Roundtrip for search page         | `buildHash` → `parseHash` preserves search page |
