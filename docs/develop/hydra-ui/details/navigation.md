# Navigation Architecture

## Overview

Hydra UI uses a **page-based model** with **sub-tabs** within pages. The navigation state is encoded in the URL hash, enabling shareable links and browser history integration.

```text
┌─────────────────────────────────────────────┐
│  Page (URL-encoded, `page=` param)          │
│                                             │
│  "list"     ← Default (omitted from URL)   │
│  "graph"    ← Dependency graph              │
│  "charts"   ← Helm charts view             │
│  "details"  ← Entity detail page           │
│  "settings" ← Settings page                │
│  "search"   ← Fulltext search page         │
│                                             │
│  Tab (URL-encoded, `tab=` param)            │
│  → Sub-tab within the current page          │
└─────────────────────────────────────────────┘
```

## Pages

Six top-level pages are available. The `page` URL parameter determines which page is shown.

| Page               | URL                               | Description                                                                           |
| ------------------ | --------------------------------- | ------------------------------------------------------------------------------------- |
| **List** (default) | `#cluster`                        | Entity table with filters, column sorting, and search. Rendered by `EntityListPanel`. |
| **Graph**          | `#cluster?page=graph`             | Interactive dependency graph with Cytoscape.js. Rendered by `GraphPanel`.             |
| **Charts**         | `#cluster?page=charts`            | Helm charts view. Rendered by `ChartPage`.                                            |
| **Details**        | `#cluster?page=details&node=<id>` | Entity detail page with tabs. Rendered by `EntityPage` inside `GraphPanel`.           |
| **Settings**       | `#cluster?page=settings`          | Settings page with tabs. Rendered by `SettingsPage`.                                  |
| **Search**         | `#cluster?page=search`            | Fulltext search across manifests, templates, and values. Rendered by `SearchPage`.    |

The list page is the "home" screen. There is no close button on it — it is always accessible via the Home icon in the sidebar header.

## Tabs

Tabs are sub-views within a page. The `tab` URL parameter determines which tab is shown. When no `tab` is specified, the default tab for that page is used.

### Details Page Tabs

Entity pages share a unified `EntityPage` component with tab switching.

| Tab                   | URL `tab=` value | Description                                                  |
| --------------------- | ---------------- | ------------------------------------------------------------ |
| **Details** (default) | `details`        | Entity details (metadata, tags, secretKeys)                  |
| **Relations**         | `relations`      | Entity relations table (outgoing/incoming refs)              |
| **Outgoing RBAC**     | `outgoing-rbac`  | What RBAC permissions this entity has                        |
| **Incoming RBAC**     | `incoming-rbac`  | Who can access this resource type                            |
| **Secrets**           | `secrets`        | Secret keys, producers, consumers (Secret entities only)     |
| **Clone**             | `clone`          | Clone info for replicated entities                           |
| **Manifest**          | `manifest`       | Entity manifest (internal, not a separate navigation target) |
| **Template**          | `template`       | Entity template (internal, not a separate navigation target) |

### Charts Page Tabs

The charts page (`ChartPage.tsx`) has sub-tabs for different views of the selected chart.

| Tab                   | URL `tab=` value | Icon                | Description                                                             |
| --------------------- | ---------------- | ------------------- | ----------------------------------------------------------------------- |
| **Details** (default) | `details`        | `pi pi-info-circle` | Version overview, dependency/parent trees, Chart.yaml                   |
| **Values**            | `values`         | `pi pi-database`    | Unified values view                                                     |
| **Manifests**         | `manifests`      | `pi pi-file`        | Rendered Kubernetes manifests                                           |
| **Templates**         | `templates`      | `pi pi-copy`        | Helm template sources                                                   |
| **Files**             | `files`          | `pi pi-folder-open` | Hierarchical tree of all files in the chart `.tgz` archive              |
| **App** (optional)    | `app`            | `pi pi-cloud`       | Application-specific view (only shown when chart has an associated app) |

### Settings Page Tabs

| Tab                         | URL `tab=` value | Content                                                                     |
| --------------------------- | ---------------- | --------------------------------------------------------------------------- |
| **Grouping Keys** (default) | `groupingKeys`   | Custom entity categories (key/value pairs, fallback key)                    |
| **Predefined Filters**      | `filterGroups`   | Predefined filter expression definitions (recursive expression tree editor) |
| **Graph**                   | `graph`          | Grouping levels, layout direction, clone rules, color rules                 |
| **Search**                  | `search`         | Search tree grouping, search fields, match configuration                    |
| **State (YAML)**            | `state`          | Raw YAML state editor (StateEditor component)                               |
| **Reset**                   | `reset`          | Reset all settings to defaults                                              |

## URL Hash Format

```text
#<clusterName>?page=<page>&node=<entityId>&tab=<tab>
```

| Parameter     | Required | Default      | Description                                                                                                                                         |
| ------------- | -------- | ------------ | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `clusterName` | yes      | `in-cluster` | Cluster name (before `?`)                                                                                                                           |
| `page`        | no       | `list`       | Active page: `list`, `graph`, `charts`, `details`, `settings`, `search`                                                                             |
| `node`        | no       | —            | Entity ID (only for `details` page)                                                                                                                 |
| `tab`         | no       | —            | Active sub-tab within a page (e.g. `relations`, `outgoing-rbac` for details; `groupingKeys`, `graph` for settings)                                  |
| `q`           | no       | —            | Fulltext search query (search page)                                                                                                                 |
| `sources`     | no       | —            | Comma-separated search source types (search page)                                                                                                   |
| `appId`       | no       | —            | App ID for chart selection (e.g. `dev.demo-infra.operator-clickhouse`). When present on the `charts` page, the corresponding chart is auto-selected. |

Filters are managed exclusively via `activeFilterSlots` in localStorage (see [filters.md](filters.md)). Old filter URL parameters (`f.`, `x.`, `n.`) are ignored if present.

### Examples

```text
#in-cluster                                              → List page (default)
#dev?page=graph                                          → Graph page
#dev?page=charts                                         → Charts page
#dev?page=details&node=v1/Pod/default/nginx              → Details page, details tab (default)
#dev?page=details&node=v1/Pod/default/nginx&tab=relations → Details page, relations tab
#dev?page=details&node=v1/Secret/ns/s&tab=secrets        → Details page, secrets tab
#dev?page=details&node=v1/Pod/default/nginx&tab=outgoing-rbac → Details page, outgoing RBAC tab
#dev?page=details&node=v1/Pod/default/nginx&tab=clone    → Details page, clone tab
#dev?page=settings                                       → Settings page, groupingKeys tab (default)
#dev?page=settings&tab=graph                             → Settings page, graph tab
#dev?page=search                                         → Fulltext search page
#dev?page=charts&appId=dev.demo-infra.operator-clickhouse    → Charts page with operator-clickhouse selected
#dev?page=charts&appId=dev.demo-infra.operator-clickhouse&tab=values → Charts page, values tab, operator-clickhouse
```

## App Selection Deep Links

When `appId` is present in the URL on the charts page, the corresponding chart is auto-selected (equivalent to calling `handleOpenChart`).

- `setAppId(appId)` updates the URL with the new `appId` via `replaceState` (no history entry).
- `setAppId(undefined)` removes the `appId` parameter from the URL.
- The dedicated `setAppId` method is used instead of `setPage("charts", ...)` — this keeps the URL update separate from page navigation and avoids creating a browser history entry.
- Changing the chart selection in the UI calls `setAppId` with the new app ID.
- Clearing the chart selection calls `setAppId(undefined)`.
- The `appId` parameter is only meaningful on the `charts` page; on other pages it is ignored.

All hash-updating methods (`setTab`, `setSearchParams`, `setAppId`) read the current hash state via `parseHash` before rebuilding. This ensures that `appId` is preserved across tab switches and search parameter updates. The implementation must pass `current.appId` through when calling `buildHash`.

Auto-selection is handled in `App.tsx`: a `useEffect` watches `navState.appId` and, when it changes on the charts page, calls `handleOpenChart` with the matching chart name. This sets both `chartPageSelectedName` and the `selectionSlot` filter.

## Sidebar Header

The sidebar header contains the page switcher and quick-access icons:

```text
┌──────────────────────────────────────────────────┐
│  🐉 Hydra  cluster  [🔍] [🏠] [📊] [◈]  🌙  ⚙ │
│                       ▲    ▲    ▲    ▲    ▲   ▲  │
│                       │    │    │    │    │   └ Settings page
│                       │    │    │    │    └ Theme toggle
│                       │    │    │    └ Graph page
│                       │    │    └ Charts page
│                       │    └ List page (Home)
│                       └ Search page
└──────────────────────────────────────────────────┘
```

- **Search icon** (`Search`): Navigates to fulltext search page. Highlighted when search page is active.
- **Home icon** (`Home`): Navigates to list page. Highlighted when list page is active.
- **Charts icon** (`List`): Navigates to charts page. Highlighted when charts page is active.
- **Graph icon** (`Hub`): Navigates to graph page. Highlighted when graph page is active.
- **Theme toggle**: Cycles between auto/light/dark.
- **Settings icon** (`Settings`): Opens the settings page. Highlighted when settings page is active.

## Navigation Behavior

### Close/Back

| Context       | Action  | Behavior                                          |
| ------------- | ------- | ------------------------------------------------- |
| Details page  | Click ✕ | `history.back()` — returns to previous state      |
| Settings page | Click ✕ | `history.back()` — returns to previous state      |
| Search page   | Click ✕ | `history.back()` — returns to previous state      |
| List page     | —       | No close button (home page)                       |
| Graph page    | —       | No close button (use Home icon to return to list) |
| Charts page   | —       | No close button (use Home icon to return to list) |

### Escape Key

| State                                     | Behavior                            |
| ----------------------------------------- | ----------------------------------- |
| Details, settings, or search page is open | `history.back()` — closes the page  |
| On list, graph, or charts page            | Toggles between list and graph page |

### Breadcrumb

Entity pages display a breadcrumb in the header bar:

```text
Secret / my-secret / Secrets
  ▲         ▲          ▲
  Kind      Name       Active Tab
```

This provides context about the current entity and which tab is active.

### Sidebar Clicks

Clicking an entity in the sidebar (Files tree, Nodes tree) opens the entity details page (`page=details&node=<id>`), regardless of the current page.

## State Model

### Types (`state.ts`)

```typescript
/** Top-level page types. */
export type PageType =
  | "list"
  | "graph"
  | "charts"
  | "details"
  | "settings"
  | "search";
```text

### Navigation Hook (`useHashNavigation.ts`)

```typescript
type HashNavState = {
  clusterName: string;
  page: PageType;
  nodeId: string;
  tab?: string;
  searchQuery?: string;
  searchSources?: string[];
  appId?: string;
};

function useHashNavigation(): [
  HashNavState,
  {
    setPage: (
      page: PageType,
      nodeId?: string,
      tab?: string,
      search?: SearchHashParams,
    ) => void;
    setTab: (tab: string | undefined) => void;
    setSearchParams: (query: string, sources: string[]) => void;
    setAppId: (appId: string | undefined) => void;
    goBack: () => void;
  },
];
```

- `setPage()`: Navigates to a page (pushes history entry via `pushState`). Closes any open detail/settings page when switching to list/graph/charts. Accepts an optional `search` parameter for search-page deep links.
- `setTab()`: Updates the active sub-tab within the current page (uses `replaceState` — no history entry, so tab switches don't clutter browser back/forward)
- `setSearchParams()`: Updates the search query and source filters on the search page (uses `replaceState` — no history entry).
- `setAppId()`: Updates the `appId` in the URL via `replaceState` (no history entry). Pass `undefined` to remove the parameter.
- `goBack()`: Calls `history.back()` to return to previous state

### Pure Helper (`buildHash`)

```typescript
function buildHash(
  clusterName: string,
  page: PageType,
  nodeId: string,
  tab?: string,
  search?: SearchHashParams,
  appId?: string,
): string;
```text

`appId` is the last optional parameter. When set, it is written as `appId=<value>` in the URL query string.

## Settings Page

The settings page (`SettingsPage.tsx`) consolidates all configuration into tabs. The active tab is persisted in the URL via the `tab=` parameter (e.g. `#cluster?page=settings&tab=graph`). Tab switches use `replaceState` so they don't create browser history entries.

### Edit-Copy Pattern

The settings page uses an **edit-copy pattern** for all editable tabs (Filters, Grouping Keys, Graph, Search). Changes are made to a local copy of the state and only committed to the live application state via explicit user action:

| Action     | Behavior                                                               |
| ---------- | ---------------------------------------------------------------------- |
| **Save**   | Apply edit state → live state, then close the settings page            |
| **Apply**  | Apply edit state → live state, keep settings page open                 |
| **Revert** | Reset edit state from current live state (discard uncommitted changes) |
| **Cancel** | Revert + close the settings page                                       |

The action bar is shown at the bottom of the settings page for editable tabs. Save and Apply are disabled when there are no unsaved changes. An "Unsaved changes" indicator is shown when the edit state differs from the live state.

The State (YAML) tab has its own edit-copy pattern (managed by `StateEditor`). The Reset tab has no editable state.

### Unsaved Changes Guard

When the user attempts to switch tabs or close the settings page while there are unsaved changes (`isDirty === true`), a confirmation dialog is shown. The dialog offers three options:

| Option                 | Behavior                                                                                 |
| ---------------------- | ---------------------------------------------------------------------------------------- |
| **Save & continue**    | Apply edit state → live state, then execute the pending navigation (tab switch or close) |
| **Discard & continue** | Revert edit state from live state, then execute the pending navigation                   |
| **Stay on page**       | Cancel the navigation, remain on the current tab                                         |

Implementation details:

- A `pendingAction` state (`{ type: 'tab', tab } | { type: 'close' } | null`) stores the deferred navigation.
- Tab changes via `TabMenu` and the close button (×) are intercepted: if `isDirty`, the action is stored in `pendingAction` and the dialog opens instead of navigating immediately.
- The Cancel button in the action bar already explicitly reverts and closes, so it does **not** trigger the guard.

The `EditableSettings` type bundles all settings that can be edited:

```typescript
type EditableSettings = {
  groupingKeys: GroupingKeyDefinition[];
  filterGroups: FilterGroupDefinition[];
  graphGroupingConfig: GraphGroupingConfig;
  colorRules: GraphColorRule[];
  cloneRules: CloneRule[];
  autoClone: AutoCloneConfig;
  searchGrouping: GroupingField[];
  searchConfig: SearchConfig;
};
```

`App.tsx` passes the current live settings as a `settings` prop and receives committed changes via the `onApply` callback.

## Component Architecture

```text
App.tsx
 ├── Sidebar (left panel)
 │   ├── Logo header with page switcher (Search, Home, Charts, Graph, Theme, Settings)
 │   └── Accordions (Filter, Charts, Files, Nodes, Groups)
 │
 └── Main panel (right)
     ├── EntityListPanel     (when page=list)
     ├── ChartPage           (when page=charts)
     ├── SettingsPage         (when page=settings)
     ├── SearchPage          (when page=search)
     └── GraphPanel          (when page=graph or page=details)
         ├── Cytoscape graph
         ├── Zoom controls
         └── EntityPage      (when page=details, tab determines active tab)
```

## Source Files

| File                                 | Purpose                                                              |
| ------------------------------------ | -------------------------------------------------------------------- |
| `src/state.ts`                       | `PageType` type definition                                           |
| `src/useHashNavigation.ts`           | URL hash parsing/building, navigation hook                           |
| `src/App.tsx`                        | Main orchestrator: page switching, sidebar, routing                  |
| `src/components/GraphPanel.tsx`      | Graph view with entity pages                                         |
| `src/components/EntityPage.tsx`      | Unified entity page (tabs: details, relations, RBAC, secrets, clone) |
| `src/components/EntityListPanel.tsx` | List page (entity table with filters)                                |
| `src/components/SettingsPage.tsx`    | Settings page with tabs                                              |
| `src/components/ChartPage.tsx`       | Charts page                                                          |
| `src/components/SearchPage.tsx`      | Fulltext search page                                                 |

## Tests

### Navigation Tests (`src/__tests__/useHashNavigation.test.ts`)

Tests for `parseHash()` and `buildHash()` in `src/useHashNavigation.ts`.

#### `parseHash`

| Test                                    | Verifies                                                                                   |
| --------------------------------------- | ------------------------------------------------------------------------------------------ |
| Empty hash defaults                     | Empty hash returns `clusterName: "in-cluster"`, `page: "list"`, `nodeId: ""`               |
| Cluster name only                       | `#my-cluster` parses to list page with correct cluster name                                |
| `page=graph`                            | `page` parameter parsed correctly                                                          |
| `page=charts`                           | Charts page parsed correctly                                                               |
| Default list page                       | Missing `page` param defaults to `"list"`                                                  |
| `page=details` with node                | Page type and node ID parsed correctly, `tab` is undefined (default tab)                   |
| `page=details` with node and tab        | `tab` parameter parsed from URL for entity page                                            |
| `page=details` with tab=relations       | Specific entity tab value preserved                                                        |
| `page=details` with tab=outgoing-rbac   | RBAC entity tab parsed correctly                                                           |
| `page=details` with tab=clone           | Clone tab parsed correctly                                                                 |
| `page=settings` without node            | Settings page with empty `nodeId`                                                          |
| `page=settings` with tab                | `tab` parameter parsed from URL                                                            |
| `page=settings` with `tab=groupingKeys` | Specific tab value preserved                                                               |
| Old filter params ignored               | `f./x./n.` params are ignored (filters no longer in URL)                                   |
| Invalid page types rejected             | Unknown page type results in `page: "list"`                                                |
| Cluster names with slashes rejected     | Slash in cluster name falls back to `"in-cluster"`                                         |
| `page=search`                           | Search page type parsed correctly                                                          |
| All valid page types                    | All 6 page types (`list`, `graph`, `charts`, `details`, `settings`, `search`) are accepted |
| `appId` parameter parsed                | `#dev?page=charts&appId=dev.demo.service-auth` parses `appId` correctly                     |
| `appId` absent defaults to undefined    | `#dev?page=charts` has `appId: undefined`                                                  |
| `appId` preserved on non-charts page    | `#dev?page=graph&appId=x` parses `appId: "x"` (stored in state, UI ignores it)             |

#### `buildHash`

| Test                                | Verifies                                                                    |
| ----------------------------------- | --------------------------------------------------------------------------- |
| List page (default)                 | Builds `#my-cluster` with no query params                                   |
| Graph page                          | Includes `page=graph`                                                       |
| Charts page                         | Includes `page=charts`                                                      |
| Details page with node              | Includes `page=details` and `node=` params                                  |
| Details page with node and tab      | Includes `page=details`, `node=` and `tab=` params                          |
| Special chars in cluster name       | Cluster name is URL-encoded                                                 |
| Empty node omitted                  | `node=` param omitted when `nodeId` is empty                                |
| Tab param included                  | `tab=` param included when set                                              |
| Tab param omitted                   | `tab=` param absent when not set                                            |
| Roundtrip settings with tab         | `buildHash` → `parseHash` preserves settings page tab                       |
| Roundtrip details with tab          | `buildHash` → `parseHash` preserves details page entity tab                 |
| Search page                         | Includes `page=search`                                                      |
| Roundtrip search page               | `buildHash` → `parseHash` preserves search page                             |
| `appId` included when set           | `buildHash` with `appId` includes `appId=` param                            |
| `appId` omitted when undefined      | `buildHash` without `appId` omits the param                                 |
| Roundtrip with appId                | `buildHash` → `parseHash` preserves `appId`                                 |
| Roundtrip with dots in appId        | `buildHash` → `parseHash` preserves `dev.demo-infra.operator-clickhouse`     |
| Roundtrip charts with appId and tab | `buildHash` → `parseHash` preserves both `appId` and `tab` together         |
| Full roundtrip                      | Complex state (cluster, page, node, tab) survives `buildHash` → `parseHash` |
