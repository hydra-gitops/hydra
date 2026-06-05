# Layout API Reference

This page lists the exported layout-related types, functions, and constants grouped by source module.

Back to [Graph Layout Architecture](../layout.md).

Comprehensive list of all exported functions, types, and constants per source module.

## model.ts — Core Types

| Export                   | Kind  | Description                                                                                                                                                                                                                                                                                        |
| ------------------------ | ----- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GroupingField`          | type  | Union of all field names usable for grouping (`"namespace"` \| `"nscs"` \| `"kind"` \| `"version"` \| `"group"` \| `"apiVersion"` \| `"gvk"` \| `"gvkn"` \| `"entityGroup"` \| `"name"` \| `"appId"` \| `"clusterName"` \| `"rootAppId"` \| `"rootAppName"` \| `"childAppId"` \| `"childAppName"`) |
| `RbacRule`               | type  | RBAC policy rule: `apiGroups`, `resources`, `verbs`, `resourceNames?`                                                                                                                                                                                                                              |
| `RbacRuleSource`         | type  | Identifies the source Role or ClusterRole of a rule: `entityId`, `entityName`, `entityKind`, `entityNamespace`                                                                                                                                                                                     |
| `RbacRuleWithSource`     | type  | `RbacRule` annotated with `source: RbacRuleSource` for the RBAC panel                                                                                                                                                                                                                              |
| `STANDARD_VERBS`         | const | Standard Kubernetes RBAC verbs used as table columns: `get`, `list`, `watch`, `create`, `update`, `patch`, `delete`, `deletecollection`                                                                                                                                                            |
| `RbacRoleVerbEntry`      | type  | Shared base: a Role entry with `source`, `verbs`, `resourceNames`                                                                                                                                                                                                                                  |
| `RbacDisplayScope`       | type  | Shared base: scope grouping with `namespace` and `isClusterScoped`                                                                                                                                                                                                                                 |
| `OutgoingRbacSource`     | type  | Outgoing: per-source verb contribution (`RbacRoleVerbEntry`)                                                                                                                                                                                                                                       |
| `OutgoingRbacScope`      | type  | Outgoing: scope with aggregated `verbs` and `sources: OutgoingRbacSource[]`                                                                                                                                                                                                                        |
| `OutgoingRbacEntry`      | type  | Outgoing: one unique `(apiGroup, resource)` with `scopes: OutgoingRbacScope[]`                                                                                                                                                                                                                     |
| `IncomingRbacRoleMatch`  | type  | Incoming: a Role or ClusterRole match with `connectedEntities: HydraEntity[]`                                                                                                                                                                                                                      |
| `IncomingRbacScope`      | type  | Incoming: scope containing `roles: IncomingRbacRoleMatch[]`                                                                                                                                                                                                                                        |
| `GvkOption`              | type  | Incoming: `GVK` filter dropdown option with `gvk`, `kind`, `count`                                                                                                                                                                                                                                 |
| `IncomingRbacResult`     | type  | Incoming: complete analysis result with `scopes`, `allVerbs`, `gvkOptions`, target metadata                                                                                                                                                                                                        |
| `HydraEntity`            | type  | Parsed Kubernetes entity: `id`, `group`, `version`, `apiVersion`, `kind`, `namespace`, `name`, `gvk`, `appIds`, `templatePath`, `templateIndex`, `tags`, `rbacRules`, `secretKeys`                                                                                                                 |
| `HydraReference`         | type  | Directed edge: `from`, `to`, `labels`, `reverse`                                                                                                                                                                                                                                                   |
| `HydraGroup`             | type  | Named group of entity IDs                                                                                                                                                                                                                                                                          |
| `ReachabilityEntry`      | type  | Target + refs pair for BFS results                                                                                                                                                                                                                                                                 |
| `ReachabilityLevel`      | type  | `Map<targetId, HydraReference[]>` per BFS level                                                                                                                                                                                                                                                    |
| `ReachabilityInfo`       | type  | Outgoing + incoming reachability maps per entity                                                                                                                                                                                                                                                   |
| `ReachabilityMap`        | type  | `Map<entityId, ReachabilityInfo>`                                                                                                                                                                                                                                                                  |
| `HydraData`              | type  | Combined parsed result: entities, references, groups, reachability                                                                                                                                                                                                                                 |
| `LayoutDirection`        | type  | `"horizontal"` \| `"vertical"`                                                                                                                                                                                                                                                                     |
| `LeafFieldType`          | type  | Fields for entity display: `"name"` \| `"id"` \| ... \| `"tags"`                                                                                                                                                                                                                                   |
| `GroupDisplayFieldEntry` | type  | Single display entry: `field`, `label`, `text`, or `itemCount`                                                                                                                                                                                                                                     |
| `GroupingLevelDisplay`   | type  | `header + description + tooltip` arrays of display entries                                                                                                                                                                                                                                         |
| `ColorRuleTarget`        | type  | `"group"` \| `"node"` \| `"all"`                                                                                                                                                                                                                                                                   |
| `ColorRuleMode`          | type  | `"unchanged"` \| `"color"` \| `"auto"`                                                                                                                                                                                                                                                             |
| `GraphColorRule`         | type  | Color rule: `field` (`GroupingField`), `value`, `target`, `mode`, `color?`                                                                                                                                                                                                                         |
| `CloneRule`              | type  | Clone rule: `field` (`LeafFieldType`), `value`, `per` (`GroupingField`)                                                                                                                                                                                                                            |
| `ReplicateRule`          | type  | Deprecated alias for `CloneRule`                                                                                                                                                                                                                                                                   |
| `AutoCloneConfig`        | type  | Auto-clone config: `enabled`, `thresholdIn`, `thresholdOut`, `per` (`GroupingField`)                                                                                                                                                                                                               |
| `GroupingKeyDefinition`  | type  | User-defined grouping key: `name`, `entries` (each with own field), `fallbackKey`                                                                                                                                                                                                                  |
| `FilterField`            | type  | Filter field union: `"namespace"` \| `"kind"` \| ... \| `"entityGroup"`                                                                                                                                                                                                                            |
| `FilterRowState`         | type  | Filter row: `id`, `field`, `values`, `includeRefs`                                                                                                                                                                                                                                                 |

## parseHydra.ts — YAML Parsing

| Export                 | Kind     | Signature                                                        |
| ---------------------- | -------- | ---------------------------------------------------------------- |
| `getNamespaceLabel`    | function | `(namespace: string) -> string` — formats namespace for display  |
| `parseEntityId`        | function | `(id: string) -> HydraEntity` — splits entity ID into components |
| `parseHydraYaml`       | function | `(rawYaml: string) -> HydraData` — parses full YAML file         |
| `buildReachabilityMap` | function | `(entities, references) -> ReachabilityMap` — BFS reachability   |

## treeLogic.ts — Grouping & Tree Building

| Export                   | Kind     | Description                                                                                                     |
| ------------------------ | -------- | --------------------------------------------------------------------------------------------------------------- |
| `GroupingField`          | type     | Re-exported from `model.ts`                                                                                     |
| `LeafField`              | type     | Alias for `LeafFieldType` (backward compatibility)                                                              |
| `SearchField`            | type     | `"id"` \| `"name"` \| `"gvk"` \| `"gvkn"` \| `"path"` \| `"leafName"` \| `"leafDescription"` \| `"entityGroup"` |
| `TreeNode`               | type     | Tree node: `key`, `label`, `entityIds`, `children` (`Map<string, TreeNode>`)                                    |
| `SerializedTreeNode`     | type     | JSON-safe tree node for golden-file tests                                                                       |
| `CollapsedGroupResult`   | type     | Visible entities + collapsed groups                                                                             |
| `GroupInfo`              | type     | Group metadata: `label`, `entityIds`, `isExpanded`, `parentGroupKey`, `directChildEntityIds`                    |
| `AllGroupsResult`        | type     | `Map<string, GroupInfo>`                                                                                        |
| `GROUPING_LABELS`        | const    | `Record<GroupingField, string>` — display labels                                                                |
| `GROUPING_DESCRIPTIONS`  | const    | `Record<GroupingField, string>` — field descriptions                                                            |
| `ALL_FIELDS`             | const    | All available `GroupingField` values                                                                            |
| `buildEntityGroupMap`    | function | `(groups) -> Map<entityId, groupName>`                                                                          |
| `buildNscsMap`           | function | `(entities, entityGroupMap, groups, references?) -> Map<entityId, nscsValue>`                                   |
| `getFieldValue`          | function | `(entity, field, entityGroupMap, nscsMap?) -> string` — raw value for grouping                                  |
| `getFieldLabel`          | function | `(entity, field, entityGroupMap, nscsMap?) -> string` — display label                                           |
| `getLeafFieldValue`      | function | `(entity, field: LeafField) -> string` — leaf field value                                                       |
| `fuzzyMatch`             | function | `(text, pattern) -> boolean` — fuzzy substring matching                                                         |
| `getEntityPath`          | function | `(entity, grouping, entityGroupMap, nscsMap?) -> string` — full path string                                     |
| `buildTree`              | function | `(entities, grouping, entityGroupMap, depth?, prefix?, nscsMap?) -> TreeNode`                                   |
| `serializeTree`          | function | `(node, includeEntityNames?) -> SerializedTreeNode`                                                             |
| `getAllGroups`           | function | `(tree, expandedItems) -> AllGroupsResult` — all groups with metadata                                           |
| `getCollapsedGroups`     | function | `(tree, expandedItems, entityMap) -> CollapsedGroupResult`                                                      |
| `filterEntitiesBySearch` | function | `(entities, searchText, grouping, ...) -> HydraEntity[]` — search-based filtering                               |

## filterLogic.ts — Filter Application

| Export                | Kind     | Signature                                                                                            |
| --------------------- | -------- | ---------------------------------------------------------------------------------------------------- |
| `getEntityFieldValue` | function | `(entity, field: FilterField) -> string` — entity field value for filtering                          |
| `expandWithRefs`      | function | `(entityIds, reachability) -> Set<string>` — expand with transitive refs                             |
| `applyFilters`        | function | `(entities, filters, groups, reachability) -> HydraEntity[]` — sequential AND logic in defined order |

## colorLogic.ts — Color Resolution

| Export                  | Kind     | Signature                                                   |
| ----------------------- | -------- | ----------------------------------------------------------- | ----------------------------- |
| `COLOR_PALETTE_HUES`    | const    | `string[][]` — `5 x 10` Material Design palette             |
| `COLOR_PALETTE_GREYS`   | const    | `string[]` — 5 grey values                                  |
| `autoColor`             | function | `(value: string) -> string` — deterministic color from hash |
| `resolveColorFromRules` | function | `(rules, elementType, fieldValueGetter) -> string           | undefined` — first match wins |

## cloneLogic.ts — Entity Cloning

| Export                   | Kind     | Signature                                                                                                       |
| ------------------------ | -------- | --------------------------------------------------------------------------------------------------------------- |
| `CloneResult`            | type     | Entities, references, `cloneIds`, `entityGroupOverrides`, `nscsOverrides`                                       |
| `EdgeCounts`             | type     | Incoming + outgoing edge counts per entity                                                                      |
| `buildCloneId`           | function | `(perValue, originalId) -> string` — creates clone entity ID                                                    |
| `isCloneId`              | function | `(id) -> boolean` — checks whether an ID is a clone                                                             |
| `getOriginalId`          | function | `(cloneId) -> string` — extracts the original ID from a clone ID                                                |
| `cloneEntities`          | function | `(entities, references, rules, groups, entityGroupMap, nscsMap) -> CloneResult`                                 |
| `buildCloneSiblingMap`   | function | `(cloneIds) -> Map<string, Set<string>>` — maps clones of the same entity                                       |
| `expandCloneSelection`   | function | `(ids, siblingMap) -> Set<string>` — expands selection to sibling clones                                        |
| `buildEdgeCounts`        | function | `(references) -> EdgeCounts` — counts non-reverse edges per entity                                              |
| `buildAutoCloneRules`    | function | `(edgeCounts, thresholdIn, thresholdOut, per) -> CloneRule[]` — rules for high-fanout entities (`>= threshold`) |
| `getEffectiveCloneRules` | function | `(autoClone, edgeCounts, manualRules) -> CloneRule[]` — auto + manual combined                                  |

## groupingKeyLogic.ts — Custom Grouping Keys

| Export                     | Kind     | Signature                                                                                        |
| -------------------------- | -------- | ------------------------------------------------------------------------------------------------ |
| `buildGroupingKeyMaps`     | function | `(entities, groupingKeys, entityGroupMap, nscsMap?) -> Map<keyName, Map<entityId, resolvedKey>>` |
| `resolveTemplatePathValue` | function | `(templatePath, pathLevel, keyName) -> string` — path-based key resolution                       |
| `allGroupingKeyNames`      | function | `(keys) -> string[]` — extracts all key names                                                    |

## graphModel.ts — Graph Node/Edge Computation

| Export | Kind | Signature |
| --- | --- | --- |
| `GROUP_LABEL_HEIGHT` | const | `60` — height reserved for group label header |
| `NodeGeometry` | type | `x`, `y`, `width`, `height` |
| `GraphState` | type | `expandedGroups`, `selectedNodeIds`, `nodeGeometry`, `grouping` |
| `computeVisibleEntities` | function | `(tree, expandedGroups, entityMap) -> { visibleEntities, collapsedGroups }` |
| `computeGraphNodes` | function | `(visibleEntities, collapsedGroups, expandedGroups, nodeGeometry, tree, ...) -> GraphNode[]` |
| `computeGraphEdges` | function | `(references, visibleEntities, collapsedGroups) -> GraphEdge[]` — ref tags such as `optional:ref` are passed through to edges |
| `getAllGroupKeys` | function | `(tree) -> string[]` — all group keys in the tree |

## layoutLogic.ts — Layout Computation

| Export                   | Kind     | Signature                                                                                                 |
| ------------------------ | -------- | --------------------------------------------------------------------------------------------------------- |
| `LayoutNode`             | type     | `id`, `width`, `height`                                                                                   |
| `LayoutEdge`             | type     | `from`, `to`                                                                                              |
| `computeEntityNodeSize`  | function | `(name, kind, tags?) -> { width, height }` — deterministic size formula                                   |
| `computeNodeSizes`       | function | `(entities) -> Map<entityId, { width, height }>` — sizes for all entities                                 |
| `computeNodeLayout`      | function | `(nodes, edges, options?) -> Map<id, { x, y }>` — Sugiyama-style layout                                   |
| `computeRecursiveLayout` | function | `(tree, references, nodeSizes, layoutDirections?) -> Map<entityId, NodeGeometry>` — full recursive layout |

## filesTreeLogic.ts — Files Tree

| Export                | Kind     | Signature                                                                                               |
| --------------------- | -------- | ------------------------------------------------------------------------------------------------------- |
| `FilesTreeNodeType`   | type     | `"root"` \| `"cluster"` \| `"rootApp"` \| `"childApp"` \| `"directory"` \| `"file"` \| `"entity"`       |
| `FilesTreeFilter`     | type     | `{ field: string; value: string }`                                                                      |
| `FilesTreeNode`       | type     | `name`, `type`, `entityCount`, `children`, `entityId?`, `entityLabel?`, `filterInfo: FilesTreeFilter[]` |
| `buildFilesTree`      | function | `(entities) -> FilesTreeNode` — builds cluster/app/directory/file tree from `appIds` and `templatePath` |
| `getEntityIdsForNode` | function | `(node) -> string[]` — recursive entity ID collection                                                   |
