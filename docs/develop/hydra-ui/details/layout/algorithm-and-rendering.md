# Algorithm and Rendering

This page documents how Hydra UI turns the grouped data model into positioned graph nodes, rendered DOM labels, and interactive Cytoscape behavior.

Back to [Graph Layout Architecture](../layout.md).

## Node Types

- **Entity node**: a single Kubernetes object (Pod, Service, Deployment, and so on). Properties: `id`, `name`, `kind`, `namespace`, `apiVersion`, `gvk`, `templatePath`, `templateIndex`, `tags`. Tags are optional string labels (for example `["app:missing"]`) displayed as an italic third line in the node.
- **Group node**: a visual container for entities (for example all Pods in namespace `default`). Properties: `label`, `entityCount`, `depth`, `isExpanded`. Groups can be nested.

## Node Geometry

```typescript
nodeGeometry: Map<entityId, { x; y; width; height }>;
```text

- `x, y` are **absolute coordinates** (not relative to the parent group)
- Only **entity nodes** have their own position and size in the model
- **Group nodes** have no stored position or size; they are computed at render time:
  - **Leaf groups** (no sub-groups): bounding box of their child entities
  - **Non-leaf groups** (containing sub-groups): bounding box of their child **group** bounding boxes — this ensures each nesting level adds its own padding + header, so parent groups are always visibly larger than their children
- **Leaf nodes** (entity nodes) are sized to fit their labels

## Layout Algorithm

The layout effect in `App.tsx` is guarded by an `isGraphVisible` check: it only runs when the graph view is active (or an entity page is open over the graph). When the settings page is open, which covers the entire graph panel, the layout is skipped entirely. This avoids expensive DOM measurements and position calculations while the user is editing settings.

### Step 1: Measure label sizes

**UI (`App.tsx`):** `measureNodeSizes()` in `src/domMeasure.ts`

Inserts each entity label invisibly into the DOM using the exact same HTML and CSS as the `GraphPanel` entity label template, then measures with `getBoundingClientRect()`. The label text is resolved from the `nodeDisplay` configuration (header + description). This produces pixel-accurate sizes based on the actual font and text content, independent of zoom level.

**Unit tests:** `computeNodeSizes()` in `src/layoutLogic.ts`

Uses a deterministic character-width formula (no DOM required):

- `width = max(80, max(kind.length × 6, name.length × 7, tags.join(", ").length × 5) + 16)`
- `height = tags.length > 0 ? 54 : 40` (`14px` extra for the tags line)

Both return `Map<entityId, { width, height }>` and the layout algorithm (`computeRecursiveLayout`) accepts this map regardless of how it was computed.

### Step 2: Compute positions (bottom-up) — `computeRecursiveLayout()` in `src/layoutLogic.ts`

The layout is computed recursively from bottom to top, starting at the leaf groups. It is fully deterministic (no randomness), a pure function, and has no DOM access.

#### Step 2.1: Layout function — `computeNodeLayout()`

A generic layout function computes positions for a flat set of nodes using a layered (Sugiyama-style) algorithm:

**Input:** `nodes: Array<{ id, width, height }>, edges: Array<{ from, to }>, options?: { stackDirection?: "vertical" | "horizontal" }`

**Output:** `Map<nodeId, { x, y }>`

Rules:

- Connected nodes are arranged **left to right** (source left, target right) via topological layering
- Unconnected nodes are arranged according to `stackDirection`:
  - `"vertical"` (default): **top to bottom**
  - `"horizontal"`: **left to right**
- Multiple connected components are stacked in the same direction
- Nodes must **not overlap**

Algorithm:

1. Find connected components (undirected BFS, sorted by ID for determinism)
2. For each component with edges, assign layers via longest-path from sources and position them left to right
3. For each component without edges, stack nodes top to bottom (`vertical`) or left to right (`horizontal`)
4. Stack components top to bottom or left to right according to `stackDirection`

#### Step 2.2: Recursive bottom-up layout — `computeRecursiveLayout()`

**Signature:** `computeRecursiveLayout(tree, references, nodeSizes, layoutDirections?)`

The optional `layoutDirections` array configures the stacking direction at each tree depth. It is derived from `GraphGroupingConfig`:

```typescript
layoutDirections = [
  config.topLevelLayout,               // depth 0: root -> first-level groups
  config.levels[0].layout,             // depth 1: first-level -> second-level groups
  config.levels[1].layout,             // depth 2: second-level -> entities
  ...
]
```

If `layoutDirections[depth]` is not set, defaults are: root (`depth === 0`) = `"horizontal"`, deeper levels = `"vertical"`.

**Process:**

1. **Layout leaf groups:** For each leaf group (groups without children), run `computeNodeLayout()` with the entity nodes, their intra-group edges, and the `stackDirection` for this depth.
2. **Compute bounding box:** After laying out a leaf group, compute its bounding box (position + size of all children + padding + `GROUP_LABEL_HEIGHT`). For non-leaf groups, the bounding box is computed from the positioned child **group** bounding boxes (not raw entity positions), so each nesting level adds its own padding and header.
3. **Replace with bounding box node:** The laid-out leaf groups are replaced by new nodes with the size of their bounding box. This turns the parent groups into new leaf groups.
4. **Repeat recursively:** Run `computeNodeLayout()` on the parent group with the `stackDirection` for the current depth. Inter-group edges are derived from entity-level references that cross group boundaries.
5. **Shift to absolute coordinates:** After positioning groups, all entity positions within each group are shifted by the group's computed offset, so that all `x, y` values in `nodeGeometry` are always **absolute coordinates**.

```text
Example: grouping [{namespace, L->R}, {entityGroup, T->B}], topLevel=horizontal

1. Layout entityGroup "frontend":  [pod-1] -> [svc-1]     -> bounding box
2. Layout entityGroup "backend":   [pod-2] -> [svc-2]     -> bounding box
3. Layout namespace "default":     [frontend-bbox] [backend-bbox]  (direction: L->R from levels[0])
4. Layout root:                    [default-bbox] [kube-system-bbox]  (direction: horizontal from topLevel)
5. Shift all entity positions to absolute coordinates
```text

### Step 3: Render nodes to the DOM

`GraphPanel` receives `graphNodes` with final positions and resolved display strings (`header`, `description`, `tooltip`, `tags`). Nodes are rendered directly at the correct location: no layout computation happens in the DOM.

**Entity node label template** (via `cytoscape-node-html-label`):

1. **Header**: first line, bold, `9px` font (for example the entity's `kind`)
2. **Description**: second line, `12px` font (for example the entity's `name`)
3. **Tags**: optional third line, italic, `9px` font, `85%` opacity (for example `app:missing`). Only shown if the entity has tags.

Node size is computed to fit all lines: base height is `40px` without tags and `54px` with tags (`+14px` for the tags line). The `domMeasure.ts` module uses actual DOM measurement in the UI; `layoutLogic.ts` uses a deterministic formula for unit tests.

Group nodes (background rectangles) are computed differently depending on nesting:

- **Leaf groups** (no sub-groups): bounding box of their child entity positions + padding + label header
- **Non-leaf groups** (containing sub-groups): bounding box of their child **group** bounding boxes + padding + label header, so a parent group is always visibly larger than its children

### Graph Interaction

The rendered graph supports direct mouse interaction:

- **Left click** on a node or group selects it (`Ctrl`/`Cmd`+click for multi-select); clicking the background deselects all
- **Left drag** on a node or group moves it via drag and drop
- **Left drag** on the background draws a **rubber band** rectangle; on release, the view zooms to fit the selected area. The rubber band is cancelled (no zoom) when:
  - The selection is **smaller than 10px** in either dimension (avoids accidental zooms)
  - **Right click** during the drag (switches to panning)
  - **Escape** key
  - Releasing the mouse button outside the graph container
- **Right click** on an entity node opens the **circular context menu** (`cxtmenu`) with commands: Details (opens Entity Page), Show or Hide Dependencies, Clone (clone nodes only)
- **Long press** on an entity node also opens the context menu (alternative to right-click)
- **Right drag** on the background (or group nodes) pans (scrolls) the entire view
- **Mouse wheel** zooms in or out centered on the cursor position
- **Zoom toolbar buttons** (`Fit All`, `Fit Selection`, `Zoom In`, `Zoom Out`) zoom centered on the viewport middle

#### Context Menu (cxtmenu)

Entity nodes feature a circular **context menu** powered by [cytoscape-cxtmenu](https://github.com/cytoscape/cytoscape.js-cxtmenu). The menu opens on **right-click** (`cxttapstart`) or **long-press** (`taphold`) on any entity node. It uses `outsideMenuCancel: 10` to close the menu when the mouse is released more than `10px` outside the menu area.

**Menu commands** are dynamically generated per node via a function:

| Command                       | Condition                             | Action                                                                                                    |
| ----------------------------- | ------------------------------------- | --------------------------------------------------------------------------------------------------------- |
| **📋 Details**                | All entity nodes                      | Opens the Entity Page (Details tab): sortable references table with distance, relation, and reverse info. |
| **⇄ Show Deps / ⇄ Hide Deps** | All entity nodes                      | Toggles dependency highlighting (`showDependencies`). The command label reflects the current state.       |
| **⧉ Clone**                   | Clone nodes only (`isClone === true`) | Opens the [Clone Page](cloning-and-colors.md#clone-page) for the clicked clone node.                      |

The Entity Page provides tabs (`Details`, `Outgoing RBAC`, `Incoming RBAC`, `Secrets`) and the context menu opens the `Details` tab as the default entry point. Full RBAC and entity-page details live in [rbac-display.md](../rbac-display.md).

**Right-click handling:** When the user right-clicks on an entity node, the `cxtmenu` handles the interaction. The right-click pan handler explicitly checks whether an entity node is under the cursor and skips panning when the context menu is active. Right-clicking on the background or on group nodes still initiates panning as before.

**Implementation:**

- `cytoscape-cxtmenu` extension registered globally alongside `cytoscape-node-html-label`
- `cxtmenu` initialized in the `GraphPanel` Cytoscape init effect with `selector: 'node[type="entity"]'`
- Callback refs (`showDependenciesRef`, `onToggleDependenciesRef`, `onNodeSelectRef`) used to access the latest React state from `cxtmenu` command functions (avoids stale closures)
- Type declaration in `src/cytoscape-cxtmenu.d.ts`

#### Entity Page (Tabbed)

The Entity Page is a unified, tabbed page providing detailed information about any entity. Full details live in [rbac-display.md](../rbac-display.md).

**Tabs:**

| Tab               | Description                                                                                                                   |
| ----------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Details**       | Entity metadata table (`ID`, `GVK`, `namespace`, `name`, `tags`, `secret keys`) with quick-add filter actions per row         |
| **Outgoing RBAC** | 3-level permissions table (`API Group/Resource -> Scope -> Source`) aggregated from outgoing-reachable Roles and ClusterRoles |
| **Incoming RBAC** | Who can access this resource type, with `GVK` chip filter and `"All namespaces"` checkbox                                     |
| **Secrets**       | Secret keys and consumers (only visible for `kind === "Secret"` entities)                                                     |

**Navigation model:**

- **Tab switch** changes the `page` URL parameter and keeps the same entity
- **Entity link** changes the `node` URL parameter and keeps the same tab
- Every change creates a browser history entry (`Back`/`Forward` works)

**Interaction:**

- **Left-click entity node:** first click selects the node. Clicking an already selected node opens the Entity Page (`Details` tab).
- **Right-click entity node -> `📋 Details`:** opens the Entity Page (`Details` tab)
- **Click a tab:** switches to that tab (same entity)
- **Click an entity link in any tab:** navigates to that entity (same tab)
- **Click a Level 1 row (RBAC tabs):** expands or collapses to show scopes and sources
- **Row/column hover (RBAC tabs):** highlights the entire row and column (verb columns only)
- **Click backdrop / Escape:** closes the page
- **Click any entity node while a page is open:** navigates to that entity in the current tab

**Implementation:**

- Unified `EntityPage` component in `src/components/EntityPage.tsx` with tab content: `DetailsTabContent`, `OutgoingRbacTabContent`, `IncomingRbacTabContent`, `SecretsTabContent`
- Pure logic imports: `normaliseRbacRules()` and `computeVerbCoverage()` from `src/components/RbacInfoPanel.tsx`, `analyseIncomingRbac()` from `src/incomingRbac.ts`
- Shared RBAC display types in `src/model.ts` (`STANDARD_VERBS`, `RbacRoleVerbEntry`, `RbacDisplayScope`, outgoing and incoming types)
- RBAC rule aggregation via outgoing reachability in `App.tsx` (`graphNodes` `useMemo`, `annotateRules()`)
- hydra-go: `RbacRuleModel` / `extractRbacRules()`, `extractSecretKeys()` / `extractSopsSecretKeys()` in `core/view/dependencies.go`
- Tests: `src/__tests__/rbacNormalise.test.ts` (26 tests), `src/__tests__/incomingRbac.test.ts` (12 tests)

#### Selection Highlighting (Reachability)

When one or more nodes are selected, all **transitively reachable** nodes and edges are highlighted, exclusively via the `ReachabilityMap`. No nodes are dimmed.

**Distance badges:** Every reachable **entity node** (including the selected node) displays a round badge in its **upper-right corner** showing the BFS distance. Group nodes do not show badges.

- **0**: the selected node itself
- **1**: direct neighbor
- **2**: two hops away, and so on

The distance is **signed** to indicate direction:

| Distance        | Meaning                                              |
| --------------- | ---------------------------------------------------- |
| `0`             | The selected node itself                             |
| `+N` (positive) | Reachable via **outgoing** edges (`N` hops forward)  |
| `-N` (negative) | Reachable via **incoming** edges (`N` hops backward) |

If a node is reachable via both directions, the distance with the **smaller absolute value** is shown (outgoing preferred on tie). All badges, highlighted edges, and the selected-node border use the same **red** color (`#ff0000`).

**Edge highlighting:** All edges along transitive paths are highlighted in **red** (`#ff0000`). Other edges keep their normal theme color.

**Implementation:** `GraphPanel` receives `reachability: ReachabilityMap` and `entityToNodeId: Map<entityId, cytoscapeNodeId>` as props. The `entityToNodeId` mapping (computed in `App.tsx` from `visibleEntities` + `collapsedGroups`) translates entity IDs to their visible Cytoscape node IDs, handling collapsed groups transparently.

### Step 4: Fit-all zoom and hide loading screen

While steps 1-3 are running, `isLayoutPending` is `true`. During this time, a `ProgressSpinner` loading indicator is shown over the graph area and `graphNodes` returns an empty array to prevent a flash of unpositioned nodes.

After the layout completes (`nodeGeometry` is populated), the graph performs an **automatic fit-all zoom** before hiding the loading indicator:

1. `isZoomPending` is set to `true` (keeps the loading indicator visible)
2. `nodeGeometry` is set (nodes are rendered by `GraphPanel` behind the loading indicator)
3. `GraphPanel`'s sync effect adds nodes to Cytoscape **and re-applies the selection** (via a `selectedNodeIdsRef`) — this is necessary because the selection effect may have run before the nodes existed
4. The `onSyncComplete` callback (`handleSyncComplete` in `App.tsx`) calls `cy.resize()` to ensure Cytoscape has correct container dimensions (the `GraphPanel` container may have been hidden with `display: none` on a different page), then `cy.fit(undefined, 50)` to zoom the viewport to encompass all nodes
5. `isZoomPending` is set to `false`, hiding the loading indicator

**Fit-all zoom triggers:** This step fires on the initial graph open and whenever the layout is recalculated (filter changes, grouping changes, layout direction changes, node display changes): any event that resets `nodeGeometry` to an empty map. Navigating away from the graph and back does **not** trigger a zoom; the viewport is preserved.

**`cy.resize()`:** All zoom operations (`fit-all`, `fit-selection`, `zoom-in`, `zoom-out`) call `cy.resize()` first to update Cytoscape's cached container dimensions. This is necessary because `GraphPanel` is always mounted in the DOM but hidden with `display: none` when another page is active.

The loading indicator is shown whenever `isLayoutPending || isZoomPending`, ensuring the user never sees an unzoomed flash of nodes.

## Data Flow

```text
HydraData (parseHydraYaml)
  │
  ├── parseEntityId(id) per entity                                (parseHydra.ts)
  │     -> HydraEntity { id, group, version, apiVersion, kind, namespace, name, gvk,
  │                      templatePath, templateIndex }
  │
  ├── buildReachabilityMap(entities, refs) -> reachability        (parseHydra.ts, on load)
  │
  ├── filters + reachability -> filteredEntities                  (App.tsx useMemo)
  │
  ├── -- Grouping Pipeline ---------------------------------------
  │
  ├── buildEntityGroupMap(groups)                                 (treeLogic.ts)
  │     -> entityGroupMap: Map<entityId, groupName>
  │
  ├── buildNscsMap(entities, entityGroupMap, groups, refs)        (treeLogic.ts)
  │     -> nscsMap: Map<entityId, resolvedNamespace>
  │     Uses unique group IDs (not names) for peer resolution.
  │     Uses entityGroupMap as fallback for clone entities not in groups[].ids.
  │     Strategies: 1) group peers, 2) direct refs, 3) group refs.
  │     Iterative: resolves transitive chains (CR<-CRB->SA) across passes.
  │
  ├── -- Cloning -------------------------------------------------
  │
  ├── cloneEntities(entities, refs, rules, groups,               (cloneLogic.ts)
  │                  entityGroupMap, nscsMap)
  │     -> clonedEntities, clonedReferences, cloneIds
  │     -> entityGroupOverrides, nscsOverrides
  │     per=entityGroup: referencing entities' (entityGroup, nscs) compound pairs -> clone targets.
  │     per=nscs: referencing entities' nscs values -> clone targets.
  │
  ├── buildTree(clonedEntities, grouping, entityGroupMap,        (treeLogic.ts)
  │             nscsMap)
  │     Per entity: getFieldValue(entity, field, entityGroupMap, nscsMap)
  │     -> bucket by value -> recurse per grouping level
  │     -> graphTree: TreeNode
  │
  ├── -- Layout & Rendering -------------------------------------
  │
  ├── measureNodeSizes(entities, nodeDisplay) -> nodeSizes       (domMeasure.ts, DOM)
  │     (tests use computeNodeSizes from layoutLogic.ts instead)
  │
  ├── expandedGroups + graphTree                                 (graphModel.ts)
  │     -> visibleEntities + collapsedGroups
  │
  ├── computeRecursiveLayout(tree, refs, nodeSizes)              (layoutLogic.ts)
  │     │
  │     ├── computeNodeLayout() per leaf group             <- step 2.1
  │     ├── bounding box -> parent group layout            <- step 2.2
  │     └── shift to absolute coords -> nodeGeometry       <- step 2.3
  │
  ├── computeGraphNodes(visibleEntities, collapsedGroups,        (graphModel.ts)
  │     expandedGroups, nodeGeometry, tree, levelDisplays?,
  │     nodeDisplay?, entityMap?, colorRules?, entityGroupMap?,
  │     nscsMap?, groupingKeyMaps?)
  │     │
  │     ├── entity nodes: x, y from nodeGeometry + header/desc/tooltip from nodeDisplay + tags
  │     ├── group nodes: header/desc/tooltip from levelDisplays[depth]
  │     ├── leaf group nodes: bounding box from child entities
  │     └── non-leaf group nodes: bounding box from child group bboxes
  │
  ├── visibleEntities + collapsedGroups -> entityToNodeId        (App.tsx useMemo)
  │
  └── GraphPanel renders Cytoscape (layout: "preset")            (GraphPanel.tsx)
        │
        ├── receives reachability + entityToNodeId
        ├── on selection: BFS distance via ReachabilityMap
        ├── edge highlighting: outgoing + incoming in red
        └── distance badges: round badge (upper-right) per reachable node
```
