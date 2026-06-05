# Cluster Dump — UI Integration

## Overview

The Cluster Dump feature allows the CLI to export rendered manifests and Helm chart archives alongside the `hydra.yaml` dependency model. The UI uses these files to display two additional tabs on the Entity detail page: **Manifest** and **Template**.

## Data Flow

```text
CLI: hydra ui <output-dir>
  │
  │  Exports ALL clusters from the context into subdirectories:
  │
  ├─ <output-dir>/in-cluster/
  │    ├─ hydra.yaml          (dependency model: in-cluster apps + root app entities from all clusters)
  │    ├─ charts/<name>.tgz   (Helm chart archives)
  │    └─ manifests/...       (rendered YAML per entity, including ArgoCD Application CRs)
  │
  └─ <output-dir>/<cluster>/  (e.g. dev/)
       ├─ hydra.yaml          (dependency model: child app entities only)
       ├─ charts/<name>.tgz
       └─ manifests/...       (rendered YAML per entity, child app workloads only)
        │
        ▼
UI: serves <output-dir>/ as static files via Vite dev server (or production build)
  │
  └─ ClusterLoader (src/clusterLoader.ts)
       ├─ loadHydraYaml()    → /data/<cluster>/hydra.yaml (fallback URLs)
       ├─ loadManifest(path) → /data/<cluster>/manifests/<manifestPath>
       └─ loadTemplate(path) → /data/<cluster>/charts/<chartName>.tgz + extraction
```

**Export logic:** The CLI renders in-cluster first (all apps including root apps). Then for each other cluster, it renders all apps with in-cluster's CRD scope info, splits the result into root app entities (ArgoCD Application CRs) and child app entities (workloads), merges root app entities into the in-cluster result, and writes child app entities to the cluster's own directory. This ensures all Application CRs are consolidated in the in-cluster export.

All data loading goes through `ClusterLoader` (one instance per cluster, created in `App.tsx` via `useMemo`). The loader is passed as a prop: `App → GraphPanel → EntityPage`.

## Loading hydra.yaml

`ClusterLoader.loadHydraYaml()` tries three URL patterns in order:

1. `/data/<clusterName>/hydra.yaml` (directory-based format)
2. `/<clusterName>/hydra.yaml` (legacy directory-based)
3. `/<clusterName>.hydra.yaml` (flat file fallback)

All requests use `safeFetch()`, which rejects non-2xx responses and HTML content types (SPA fallback guard).

## Entity Detail Tabs

### Manifest Tab

Displays the rendered Kubernetes manifest for the current entity.

**Availability:** Only shown when `entity.manifestPath` is non-empty.

**Data Loading:**

1. Call `loader.loadManifest(entity.manifestPath)` (fetches `/data/<clusterName>/manifests/<manifestPath>`)
2. Display in a read-only CodeMirror editor with YAML syntax highlighting

**Error Handling:**

- 404: Show "Manifest not available" message
- Network error: Show error message with retry option

### Template Tab

Displays the Go template source from the Helm chart that generated the entity.

**Availability:** Only shown when `entity.templatePath` is non-empty.

**Data Loading:**

1. Call `loader.loadTemplate(entity.templatePath)` which internally:
   a. Extracts chart name from the path (first segment, e.g. `cert-manager` from `cert-manager/templates/deployment.yaml`)
   b. Fetches `/data/<clusterName>/charts/<chartName>.tgz` (cached per ClusterLoader instance)
   c. Decompresses with pako (gzip), parses the tar archive, and extracts the matching file
2. Display in a read-only CodeMirror editor with Go template syntax highlighting

**Caching:** Chart .tgz files are cached on the `ClusterLoader` instance (`Map<url, Promise<ArrayBuffer>>`). Failed fetches are evicted from the cache so retries work. The cache lives as long as the loader instance (i.e. until the cluster changes).

**Error Handling:**

- 404: Show "Chart archive not available" message
- Template not found in archive: Show "Template file not found in chart" message

## CodeMirror Viewer

Both tabs reuse the same read-only CodeMirror viewer component with:

- `@codemirror/lang-yaml` for YAML syntax highlighting (Manifest tab)
- Plain text mode for Go templates (Template tab)
- Dark/light theme support via `oneDark` theme
- Line numbers, code folding, search

## TGZ Extraction

The UI uses `pako` for gzip decompression and a minimal tar parser to extract files from Helm chart archives in the browser.

Tar parsing extracts:

- File name (from tar header, 512-byte blocks)
- File content (following data blocks)

Only the requested template file is extracted; other files are skipped.

## Tests

### ClusterLoader Tests (`src/__tests__/clusterLoader.test.ts`)

| Test                                   | Verifies                                                  |
| -------------------------------------- | --------------------------------------------------------- |
| `loadHydraYaml` first URL succeeds     | Loads from `/data/<cluster>/hydra.yaml` on first try      |
| `loadHydraYaml` fallback to second URL | Falls back on 404 to `/<cluster>/hydra.yaml`              |
| `loadHydraYaml` fallback to third URL  | Falls back to `/<cluster>.hydra.yaml` when first two fail |
| `loadHydraYaml` rejects HTML           | SPA fallback guard rejects `text/html` responses          |
| `loadHydraYaml` all URLs fail          | Throws when no URL pattern succeeds                       |
| `loadManifest` fetches by path         | Correct URL constructed with `/data/` prefix              |
| `loadManifest` rejects HTML            | Content-type guard rejects HTML responses                 |
| `loadManifest` rejects HTTP error      | Throws on non-2xx status                                  |
| `loadTemplate` fetch + cache           | Fetches .tgz, extracts template, second call hits cache   |
| `loadTemplate` not found in archive    | Throws when template file not found in .tgz               |
| `loadTemplate` cache eviction on error | Failed fetches are evicted, retries trigger new fetch     |
| `loadTemplate` rejects HTML            | Content-type guard applies to .tgz fetches                |

### Parse Tests (`src/__tests__/parseHydra.test.ts`)

| Test                                                | Verifies                                               |
| --------------------------------------------------- | ------------------------------------------------------ |
| `parseHydraYaml` with manifestPath                  | Entity with `manifestPath` in YAML is parsed correctly |
| `parseHydraYaml` manifestPath defaults when missing | Missing `manifestPath` defaults to `""`                |

### TGZ Extract Tests (`src/__tests__/tgzExtract.test.ts`)

| Test                                               | Verifies                                                           |
| -------------------------------------------------- | ------------------------------------------------------------------ |
| `extractFileFromTar` extracts file by path         | Correct file content is returned for a given path                  |
| `extractFileFromTar` returns null for missing file | Returns null when the requested file does not exist in the archive |
| `extractChartName` from templatePath               | First segment of templatePath is returned as chart name            |
| `extractChartName` handles sub-chart paths         | Correct chart name from `parent/charts/sub/templates/file.yaml`    |

## Cross-Cluster Loading

Some data only exists in a specific cluster's export and is not available in the currently viewed cluster. The primary example is **ArgoCD Application CRs**, which are consolidated in the "in-cluster" (management cluster) export. Root apps from all clusters are rendered and their Application CR entities are merged into the in-cluster export during the `hydra ui` context export.

### Concept

When the user views a cluster other than "in-cluster", certain features need access to entities from the in-cluster export. The UI handles this on the client side by creating additional `ClusterLoader` instances for other clusters.

### Solution

The UI can create a second `ClusterLoader("in-cluster")` to load the in-cluster `hydra.yaml` alongside the current cluster's data. This loader:

- Calls `loadHydraYaml()` with the `"in-cluster"` cluster name, using the same URL fallback logic as the primary loader
- Parses the returned `hydra.yaml` to obtain the in-cluster entity map
- Is **cached** — created once and reused across tab switches, chart changes, and other navigation within the same session. The cached loader persists as long as the user remains in the application.
- Is only created when needed (lazy initialization) — viewing in-cluster itself does not trigger a second loader since the entities are already present
- **Error handling:** If the in-cluster `hydra.yaml` cannot be loaded (network error, 404, or in-cluster export not available), the dependent feature degrades gracefully — e.g. the App tab is hidden rather than showing an error

### Used By

- **Chart Page → App Tab:** Uses the in-cluster `ClusterLoader` to find ArgoCD Application entities when the user is viewing a non-in-cluster cluster. See `CHART_PAGE.md` → "App Tab" for details.
