# Layout Tests

This page documents the Vitest coverage for the graph layout pipeline, including grouping, cloning, colors, graph models, and end-to-end golden-file scenarios.

Back to [Graph Layout Architecture](../layout.md).

Most tests use Vitest and follow the golden-file pattern: YAML input model -> function under test -> comparison against `*.expected.yaml`. Label sizes are included in the model so no DOM access is needed.

Test files live in `src/__tests__/`. Each test loads a YAML model from `src/__tests__/testdata/`, runs the function under test, and compares the output against the corresponding expected file in `src/__tests__/testdata/golden/`.

Each test has at least two given/expected file pairs (sub-cases) that are discovered dynamically via `import.meta.glob`. Adding a new sub-case only requires adding new YAML files: no code changes are needed. File naming convention: `<testName>.<caseName>.given.yaml` and `<testName>.<caseName>.expected.yaml`.

## Clone Tests (`src/__tests__/cloneLogic.test.ts`)

Tests for `cloneEntities()`, `buildCloneSiblingMap()`, `expandCloneSelection()`, `buildEdgeCounts()`, `buildAutoCloneRules()`, and `getEffectiveCloneRules()` in `src/cloneLogic.ts`.

### Core Clone Tests

| Test                             | Verifies                                                                                                                               |
| -------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------- |
| `clone-by-nscs`                  | `CustomResourceDefinition` referenced from two namespaces -> clone per `nscs`, original removed, references rewired                    |
| `clone-by-nscs-multi-crd`        | Two CRDs in separate groups, each referenced from different namespaces -> cloned independently                                         |
| `clone-no-match`                 | Rule matches no entities: input unchanged                                                                                              |
| `clone-no-group`                 | Matched entity not in any entity group: kept as-is (no cloning without group)                                                          |
| `clone-single-ns`                | All referencing entities in one namespace -> single clone replaces original                                                            |
| `clone-per-nscs-integration`     | `PrometheusRule` scenario: CRD + CRs from `demo` + `monitoring`, `per=nscs` -> 2 clones, `nscsOverrides` set, no `entityGroupOverrides` |
| `clone-per-entitygroup`          | `per=entityGroup`: same-group (2 `nscs`) -> 2 clones; two-groups (2 referencing groups) -> 2 clones in referencing groups              |
| integration: per `nscs` + `NSCS` | Full pipeline: `nscsOverrides`, references rewired, filtered entities                                                                  |
| integration: per `entityGroup`   | Shared -> clones per `(entityGroup, nscs)` pair; same-group same-`nscs` -> 1 clone; same-group diff-`nscs` -> `N` clones               |
| `buildCloneSiblingMap`           | Clones from the same original are grouped; each clone maps to its sibling set                                                          |
| `expandCloneSelection`           | Selecting one clone expands to all siblings; non-clone IDs pass through; empty sibling map returns original set                        |

### Auto-Clone Tests

| Test                                        | Verifies                                                      |
| ------------------------------------------- | ------------------------------------------------------------- |
| `buildEdgeCounts` empty                     | Returns empty maps for no references                          |
| `buildEdgeCounts` incoming/outgoing         | Correctly counts incoming and outgoing non-reverse edges      |
| `buildEdgeCounts` excludes reverse          | Edges with `reverse: true` are excluded from counts           |
| `buildEdgeCounts` multiple targets          | Correctly counts edges to multiple target entities            |
| `buildAutoCloneRules` no threshold exceeded | Returns empty array when no entity exceeds either threshold   |
| `buildAutoCloneRules` incoming threshold    | Generates rules for entities exceeding the incoming threshold |
| `buildAutoCloneRules` outgoing threshold    | Generates rules for entities exceeding the outgoing threshold |
| `buildAutoCloneRules` deduplication         | Entities exceeding both thresholds produce only one rule      |
| `buildAutoCloneRules` exact threshold       | Entities exactly at the threshold (`>=`) are included         |
| `buildAutoCloneRules` below threshold       | Entities below the threshold are excluded                     |
| `buildAutoCloneRules` per field             | Generated rules use the configured `per` field                |
| `buildAutoCloneRules` deterministic sort    | Rules are sorted by entity ID for deterministic output        |
| `getEffectiveCloneRules` disabled           | Returns only manual rules when auto-clone is disabled         |
| `getEffectiveCloneRules` prepends auto      | Auto-rules are prepended before manual rules when enabled     |
| `getEffectiveCloneRules` no auto matches    | Returns only manual rules when no entity exceeds a threshold  |
| `getEffectiveCloneRules` empty manual       | Works correctly with an empty manual rule list                |

## Filter Tests (`src/__tests__/filterLogic.test.ts`)

Tests for `applyFilters()` in `src/filterLogic.ts`.

| Test                     | Verifies                                                                                                   |
| ------------------------ | ---------------------------------------------------------------------------------------------------------- |
| `filter-by-namespace`    | Single-field namespace filter reduces the entity set correctly                                             |
| `filter-multiple-fields` | Multiple filters are applied sequentially (AND logic)                                                      |
| `filter-include-refs`    | `includeRefs: true` expands the filtered set to include all transitively reachable entities via references |
| `filter-empty-result`    | A filter matching no entities produces an empty set                                                        |

## Tree Building Tests (`src/__tests__/treeLogic.test.ts`)

Tests for `buildTree()` and `buildNscsMap()` in `src/treeLogic.ts`. Uses golden-file comparison via `serializeTree()`.

| Test                                             | Verifies                                                                                                                                                               |
| ------------------------------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `tree-single-grouping`                           | Single grouping level (`namespace`)                                                                                                                                    |
| `tree-multi-grouping`                            | Multi-level nested tree (`namespace` + `entityGroup`)                                                                                                                  |
| `tree-entity-ungrouped`                          | Entities without group get `entityGroup: "(ungrouped)"`                                                                                                                |
| `tree-entity-in-multiple-groups`                 | Entity in multiple groups appears in each                                                                                                                              |
| `tree-nscs-entityGroup`                          | `NSCS` grouping assigns cluster-scoped resources to their group's single namespace                                                                                     |
| `tree-nscs-only`                                 | `NSCS`-only grouping (flat namespace grouping with cluster-scope resolution)                                                                                           |
| `tree-nscs-rbac-chain`                           | `NSCS` with RBAC chain scenario (`Deployment -> SA <- RB -> Role`)                                                                                                     |
| `tree-nscs-no-groups`                            | `NSCS` behaves like `namespace` when no groups are defined                                                                                                             |
| `tree-nscs-cluster-only-refs`                    | `NSCS` Strategy 3: cluster-only group resolved via group references to a single namespace                                                                              |
| `buildNscsMap` Strategy 1 (unit)                 | Group peers: single namespace -> resolves; multiple namespaces -> stays cluster-scoped; ungrouped -> stays                                                             |
| `buildNscsMap` Strategy 2 (unit)                 | Direct refs: `ClusterRoleBinding` in multi-namespace group with single-namespace refs -> resolves; multi-namespace refs -> stays; no refs param -> stays               |
| `buildNscsMap` Strategy 2 transitive (unit)      | `ClusterRole` resolved transitively via `ClusterRoleBinding` that was resolved in a previous pass (`CRB -> SA -> monitoring`, then `CR` via `CRB`)                     |
| `buildNscsMap` transitive chain (unit)           | 3-hop chain: `SA(monitoring) <- CRB -> CR <- AggregatedCR`. All cluster-scoped entities resolve to `monitoring` across 3 passes. Multi-ref `CRB` stays cluster-scoped. |
| `buildNscsMap` Strategy 3 (unit)                 | Group refs: cluster-only group with single-namespace refs -> resolves; multi-namespace refs -> stays; no refs -> stays                                                 |
| `buildNscsMap` same-name groups (unit)           | Multiple groups named `"Shared"`: a small group (single namespace) resolves correctly despite a big group (multi namespace) sharing the same name                      |
| `buildNscsMap` clone via `entityGroupMap` (unit) | Replication clone not in `groups[].ids` is resolved to the correct namespace via `entityGroupMap` fallback (Strategy 1 peer resolution)                                |

## Expand/Collapse Tests (`src/__tests__/graphModel.test.ts`)

Tests for `computeVisibleEntities()` in `src/graphModel.ts`.

| Test                        | Verifies                                                                                |
| --------------------------- | --------------------------------------------------------------------------------------- |
| `collapse-group-visibility` | Collapsing a group hides its entities; only the group node remains as a collapsed entry |
| `expand-all-default`        | When all groups are expanded, all entities are visible and no groups are collapsed      |

## Reference / Edge Tests (`src/__tests__/graphModel.test.ts`)

Tests for `computeGraphEdges()` in `src/graphModel.ts`. Includes passing through the `optional:ref` tag on edges.

| Test                      | Verifies                                                                                   |
| ------------------------- | ------------------------------------------------------------------------------------------ |
| `collapse-edge-rerouting` | Edges targeting entities inside a collapsed group are rerouted to the collapsed group node |
| `references-basic`        | Edges are created correctly between visible entities from the reference list               |
| `references-filtered-out` | References involving filtered-out entities are excluded from the edge list                 |

## Layout Tests (`src/__tests__/layoutLogic.test.ts`)

Tests for `computeNodeLayout()` and `computeRecursiveLayout()` in `src/layoutLogic.ts`. Uses a deterministic layered layout algorithm (Sugiyama-style): fully reproducible golden files, no randomness. Each test additionally verifies structural properties (left-to-right, top-to-bottom, no overlap).

| Test                         | Verifies                                                                 |
| ---------------------------- | ------------------------------------------------------------------------ |
| `layout-connected-lr`        | Connected nodes are arranged left to right (source left, target right)   |
| `layout-unconnected-tb`      | Unconnected nodes are arranged top to bottom                             |
| `layout-no-overlap`          | No nodes overlap after layout computation                                |
| `layout-recursive-bottom-up` | Full recursive bottom-up layout with bounding boxes across nested groups |

## Graph Node Computation Tests (`src/__tests__/graphModel.test.ts`)

Tests for `computeGraphNodes()` in `src/graphModel.ts`. `nodeGeometry` is provided as input in the YAML fixture, so no DOM or layout work is needed.

| Test                             | Verifies                                                                                              |
| -------------------------------- | ----------------------------------------------------------------------------------------------------- |
| `graph-nodes-entity-positions`   | `computeGraphNodes()` assigns entity nodes their position from `nodeGeometry`                         |
| `graph-nodes-group-bounding-box` | Group nodes are computed as the bounding box of their child entities (`padding + GROUP_LABEL_HEIGHT`) |

## End-to-End Data Flow Test (`src/__tests__/graphModel.test.ts`)

Tests the complete data flow from filtering through tree building, expand/collapse, and edge computation.

| Test                     | Verifies                                                                                                     |
| ------------------------ | ------------------------------------------------------------------------------------------------------------ |
| `dataflow-full-pipeline` | Filter -> tree -> `visibleEntities`/`collapsedGroups` -> edges, including edge rerouting to collapsed groups |

## Grouping Key Tests (`src/__tests__/groupingKeyLogic.test.ts`)

Tests for `buildGroupingKeyMaps()` and `allGroupingKeyNames()` in `src/groupingKeyLogic.ts`.

| Test                                                   | Verifies                                                                                           |
| ------------------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| `buildGroupingKeyMaps` empty keys                      | Returns empty map                                                                                  |
| `buildGroupingKeyMaps` single key by group             | Assigns matching entities to the entry key, others to the fallback                                 |
| `buildGroupingKeyMaps` multiple entries                | Assigns entities to the correct entry keys; unmatched entities use the fallback                    |
| `buildGroupingKeyMaps` mixed fields (namespace + kind) | Each entry matches using its own source field                                                      |
| `buildGroupingKeyMaps` by namespace                    | Matches entities by the `namespace` field                                                          |
| `buildGroupingKeyMaps` by kind                         | Matches entities by the `kind` field                                                               |
| `buildGroupingKeyMaps` default K8s key                 | Standard Kubernetes entities -> `"true"`, custom -> `"false"`                                      |
| `buildGroupingKeyMaps` multiple independent keys       | Each key is resolved independently                                                                 |
| `buildGroupingKeyMaps` unknown field                   | Assigns the fallback for an unknown field                                                          |
| `buildGroupingKeyMaps` entityGroup                     | Matches by `entityGroup`                                                                           |
| `buildGroupingKeyMaps` templatePath fallback           | Entities with and without `templatePath` are assigned to the fallback when no entries match        |
| `buildGroupingKeyMaps` templatePath matching           | Entry with `field: "templatePath"` matches entities by their template-path value                   |
| `buildGroupingKeyMaps` chained `gk:` references        | Key entries can reference previously resolved keys via `gk:<keyName>`                              |
| `buildGroupingKeyMaps` YAML defaults chain             | `groupingKeys` from `hydra-ui-defaults.yaml` resolve correctly, including chained `gk:` references |
| `allGroupingKeyNames`                                  | Returns all key names from the list                                                                |

## Color Logic Tests (`src/__tests__/colorLogic.test.ts`)

Tests for `autoColor()`, `resolveColorFromRules()`, `COLOR_PALETTE_HUES`, and `COLOR_PALETTE_GREYS` in `src/colorLogic.ts`.

### `autoColor`

| Test                                      | Verifies                                                                   |
| ----------------------------------------- | -------------------------------------------------------------------------- |
| Returns palette color                     | `autoColor("default")` returns a valid hex color from `COLOR_PALETTE_HUES` |
| Deterministic                             | The same input value always produces the same color                        |
| Different values produce different colors | At least 2 distinct colors among 5 different input values                  |
| Empty string                              | `autoColor("")` returns a valid hex color                                  |

### `resolveColorFromRules`

| Test                                       | Verifies                                                                   |
| ------------------------------------------ | -------------------------------------------------------------------------- |
| No rules -> `undefined`                    | Empty rule list returns `undefined` for both node and group elements       |
| Auto rule matching                         | Auto mode returns the deterministic palette color for the matched value    |
| Auto rule targets both group and node      | Rule with `target: "all"` matches both element types                       |
| Auto rule non-matching                     | Non-matching field value returns `undefined`                               |
| Color (fixed) mode                         | Returns the exact hex color specified in the rule                          |
| Unchanged mode                             | Returns `undefined` for a matching value (element keeps its default color) |
| Target filtering (group only)              | Rule with `target: "group"` only matches group elements, not nodes         |
| Target filtering (node only)               | Rule with `target: "node"` only matches node elements, not groups          |
| First match wins                           | When multiple rules match, the first rule's color is used                  |
| Mixed field rules: cluster scoped          | Empty namespace matches the cluster-scoped group rule                      |
| Mixed field rules: `kube-system`           | Namespace rule matches for both group and node targets                     |
| Mixed field rules: `Pod` by kind           | Kind-based auto rule matches `Pod` nodes correctly                         |
| Mixed field rules: `Pod` rule skips groups | Node-only kind rule does not match group elements                          |
| `all` target matches group                 | Rule with `target: "all"` matches group elements                           |
| `all` target matches node                  | Rule with `target: "all"` matches node elements                            |

### `resolveColorFromRules` with `gk:` field

| Test                           | Verifies                                                               |
| ------------------------------ | ---------------------------------------------------------------------- |
| Cluster-scoped group dark grey | Cluster-scoped rule matches before `gk:` rules for group targets       |
| `gk:` dark blue                | Non-cluster-scoped entities with matching `gk:` value get dark blue    |
| Apps `gk:`                     | Entities with `gk:Category: "Apps"` match the corresponding color rule |
| No matching `gk:`              | Entities with unmatched `gk:` value get no color                       |

### Color Palette Structure

| Test              | Verifies                                                        |
| ----------------- | --------------------------------------------------------------- |
| 5 rows of 10 hues | `COLOR_PALETTE_HUES` has 5 shade rows, each with 10 hue columns |
| 5 grey values     | `COLOR_PALETTE_GREYS` has exactly 5 entries                     |
| Valid hex colors  | All palette entries match the `#rrggbb` hex format              |
