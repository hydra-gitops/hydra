# Data Model and Grouping

This page documents the graph data structures, grouping configuration, reference semantics, and namespace resolution rules used before rendering.

Back to [Graph Layout Architecture](../layout.md).

## Overview

Hydra UI renders Kubernetes objects as a directed graph, grouped by namespace and entity group. Dependencies (references) determine the reading direction: left to right.

**Single source of truth:** All relevant state (positions, sizes, group membership) lives in the React model. Cytoscape and the DOM are pure rendering targets: they read from the model but never store their own state.

**Processing order:** After loading the data, filters are applied first (in the order they are defined) to reduce the set of visible entities. Then the remaining entities are grouped into a tree hierarchy using the configured grouping fields. At least one grouping level must be defined.

## Example Model (YAML)

```yaml
filters:
  - field: namespace
    values:
      - default

grouping:
  - namespace
  - entityGroup

groups:
  - id: group-0
    name: nginx (Deployment)
    ids:
      - apps/v1/Deployment/default/nginx
      - v1/Service/default/nginx-svc
      - v1/ConfigMap/default/nginx-config

entities:
  - id: apps/v1/Deployment/default/nginx
  - id: v1/Service/default/nginx-svc
  - id: v1/ConfigMap/default/nginx-config
  - id: v1/ConfigMap/kube-system/coredns-config

references:
  - from: v1/Service/default/nginx-svc
    to: apps/v1/Deployment/default/nginx
    labels:
      - selects
  - from: apps/v1/Deployment/default/nginx
    to: v1/ConfigMap/default/nginx-config
    labels:
      - mounts
```text

- **filters**: applied first; restricts the view to the `default` namespace, so `coredns-config` from `kube-system` is hidden
- **grouping**: defines which entity fields are used for nesting the tree hierarchy (here: first by `namespace`, then by `entityGroup`). At least one grouping level is required.
- **groups**: predefined groups from the data source that bundle related resources (for example a Deployment with its Service, ConfigMap, and other owned resources). Each group has an `id` that becomes the value of the `entityGroup` field on its member entities, so groups can be used as a grouping level via `entityGroup`.
- **entities**: flat list of all Kubernetes objects across namespaces (`default` and `kube-system`)
- **references**: dependencies between entities (edges in the graph)
- **reachability**: pre-computed transitive reachability map (built automatically from entities + references on load, not part of the YAML input). Used for `includeRefs` filter expansion and edge highlighting.

Resulting tree for the filtered entities:

```text
root
  └── :namespace:default
        └── :namespace:default:entityGroup:nginx (Deployment)
              ├── apps/v1/Deployment/default/nginx
              ├── v1/Service/default/nginx-svc
              └── v1/ConfigMap/default/nginx-config
```

## Data Model

### Filters

```typescript
filters: Array<{
  id: string;
  field: FilterField;
  values: string[];
  includeRefs: boolean;
}>;
```text

Applied first after loading data. Each filter targets an entity field: `namespace`, `kind`, `entityGroup`, `apiVersion`, `group`, `gvk`, or `name`. Multiple filters are applied sequentially in the defined order (AND logic). Setting `includeRefs: true` expands the result to include all transitively reachable entities via references. Filters are persisted as part of `HydraUiState` (see [state.md](../state.md)).

### Grouping and Tree

```typescript
type LayoutDirection = "horizontal" | "vertical";

type GroupDisplayFieldEntry =
  | { type: "field"; field: LeafField } // entity field value (same for all entities in the group)
  | { type: "label" } // the group's own display label
  | { type: "text"; text: string } // literal text
  | { type: "itemCount" }; // number of entities in the group

type GroupingLevelDisplay = {
  header: GroupDisplayFieldEntry[]; // fields composing the group header label
  description: GroupDisplayFieldEntry[]; // fields composing the group description
  tooltip: GroupDisplayFieldEntry[]; // fields composing the group tooltip
};

type GraphGroupingConfig = {
  topLevelLayout: LayoutDirection; // how top-level groups are arranged (non-deletable)
  levels: Array<{
    field: GroupingField; // entity field used for nesting
    layout: LayoutDirection; // how child groups/entities at this level are arranged
    display: GroupingLevelDisplay; // header/description/tooltip for groups at this level
  }>;
  nodeDisplay: GroupingLevelDisplay; // header/description/tooltip for entity nodes (leaves)
};

// ============================================================================
// Grouping Key definitions (custom named categories)
// ============================================================================

type GroupingKeyEntry = {
  key: string; // resolved output key (for example "true", "core")
  field: string; // source field for THIS entry (for example "group", "namespace", "gk:OtherKey")
  values: string[]; // field values that map to this key
  pathLevel?: number; // directory depth (only for field === "templatePath")
};

type GroupingKeyDefinition = {
  name: string; // unique name (for example "Type")
  entries: GroupingKeyEntry[]; // key/value pairs — first matching entry wins; each entry has its own field
  fallbackKey: string; // key for entities that match no entry (for example "false")
};
```

A configurable grouping configuration determines the nesting of the group hierarchy, the **layout direction**, and the **display fields** (header, description, tooltip) at each level. Color configuration is managed separately (see [Entity Cloning and Colors](cloning-and-colors.md#color-configuration)). Custom grouping keys are configured on a dedicated **"Grouping Keys"** tab (see [grouping-keys.md](../grouping-keys.md)).

**Persistence:** `GraphGroupingConfig`, `groupingKeys`, `colorRules`, and `cloneRules` are each persisted as separate fields in `HydraUiState`. See [state.md](../state.md) for persistence details (YAML serialization, default stripping, localStorage).

In the UI, the Graph settings section contains four sub-sections: **Grouping**, **Clone**, **Color**, and **Labels**. Each sub-section has a `[+]` icon next to its header to add new entries. The **Clone** section is placed directly after **Grouping** because the `per` field must reference an active grouping level. The **Labels** section at the bottom configures display settings (Header, Description, Tooltip) for each grouping level and for entity nodes.

The **Grouping** sub-section entries appear in a single list:

- **Top Level** (`topLevelLayout`): The first entry in the list. Controls how the root's children (first grouping level groups) are arranged. Cannot be moved (up/down always disabled) or deleted. Shown as a static label. Default: `"horizontal"` (left-to-right).
- **Grouping levels** (`levels`): Each subsequent entry has a `field` dropdown (changeable: shows all unused fields as alternatives) and a `layout` direction for arranging its children. Changing the field resets display settings (in the Labels section) to the defaults for the new field. Levels can be added (via `[+]` on the Grouping header), removed, and reordered. At least one level must be defined.

The **Labels** sub-section (at the bottom of Graph settings) configures display settings for each grouping level and for entity nodes. For each active grouping level, it shows the level name followed by **Header**, **Description**, and **Tooltip** field lists. At the bottom, **Node** (`nodeDisplay`) configures how individual entity nodes (leaves) are displayed. Defaults: Header = `[Field Kind]`, Description = `[Field Name]`, Tooltip = `[Field ID]`.

Each field list is composed of entries that can be:

- **Label**: the group's own display label (derived from the grouping field value)
- **Field**: an entity field value (`Name`, `ID`, `GVK`, `GVKN`, `Group`, `Version`, `Kind`, `Namespace`)
- **Text**: a literal string (for example `"("`, `" Items)"`)
- **Item Count**: the number of entities in the group

Entries can be added, removed, and reordered (up/down). All three lists can be empty.

**Defaults per grouping field:**

| Field         | Header    | Description                                | Tooltip        |
| ------------- | --------- | ------------------------------------------ | -------------- |
| All fields    | `[Label]` | `[Text "("] [Item Count] [Text " Items)"]` | `[Label]`      |
| `gvk`, `gvkn` | `[Label]` | `[Text "("] [Item Count] [Text " Items)"]` | `[Field Kind]` |
| `nscs`        | `[Label]` | `[Text "("] [Item Count] [Text " Items)"]` | `[Label]`      |

Available fields (order defines nesting depth):

| Field                           | Value (`getFieldValue`)                                               | Label (`getFieldLabel`)                  | Description                                                                                                                                                                                                                                                                          |
| ------------------------------- | --------------------------------------------------------------------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `namespace`                     | `entity.namespace` (empty string `""` for cluster-scoped)             | `"Namespace <ns>"` or `"Cluster Scoped"` | Kubernetes namespace. Cluster-scoped resources (`namespace = ""`) are grouped under "Cluster Scoped".                                                                                                                                                                                |
| `nscs`                          | `nscsMap.get(entity.id)` (falls back to `entity.namespace`)           | `"Namespace <ns>"` or `"Cluster Scoped"` | Like `namespace`, but resolves cluster-scoped resources into a namespace using three strategies (see [NSCS Grouping Logic](#nscs-grouping-logic) below).                                                                                                                             |
| `entityGroup`                   | `entityGroupMap.get(entity.id)` or `"(ungrouped)"`                    | Same as value                            | Logical group from the data source. Maps entity IDs to group names via `buildEntityGroupMap`. During cloning, clone-specific placement overrides are applied via `entityGroupOverrides` from `cloneEntities()` (`src/cloneLogic.ts`). Entities not in any group get `"(ungrouped)"`. |
| `kind`                          | `entity.kind`                                                         | Same as value                            | Kubernetes resource kind (for example `Deployment`, `Service`, `ConfigMap`).                                                                                                                                                                                                         |
| `version`                       | `entity.version`                                                      | Same as value                            | API version (for example `v1`, `v1beta1`).                                                                                                                                                                                                                                           |
| `group`                         | `entity.group`                                                        | `entity.group` or `"(core)"` for empty   | API group (for example `apps`, `rbac.authorization.k8s.io`). Core API resources have an empty group, displayed as `"(core)"`.                                                                                                                                                        |
| `apiVersion`                    | `entity.apiVersion` (`group/version` or just `version`)               | Same as value                            | Full API group + version (for example `apps/v1`, `v1`).                                                                                                                                                                                                                              |
| `gvk`                           | `entity.gvk` (`group/version/kind` or `version/kind`)                 | Same as value                            | Group + Version + Kind (for example `apps/v1/Deployment`).                                                                                                                                                                                                                           |
| `gvkn`                          | `entity.gvk + "/" + entity.namespace` (or just `gvk` if no namespace) | Same as value                            | GVK + Namespace (for example `apps/v1/Deployment/default`).                                                                                                                                                                                                                          |
| `name`                          | `entity.name`                                                         | Same as value                            | Kubernetes resource name (for example `nginx`, `coredns`).                                                                                                                                                                                                                           |
| `appId`                         | `entity.appIds` (joined)                                              | Same as value                            | Hydra app IDs that produced this entity (for example `prod.myapp`).                                                                                                                                                                                                                  |
| `clusterName`                   | First dot-segment of `entity.appIds`                                  | Same as value                            | Cluster name derived from the first appId segment (for example `prod`).                                                                                                                                                                                                              |
| `rootAppId`                     | First two dot-segments of `entity.appIds`                             | Same as value                            | Root app identifier (for example `prod.myapp`).                                                                                                                                                                                                                                      |
| `rootAppName`                   | Second dot-segment of `entity.appIds`                                 | Same as value                            | Root app name (for example `myapp`).                                                                                                                                                                                                                                                 |
| `childAppId`                    | Full appId (3 segments)                                               | Same as value                            | Full child app identifier (for example `prod.myapp.child`).                                                                                                                                                                                                                          |
| `childAppName`                  | Third dot-segment of `entity.appIds`                                  | Same as value                            | Child app name (for example `child`).                                                                                                                                                                                                                                                |
| _(dynamic `gk:KeyName` fields)_ | `groupingKeyMaps.get(keyName)?.get(entity.id)`                        | Same as value                            | Resolved key from a `GroupingKeyDefinition`. Each key appears as its own column. See [grouping-keys.md](../grouping-keys.md).                                                                                                                                                        |

#### Grouping Keys

Grouping keys are configured on a dedicated **"Grouping Keys"** tab in the Settings Page (not inside Graph settings). See [grouping-keys.md](../grouping-keys.md) for the full data model, resolution algorithm, and examples.

Each key independently evaluates every entity, producing a resolved key string (for example `"true"`, `"false"`). The resolved value is available as a dynamic column (`gk:<keyName>`) in the entity list and as a field for color rules.

**Default key:** `"Type"` classifies entities by API group into `"Kubernetes"`, `"ArgoCD"`, `"Cluster Infra"`, `"Demo Infra"`, or `"other"`.

**Resolution:** `buildGroupingKeyMaps()` in `src/groupingKeyLogic.ts`. Returns `Map<keyName, Map<entityId, resolvedKey>>`.

**Default color rules:** The default state includes a color rule for cluster-scoped groups: `{ field: "namespace", value: "", target: "group", mode: "color", color: "#616161" }` (dark grey).

**Persistence:** All keys are stored in `HydraUiState.groupingKeys`. When the state matches the default, the field is stripped during serialization.

**Implementation:** `src/groupingKeyLogic.ts` (`buildGroupingKeyMaps`, `resolveTemplatePathValue`, `allGroupingKeyNames`). Tests in `src/__tests__/groupingKeyLogic.test.ts`.

### Groups

```typescript
groups: Array<{ id: string; name: string; ids: entityId[] }>;
```text

Predefined groups from the data source bundle related resources together (for example a Deployment with its Service, ConfigMap, and all other owned resources). The UI resolves an entity's `entityGroup` via `buildEntityGroupMap()` (`entity ID -> group name`). This allows `groups` to be used as a grouping level via the `entityGroup` field in the `grouping` configuration.

- **If an entity appears in multiple groups**, the effective `entityGroup` value is the one from the last mapped group entry (map overwrite behavior).
- **Entities not in any group** receive the `entityGroup` value `"(ungrouped)"`.

**Difference between `grouping` and `groups`:** `grouping` defines **which fields** to nest by (for example `["namespace", "entityGroup"]`). `groups` defines **concrete, predefined sets** of entities that share the same `entityGroup` value.

**How groups are computed:** See [grouping.md](../../../shared/details/grouping.md) for the full algorithm (seed-based absorption, workload merging, standalone SA seeds, union-find).

### Entities

```typescript
entities: Map<entityId, HydraEntity>;
```

All Kubernetes objects are stored flat and centrally. Other structures (groups, trees) reference entities only by their IDs. The entity ID encodes all metadata (for example `apps/v1/Deployment/default/nginx`).

### References

```typescript
references: Array<{
  from: entityId;
  to: entityId;
  labels: string[];
  reverse: boolean;
}>;
```text

Each reference describes a dependency between two entities (for example Service selects Deployment). Edges between entities in collapsed groups are rerouted to the group node.

When `reverse: true`, the `from` and `to` fields are **swapped at parse time** by `parseHydraYaml()` so that the data model always contains the **visual traversal direction**. The `reverse` flag is preserved as metadata. This means the layout algorithm, reachability computation, and all downstream consumers see the swapped direction, which is necessary for correct left-to-right spatial ordering and transitive reachability across the visual chain.

**Why the swap is needed:** The Go backend stores references in Kubernetes reference semantics (for example `from: RoleBinding, to: ServiceAccount` for a subject binding). But the visual chain requires the opposite traversal direction (`ServiceAccount -> RoleBinding`). Swapping at parse time ensures the layout algorithm places nodes in the correct visual order and the BFS reachability traverses the full visual chain.

#### Example: Deployment -> ServiceAccount -> Role chain

A Deployment uses a ServiceAccount (SA), and a RoleBinding (RB) binds that SA to a Role. In the YAML file (Kubernetes semantics), the RB references the SA:

```yaml
# In .hydra.yaml (Go backend output):
references:
  - from: Deployment
    to: SA
    labels: [uses]
    reverse: false # kept as-is

  - from: RB
    to: SA
    labels: [binds]
    reverse: true # swapped at parse time -> from: SA, to: RB

  - from: RB
    to: Role
    labels: [grants]
    reverse: false # kept as-is
```

After parsing, the data model contains:

```text
Deployment -> SA    (from: Deployment, to: SA, reverse: false)
SA -> RB            (from: SA, to: RB, reverse: true)    <- swapped
RB -> Role          (from: RB, to: Role, reverse: false)
```text

The layout algorithm sees the chain `Deployment -> SA -> RB -> Role` and places nodes left to right in this order. The `reverse: true` flag is passed to the rendering layer, which draws the arrowhead on the opposite end (so the arrow visually points `SA <- RB` while the spatial arrangement remains left to right).

**Summary:** `from -> to` in the data model is always the **visual traversal direction**, not the Kubernetes reference direction. The `reverse` flag indicates that the original YAML had the opposite direction.

### Reachability

```typescript
type ReachabilityLevel = Map<targetId, HydraReference[]>;

type ReachabilityInfo = {
  outgoing: Map<level, ReachabilityLevel>; // level -> targetId -> refs
  incoming: Map<level, ReachabilityLevel>; // level -> targetId -> refs
};

type ReachabilityMap = Map<entityId, ReachabilityInfo>;
```

The reachability map is a pre-computed, per-entity index of all transitively reachable entities via references. It is built once when the data is loaded (`buildReachabilityMap()` in `parseHydra.ts`) and stored as part of `HydraData`.

**Structure:** For each entity, `outgoing` maps a BFS level (`1 = direct neighbor`, `2 = two hops`, and so on) to a `ReachabilityLevel`, which maps each reachable target entity ID to the list of `HydraReference` objects that lead to it. `incoming` is analogous but follows edges in reverse.

**Algorithm:** For each entity, a BFS traversal is performed on the reference graph (up to 10 levels). At each BFS level, all newly discovered target IDs and their connecting references are recorded.

```text
Example: A -> B -> C -> D

ReachabilityMap for A:
  outgoing:
    level 1: { B: [ref A->B] }
    level 2: { C: [ref B->C] }
    level 3: { D: [ref C->D] }
  incoming: (empty)

ReachabilityMap for D:
  outgoing: (empty)
  incoming:
    level 1: { C: [ref C->D] }
    level 2: { B: [ref B->C] }
    level 3: { A: [ref A->B] }
```text

**Usage:**

- **Filter expansion (`includeRefs`)**: When a filter has `includeRefs: true`, the filtered entity set is expanded to include all transitively reachable entities (both outgoing and incoming, all levels) via `expandWithRefs()` in `filterLogic.ts`.
- **Edge highlighting and distance badges**: When a node is selected in the graph, the reachability map is used to highlight all transitively reachable edges (both directions are rendered in red) and to show a **distance badge** on each reachable node. See [Algorithm and Rendering](algorithm-and-rendering.md#selection-highlighting-reachability).

### Expand/Collapse State

```typescript
expandedGroups: Set<groupKey>;
```

Determines which groups are expanded. Entities inside collapsed groups are not displayed individually: the group is rendered as a single node instead. All groups are expanded by default.

## Grouping Pipeline

The grouping pipeline transforms a flat list of entities into a nested tree hierarchy. It runs **after filtering** and **after entity replication** (if configured).

**Implementation:** `src/treeLogic.ts` (all functions), `src/parseHydra.ts` (`parseEntityId`).

### Step 1: Entity ID Parsing (`parseEntityId`)

Each entity ID encodes its Kubernetes metadata. `parseEntityId()` in `src/parseHydra.ts` splits the ID into its component fields:

```text
Format:  <group>/<version>/<kind>/<namespace>/<name>
         or <version>/<kind>/<namespace>/<name>  (for core API, group = "")

Examples:
  apps/v1/Deployment/default/nginx
    -> group="apps", version="v1", kind="Deployment", namespace="default", name="nginx"
    -> apiVersion="apps/v1", gvk="apps/v1/Deployment"

  v1/Service/default/nginx-svc
    -> group="", version="v1", kind="Service", namespace="default", name="nginx-svc"
    -> apiVersion="v1", gvk="v1/Service"

  rbac.authorization.k8s.io/v1/ClusterRoleBinding//my-binding
    -> group="rbac.authorization.k8s.io", version="v1", kind="ClusterRoleBinding",
      namespace="" (cluster-scoped), name="my-binding"
```text

**Cluster-scoped resources** have an empty namespace (`""`). The empty segment between the two slashes (`//`) in the ID indicates this.

### Step 2: Entity Group Map (`buildEntityGroupMap`)

Maps each entity ID to its entity group **name** (for display and grouping by `entityGroup`).

```typescript
buildEntityGroupMap(groups: HydraGroup[]): Map<entityId, groupName>
```

Iterates over all groups and maps each entity ID in `group.ids` to `group.name` (or `group.id` if name is empty). If an entity appears in multiple groups, the last one wins. Entities not in any group are not in the map and resolve to `"(ungrouped)"` when used in `getFieldValue`.

**Note on group names vs IDs:** Multiple groups can share the same **name** (for example several groups named `"Shared"`). The `entityGroupMap` uses names for display purposes. The `buildNscsMap` function uses unique group **IDs** to avoid incorrectly merging distinct groups (see [NSCS Grouping Logic](#nscs-grouping-logic)).

### Step 3: NSCS Map (`buildNscsMap`) — optional

Only computed when the `nscs` field is used in the grouping configuration. Maps each entity ID to its resolved namespace, which may differ from `entity.namespace` for cluster-scoped resources.

See [NSCS Grouping Logic](#nscs-grouping-logic) for the full algorithm.

### Step 4: Field Value Resolution (`getFieldValue`, `getFieldLabel`)

Two functions resolve the value and display label for each grouping field:

- **`getFieldValue(entity, field, entityGroupMap, nscsMap?)`**: returns the raw value used for grouping (for example `"default"`, `"apps/v1/Deployment"`, `""`). Entities with the same value end up in the same tree node.
- **`getFieldLabel(entity, field, entityGroupMap, nscsMap?)`**: returns the display label shown in the UI. For most fields this is the same as the value, but `namespace`/`nscs` format as `"Namespace default"` or `"Cluster Scoped"`, and `group` shows `"(core)"` for empty API groups.

### Step 5: Tree Building (`buildTree`)

Builds the nested tree hierarchy from entities and grouping fields.

```typescript
buildTree(entities, grouping, entityGroupMap, depth?, prefix?, nscsMap?): TreeNode
```text

**Algorithm (recursive, depth-first):**

1. Create a root `TreeNode` containing all entity IDs.
2. If `depth >= grouping.length`, return root as a leaf node (no further nesting).
3. Take the grouping field at `grouping[depth]`.
4. For each entity, compute `getFieldValue(entity, field, ...)` and bucket entities by that value.
5. For each bucket (unique field value):
   a. Create a child `TreeNode` with `key = "<prefix>:<field>:<value>"` and `label = getFieldLabel(...)`.
   b. Recurse with `depth + 1` to build sub-groups from the bucketed entities.
6. Attach all child nodes to the root's `children` map.

**Key properties:**

- **Deterministic:** Same input always produces the same tree (map insertion order = entity encounter order).
- **Tree keys are unique:** Composed from the full path of field/value pairs (for example `:namespace:default:entityGroup:my-app`).
- **entityIds is propagated:** Each node stores the IDs of all entities at or below it (not just direct children), enabling count display and efficient collapse/expand.

**Example:** With `grouping: ["namespace", "entityGroup"]` and 3 entities:

```text
Input:  Deployment/default/nginx  (entityGroup: "nginx (Deployment)")
        Service/default/nginx-svc (entityGroup: "nginx (Deployment)")
        ConfigMap/default/config  (entityGroup: "(ungrouped)")

Step depth=0, field=namespace:
  bucket "default" -> [nginx, nginx-svc, config]

  Step depth=1, field=entityGroup:
    bucket "nginx (Deployment)" -> [nginx, nginx-svc]
      depth=2 -> leaf node
    bucket "(ungrouped)" -> [config]
      depth=2 -> leaf node

Result:
  root
    └── :namespace:default (label: "Namespace default", 3 entities)
          ├── :namespace:default:entityGroup:nginx (Deployment) (2 entities)
          │     ├── Deployment/default/nginx
          │     └── Service/default/nginx-svc
          └── :namespace:default:entityGroup:(ungrouped) (1 entity)
                └── ConfigMap/default/config
```

## NSCS Grouping Logic

The `nscs` field ("Namespace + Cluster Scope") is a variant of `namespace` grouping that intelligently assigns cluster-scoped resources (entities with an empty namespace) to the namespace of their entity group peers or direct references.

**Important:** Group identity uses unique group IDs (not group names) so that multiple groups sharing the same name (for example `"Shared"`) are treated independently.

**Clone entity support:** Clone entities are not part of the original `groups[].ids` arrays. To ensure clones receive the same NSCS treatment as original entities, `buildNscsMap` uses `entityGroupMap` as a fallback: for any entity not found in the group ID lists, it looks up the entity's group name from `entityGroupMap`, maps it back to the corresponding group ID, and adds the clone to that group's peer set.

For **namespaced** entities: the value is the entity's namespace (same as `namespace`).

For **cluster-scoped** entities (`namespace === ""`), three strategies are tried in order:

### Strategy 1 – Group Peers (Single Namespace)

Look up the entity's **entity group** (by unique group ID). Collect the **distinct namespaces** of all namespaced peers in the same group. If there is **exactly one** unique namespace, assign the cluster-scoped entity to that namespace.

**Example:** An entity group `"my-app"` contains:

- `Deployment/default/my-app` (`namespace: "default"`)
- `Service/default/my-app-svc` (`namespace: "default"`)
- `ClusterRole//my-app-role` (`namespace: ""`)
- `ClusterRoleBinding//my-app-binding` (`namespace: ""`)

All namespaced peers are in `default`, so `ClusterRole` and `ClusterRoleBinding` are assigned to `default`.

### Strategy 2 – Direct References (Multi-Namespace Group)

When the group spans **multiple namespaces**, the function checks the **direct references** of the specific cluster-scoped entity (not the whole group). If all referenced namespaced entities share **exactly one** namespace, assign there.

This handles cases like a `ClusterRoleBinding` in a large `"Shared"` group (spanning many namespaces) that only references a `ServiceAccount` in one namespace.

**Example:** A large `"Shared"` group contains resources from `demo`, `monitoring`, and `ingress-nginx`. The `ClusterRoleBinding` `kube-prometheus-stack-prometheus` is in this group. Its direct references are:

- `ClusterRole//kube-prometheus-stack-prometheus` (cluster-scoped, ignored)
- `ServiceAccount/monitoring/kube-prometheus-stack-prometheus` (`namespace: "monitoring"`)

Even though the group spans multiple namespaces, this specific `ClusterRoleBinding`'s references all point to `monitoring`, so it is assigned to `monitoring`.

**When Strategy 2 does not resolve:** If the `ClusterRoleBinding`'s direct references span multiple namespaces (for example subjects in both `monitoring` and `demo`), or if all references point to cluster-scoped entities, it stays in `"Cluster Scoped"`.

### Strategy 3 – Group References (Cluster-Only Group)

When the group contains **no namespaced peers at all** (only cluster-scoped entities), the function checks **references of all entities in the group** to determine the namespace:

1. Collect all references where at least one endpoint (`from` or `to`) is an entity in the group.
2. From those references, collect the **distinct namespaces** of all referenced namespaced entities.
3. If there is **exactly one** unique namespace, assign all cluster-scoped entities in the group to that namespace.
4. Otherwise, keep as `""` (`Cluster Scoped`).

**Example:** An entity group `"kyverno-create-secret"` contains:

- `ClusterRole//kyverno-create-secret` (`namespace: ""`)
- `ClusterRoleBinding//kyverno-create-secret` (`namespace: ""`)

The `ClusterRoleBinding` has references (subjects) to:

- `ServiceAccount/kyverno/kyverno-admission-controller` (`namespace: "kyverno"`)
- `ServiceAccount/kyverno/kyverno-background-controller` (`namespace: "kyverno"`)

Since the group has no namespaced peers, Strategy 3 kicks in. All referenced namespaced entities are in `kyverno`, so both cluster-scoped entities are assigned to `kyverno`.

### Transitive Resolution (Iterative Convergence)

Strategies 2 and 3 are applied **iteratively** until no more changes occur. In each pass, entities resolved in previous passes are considered "namespaced" when checking references. This handles transitive chains where each link depends on the previous one being resolved first.

#### Example: CR <- CRB -> SA chain

```text
SA(monitoring/prometheus)  <- subject -  CRB(cluster-scoped)  - roleRef ->  CR(cluster-scoped)
```text

- **Pass 1:** `CRB` checks direct references: `SA` is in `monitoring`, so `CRB` resolves to `monitoring`.
- **Pass 2:** `CR` checks direct references: `CRB` is now `monitoring` (via effective namespace from pass 1), so `CR` resolves to `monitoring`.

**Longer chains** work the same way. An `AggregatedClusterRole` referencing the `ClusterRole` would resolve in pass 3 (after the `ClusterRole` was resolved in pass 2). The algorithm converges when no new resolutions occur, with a safety limit of 10 passes.

**Stability guarantee:** Strategy 1 always uses **original** entity namespaces (not resolved ones), so it remains stable across passes. Only Strategies 2 and 3 benefit from transitive resolution via `getEffectiveNs()`, which returns the resolved namespace from `nscsMap` if non-empty, otherwise the entity's original namespace.

**Non-propagation:** Entities whose direct references span **multiple** namespaces are never resolved (they stay cluster-scoped). This prevents incorrect propagation through ambiguous entities.

### No Resolution

If none of the strategies yields a unique namespace after convergence (no group, no references, or references parameter omitted), the entity remains as `""` (`Cluster Scoped`).

The tree is built dynamically by `buildTree()`, which iterates over all entities and sorts them into nested nodes based on each grouping field value.

```text
TreeNode
  ├── key: string                    (unique key, for example ":namespace:default:entityGroup:my-app")
  ├── label: string                  (display name, for example "default" or "my-app")
  ├── entityIds: entityId[]          (IDs of all entities at or below this node)
  └── children: Map<key, TreeNode>   (child groups)
```

**Example:** With `grouping: ["namespace", "entityGroup"]`:

```text
root
  ├── :namespace:default                              (label: "default")
  │     ├── :namespace:default:entityGroup:frontend   (label: "frontend")
  │     │     └── entityIds: [pod-1, svc-1, ...]
  │     └── :namespace:default:entityGroup:backend    (label: "backend")
  │           └── entityIds: [pod-2, svc-2, ...]
  └── :namespace:kube-system                          (label: "kube-system")
        └── :namespace:kube-system:entityGroup:core   (label: "core")
              └── entityIds: [pod-3, ...]
```text
