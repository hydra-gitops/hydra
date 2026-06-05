# Entity Cloning and Colors

This page documents how Hydra UI colors graph elements and duplicates heavily connected entities to keep edges local and readable.

Back to [Graph Layout Architecture](../layout.md).

## Color Configuration

Color rules are managed as a **separate flat list** (`colorRules`), decoupled from the grouping hierarchy. This allows defining color rules independently: a color rule can target any grouping key, regardless of which fields are active in the grouping configuration.

**Data model:**

```typescript
type ColorRuleTarget = "group" | "node" | "all";
type ColorRuleMode = "unchanged" | "color" | "auto";

type GraphColorRule = {
  field: GroupingField | `gk:${string}`; // grouping field or dynamic grouping key (for example "gk:Type")
  value: string; // value to match (for example "default", "kube-system", "Pod")
  target: ColorRuleTarget; // apply to group boxes, entity nodes, or both
  mode: ColorRuleMode; // unchanged / color / auto
  color?: string; // hex color, only used when mode === "color"
};
```text

Each color rule consists of:

| Component  | Description                                                                                                                                                                                                         |
| ---------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Field**  | Grouping key to filter by: a built-in `GroupingField` (`namespace`, `entityGroup`, `kind`, `apiVersion`, `group`, `gvk`, `gvkn`, `nscs`, `name`) or a dynamic grouping key (`gk:<keyName>`, for example `gk:Type`). |
| **Value**  | The specific value to match. The UI shows a dropdown populated with all distinct values of the selected field from the currently filtered entities.                                                                 |
| **Target** | What to color: `group` (group box border + header background), `node` (entity node background), or `all` (both).                                                                                                    |
| **Mode**   | How to determine the color (see table below).                                                                                                                                                                       |
| **Color**  | Hex color, only used when mode is `color`.                                                                                                                                                                          |

**Color modes:**

| Mode        | Behavior                                                                                                                                                                                                  |
| ----------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `unchanged` | No color change: the element keeps its default color. (Previously called `none`.)                                                                                                                         |
| `auto`      | Automatic color from a deterministic palette. The color is derived by hashing the matched value, so the same value always produces the same color.                                                        |
| `color`     | A manually chosen color. Opens a **color picker grid** (Material Design palette: 10 hues x 5 shades + 5 greys) with a **Custom** option for entering an arbitrary hex value. (Previously called `fixed`.) |

**Rule evaluation:** Rules are evaluated **top-down**: first matching rule wins. A rule matches an element when:

1. The element's field value (from the entity or group's representative entity) equals the rule's `value`.
2. The element type matches the rule's `target` (`group` for group boxes, `node` for entity nodes, or `all` for both).

If no rule matches, the element keeps its default color (equivalent to `unchanged`).

**For group boxes:** The value for a group is determined by looking at the group's representative entity. Since all entities in a group share the same value for any ancestor grouping field, ancestor fields are always unambiguous. The color is applied to the **group box** (border and header background).

**For entity nodes:** The entity's own field value is used for matching. The color is applied to the **entity node background**.

**Color Picker (for `color` mode):**

The color picker popup contains a Material Design color grid:

- **10 hues** (Red, Orange, Yellow, Lime, Green, Teal, Cyan, Blue, Indigo, Purple) x **5 shades** (light to dark)
- **5 grey values** (Black, Dark Grey, Grey, Light Grey, White)
- **Custom** option: text input for an arbitrary hex color value (`#rrggbb`)

Clicking a grid color immediately applies it. The Custom input requires confirmation.

**Defaults:** One color rule is defined by default: `{ field: "namespace", value: "", target: "group", mode: "color", color: "#616161" }`, so cluster-scoped entity groups are colored dark grey.

## Entity Cloning

Entities that are heavily referenced from many groups (for example CRDs and shared ConfigMaps) create long edges crossing the entire graph. **Cloning** solves this by inserting copies of such entities into every group that references them, so edges become local.

The key principle is: **clone into the referencing group**. When an entity is referenced by entities in multiple groups (for example a ConfigMap in `"Shared"` referenced by `"kafka-producer"` and `"kafka-consumer"`), a clone is placed in each referencing group. The original entity is removed.

```typescript
type CloneRule = {
  field: LeafFieldType;    // entity field to match (kind, gvk, name, ...)
  value: string;           // value to match (for example "CustomResourceDefinition")
  per: GroupingField;      // determines clone placement and target discovery strategy
};

// In HydraUiState (separate from GraphGroupingConfig):
cloneRules: CloneRule[];   // default: []
```

The `per` field determines **where clones are placed** in the tree hierarchy and **how clone targets are discovered**. Only grouping levels that are currently active in the grouping configuration can be selected for `per`.

**Processing:** Cloning runs **after filtering and grouping computation** (`entityGroupMap`, `nscsMap`) but **before tree building**:

1. For each rule, find all entities where `entity[rule.field] === rule.value` ("matched entities").
2. Only entities that are in an **entity group** are considered (groups are the conceptual basis for cloning).
3. Determine **clone targets** (`per` values) from **referencing entities**; the strategy depends on `per`:
   - **`per=entityGroup`**: use referencing entities' **`(entityGroup, nscs)`** compound pairs. Each unique combination produces a clone. This ensures clones land in the correct group and the correct `nscs` branch, which is essential for cluster-scoped entities like CRDs.
   - **`per=nscs` / `per=namespace`**: use referencing entities' **`nscs`/`namespace`** values. This discovers which namespaces have references to the matched entity.
4. Remove the original entity.
5. Create a clone for each unique `per` value. Clone IDs have the format `clone:<perValue>:<originalId>`.
6. Rewire all references: edges pointing to or from the original are redirected to the clone whose `per` value matches the other endpoint's `per`-field value.

**Override behavior by `per` field:**

- **`per=entityGroup`**: per-values are compound keys `entityGroup\tnscs`. A clone gets `entityGroupOverrides[clone] = entityGroup` and `nscsOverrides[clone] = nscs`, and `clone.namespace = nscs`. This places the clone in the correct group and the correct `nscs` branch.
- **`per=nscs` / `per=namespace`**: the clone is placed **ungrouped** at the target namespace level. Sets `nscsOverrides[clone] = perValue` and `clone.namespace = perValue`. No `entityGroup` override is applied, so the clone is not assigned to any entity group within the namespace.

**Edge case: ungrouped entities:** If a matched entity is not in any entity group, no cloning occurs. The entity is kept as-is.

**Edge case: no clone targets:** If no referencing entities exist (for any `per` mode), no clone targets are found. The entity is kept as-is.

**Edge case: same-group, same-`nscs`:** If all referencing entities are in the same `(entityGroup, nscs)` combination, `per=entityGroup` produces exactly one clone. If they are in the same group but different namespaces, one clone per namespace is created (all in the same `entityGroup` but different `nscs` branches).

**Example** with grouping `[nscs, entityGroup]`:

ConfigMap `kafka-connection-params` is in group `"Shared"`, referenced by Deployments in three workload groups.

### per=entityGroup — clones in REFERENCING entity groups

Referencing entities produce compound pairs `{(kafka-producer, default), (kafka-consumer, default), (kafka-monitor, default)}` -> 3 clones. Each clone gets `entityGroupOverrides` (the referencing group name) and `nscsOverrides` (the referencing namespace). The original is removed from `"Shared"`.

```text
BEFORE:                                    AFTER (per=entityGroup):

┌─ nscs: default ──────────────────┐       ┌─ nscs: default ──────────────────┐
│  ┌─ Shared ──────────────────┐   │       │  ┌─ kafka-producer ──────────┐   │
│  │ CM/kafka-connection-params│◄┐ │       │  │ Deploy/kafka-producer    │   │
│  └───────────────────────────┘ │ │       │  │ CM (clone) ◄─────────────┘   │
│  ┌─ kafka-producer ──────────┐ │ │       │  └──────────────────────────┘   │
│  │ Deploy/kafka-producer   ──┘ │ │       │  ┌─ kafka-consumer ──────────┐   │
│  └───────────────────────────┘ │ │       │  │ Deploy/kafka-consumer    │   │
│  ┌─ kafka-consumer ──────────┐ │ │       │  │ CM (clone) ◄─────────────┘   │
│  │ Deploy/kafka-consumer   ──┘ │ │       │  └──────────────────────────┘   │
│  └───────────────────────────┘ │ │       │  ┌─ kafka-monitor ───────────┐   │
│  ┌─ kafka-monitor ───────────┐ │ │       │  │ Deploy/kafka-monitor     │   │
│  │ Deploy/kafka-monitor    ──┘ │ │       │  │ CM (clone) ◄─────────────┘   │
│  └───────────────────────────┘   │       │  └──────────────────────────┘   │
└──────────────────────────────────┘       └──────────────────────────────────┘
                                           (Shared group empty -> removed)
```text

### per=nscs — clones UNGROUPED at the namespace level

Referencing entities have `nscs` values `{"default"}` -> 1 clone (all in the same namespace). For cross-namespace scenarios (for example a CRD referenced from `demo` and `monitoring`), each namespace gets its own clone. The clone is ungrouped within the namespace.

```text
AFTER (per=nscs, cross-namespace example):

┌─ nscs: monitoring ───────────────────┐
│  CRD (clone, ungrouped) ◄──┐        │
│  ┌─ prometheusrules ───────┼──────┐  │
│  │ PrometheusRule/mon/alert ┘     │  │
│  │ PrometheusRule/mon/general     │  │
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘

┌─ nscs: demo ──────────────────────────┐
│  CRD (clone, ungrouped) ◄──┐        │
│  ┌─ prometheusrules ───────┼──────┐  │
│  │ PrometheusRule/demo/kafka ┘     │  │
│  │ PrometheusRule/demo/metrics     │  │
│  └────────────────────────────────┘  │
└──────────────────────────────────────┘
```

In both cases, the original CRD is removed and the `"Cluster Scoped"` branch becomes empty and disappears.

**Visual differentiation:** Clone nodes are rendered with a **dashed border** and a small clone badge (`⧉`) in the bottom-left corner (HTML label template, `title="Clone"`). The node's Cytoscape tooltip is appended with `" (clone)"`.

### Clone Page

When the **`⧉ Clone`** command is selected from the context menu of a clone node, a semi-transparent box appears around the clone's position in the graph. The clone page shows a **mini graph** containing the **aggregate neighborhood across all sibling clones** of the same original entity:

- The **selected clone node** in the center (dashed border, same color as in the main graph)
- **Copies of all directly connected nodes** from **all sibling clones** (not just the clicked one)
- **All edges** between those neighbors and the center node (deduplicated, same arrow direction as in the main graph)

The clone page uses a separate Cytoscape instance with a concentric layout (clone in center, neighbors around it).

**Interaction:**

- **Click a neighbor node** in the clone page: the main viewport animates to that node, the clone page closes, and the clicked node becomes selected.
- **Click the backdrop** (semi-transparent area outside the box): the clone page closes.
- **Press Escape**: the clone page closes.

```text
┌─────────────────────────────────────────────────┐
│                Graph Panel                       │
│                                                  │
│       ┌───────────────────────────┐              │
│       │  50% transparent backdrop │              │
│       │  ┌─────────────────────┐  │              │
│       │  │ Clone Node (center) │  │              │
│       │  │                     │  │              │
│       │  │  ┌────┐   ┌────┐   │  │              │
│       │  │  │ N1 │───│ N2 │   │  │              │
│       │  │  └────┘   └────┘   │  │              │
│       │  │       ↕             │  │              │
│       │  │  ┌────────────┐    │  │              │
│       │  │  │ Clone ⧉    │    │  │              │
│       │  │  └────────────┘    │  │              │
│       │  │       ↕             │  │              │
│       │  │  ┌────┐   ┌────┐   │  │              │
│       │  │  │ N3 │   │ N4 │   │  │              │
│       │  │  └────┘   └────┘   │  │              │
│       │  └─────────────────────┘  │              │
│       └───────────────────────────┘              │
│                                                  │
└─────────────────────────────────────────────────┘

N1..N4 = clickable neighbor copies -> click navigates to real node
```text

**Implementation:** `ClonePage` component in `src/components/ClonePage.tsx`. The clone page state (`clonePageNodeId`) is managed inside `GraphPanel`. When a clone is tapped, `GraphPanel` sets `clonePageNodeId` and passes `siblingIds` (from `cloneSiblingMap`) to the clone page. The `ClonePage` component collects edges from all siblings, rewrites sibling endpoints to the center node, deduplicates, and creates a mini Cytoscape instance with the aggregate neighborhood subgraph.

**Clone group selection:** All clones of the same original entity are treated as a group for interaction purposes:

- **Clicking** any clone (in graph or tree view) selects **all sibling clones** simultaneously.
- **Toggle selection** (`Ctrl`/`Cmd`+click) adds or removes all sibling clones at once.
- **Zoom to fit selection** encompasses all selected clones.
- **Reachability lookup** for clones uses the original entity's ID.

The sibling relationship is maintained via `buildCloneSiblingMap()`. `expandCloneSelection()` expands any selection set to include all sibling clones.

**Data flow integration:**

```text
HydraData
  ├── filters -> filteredEntities
  ├── -- Grouping Pipeline -----------------------------------
  ├── buildEntityGroupMap(groups) -> entityGroupMap
  ├── buildNscsMap(filteredEntities, entityGroupMap, groups, refs) -> nscsMap
  ├── -- Cloning ---------------------------------------------
  ├── cloneEntities(filteredEntities, refs, rules, groups, entityGroupMap, nscsMap)
  │     -> clonedEntities, clonedReferences, cloneIds
  │     -> entityGroupOverrides (per=entityGroup: clone -> referencing group name)
  │     -> nscsOverrides (per=entityGroup/nscs: clone -> target namespace)
  ├── entityGroupMap + entityGroupOverrides -> finalEntityGroupMap
  ├── nscsMap + nscsOverrides -> finalNscsMap
  ├── -- Tree Building ---------------------------------------
  ├── buildTree(clonedEntities, grouping, finalEntityGroupMap, finalNscsMap)
  │     -> TreeNode
  └── ... (layout, rendering as before)
```

**Persistence:** `cloneRules` is a separate top-level field in `HydraUiState` and persisted in the unified YAML state. The empty array (default) is stripped from YAML. See [state.md](../state.md).

**Implementation:** `cloneEntities()`, `buildCloneSiblingMap()`, `expandCloneSelection()` in `src/cloneLogic.ts`. Tests in `src/__tests__/cloneLogic.test.ts`.

## Auto-Clone (Threshold)

Entities that are referenced by, or reference, many other entities are natural candidates for cloning. The **Auto-Clone** feature automates clone rule creation based on configurable thresholds for incoming and outgoing edges.

**Concept:** When a data file is loaded, the system counts **incoming** and **outgoing** edges (excluding `ref.reverse === true`) for every entity. If the Auto-Clone feature is enabled and an entity's incoming count reaches `thresholdIn` or its outgoing count reaches `thresholdOut`, a clone rule is automatically generated.

**Edge counts:**

`buildEdgeCounts(references)` in `src/cloneLogic.ts` computes both counts:

```typescript
type EdgeCounts = { incoming: Map<string, number>; outgoing: Map<string, number> };
buildEdgeCounts(references: HydraReference[]): EdgeCounts
```text

- Iterates all references where `ref.reverse !== true`.
- `incoming`: counts per `ref.to` entity ID.
- `outgoing`: counts per `ref.from` entity ID.
- Computed once in `App.tsx` as a `useMemo` depending on `references`.

**Auto-Clone rule generation:**

`buildAutoCloneRules(edgeCounts, thresholdIn, thresholdOut, per)` generates clone rules:

```typescript
buildAutoCloneRules(
  edgeCounts: EdgeCounts,
  thresholdIn: number,
  thresholdOut: number,
  per: GroupingField,
): CloneRule[]
```

- For each entity where `incoming >= thresholdIn` or `outgoing >= thresholdOut`, generates a `CloneRule` with:
  - `field: "id"`: matches the exact entity ID
  - `value: entityId`: the specific entity that exceeded a threshold
  - `per: autoClone.per`: the configured grouping level for clone placement
- Entities exceeding both thresholds are deduplicated (one rule per entity).

**Integration into cloning pipeline:**

Auto-generated rules are prepended **before** manual `cloneRules` when passed to `cloneEntities()`. This means auto-clone rules are processed first. If a manual rule also matches the same entity, the auto-clone rule takes precedence (first match wins).

Updated data flow:

```text
HydraData
  ├── filters -> filteredEntities
  ├── -- Grouping Pipeline -----------------------------------
  ├── buildEntityGroupMap(groups) -> entityGroupMap
  ├── buildNscsMap(filteredEntities, entityGroupMap, groups, refs) -> nscsMap
  ├── -- Cloning ---------------------------------------------
  ├── buildEdgeCounts(refs) -> edgeCounts { incoming, outgoing }
  ├── buildAutoCloneRules(edgeCounts, thresholdIn, thresholdOut, per) -> autoRules
  ├── cloneEntities(filteredEntities, refs, [...autoRules, ...cloneRules],
  │                  groups, entityGroupMap, nscsMap)
  │     -> clonedEntities, clonedReferences, cloneIds
  │     -> entityGroupOverrides, nscsOverrides
  ├── entityGroupMap + entityGroupOverrides -> finalEntityGroupMap
  ├── nscsMap + nscsOverrides -> finalNscsMap
  ├── -- Tree Building ---------------------------------------
  ├── buildTree(clonedEntities, grouping, finalEntityGroupMap, finalNscsMap)
  │     -> TreeNode
  └── ... (layout, rendering as before)
```text

**State:**

```typescript
type AutoCloneConfig = {
  enabled: boolean; // whether auto-clone is active (default: true)
  thresholdIn: number; // minimum incoming edge count to trigger cloning (default: 10)
  thresholdOut: number; // minimum outgoing edge count to trigger cloning (default: 10)
  per: GroupingField; // grouping level for clone placement (default: last active grouping level)
};

// In HydraUiState:
autoClone: AutoCloneConfig; // default: { enabled: true, thresholdIn: 10, thresholdOut: 10, per: <last grouping level> }
```

The `per` field defaults to the **deepest** (last) active grouping level: the level that produces leaf groups with no sub-groups. When the grouping configuration changes and the current `per` value is no longer an active grouping level, `per` falls back to the new deepest level.

**UI (Clone section in TreePanel):**

The Auto-Clone controls appear at the **top** of the Clone section, above the manual clone rules:

- **Checkbox** `"Auto-Clone"` toggles `autoClone.enabled`
- **`<-` (`thresholdIn`)** number input: incoming edge threshold, only visible when enabled
- **`->` (`thresholdOut`)** number input: outgoing edge threshold, only visible when enabled
- **Per** dropdown: only visible when enabled, shows only active grouping levels, defaults to the deepest level
- **Auto-generated rules list**: only visible when enabled and rules exist. Each entry shows the entity ID and its edge counts (`<-N ->N`). Entries are **read-only** and visually distinguished to indicate they are auto-generated.

When the checkbox is unchecked, no auto-rules are generated and the controls and list are hidden.

Below the auto-clone controls, the manual clone rules with their `[+]` button appear as before.

**Persistence:** `autoClone` is a separate top-level field in `HydraUiState`. When matching the default (`{ enabled: true, thresholdIn: 10, thresholdOut: 10, per: <deepest level> }`), the field is stripped from the serialized YAML. See [state.md](../state.md).

**Implementation:** `buildEdgeCounts()`, `buildAutoCloneRules()` in `src/cloneLogic.ts`. Tests in `src/__tests__/cloneLogic.test.ts`.
