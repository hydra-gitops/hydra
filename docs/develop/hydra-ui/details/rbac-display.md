# Entity Page & RBAC Display Architecture

## Overview

The Entity Page is a unified, tabbed page that provides detailed information about any entity in the graph. It features a single shell with a tab bar for all entity details.

### Page Structure

```text
┌─────────────────────────────────────────────────────────────────────────────────┐
│  apps/v1/Deployment/default/nginx                        [Show in Graph]  ✕    │  ← Header (entity ID)
├──────────┬────────────┬────────────────┬───────────────┬───────────────────────┤
│ Details  │ Relations  │ Outgoing RBAC  │ Incoming RBAC │ Secrets*              │  ← Tab bar
├──────────┴────────────┴────────────────┴───────────────┴───────────────────────┤
│                                                                                │
│  (Tab content)                                                                 │
│                                                                                │
└────────────────────────────────────────────────────────────────────────────────┘

* Secrets tab only visible for `kind === "Secret"` entities
```

### Tabs

Tab navigation uses PrimeReact `TabMenu`. RBAC tabs use custom templates for "shield with arrow" icons.

| Tab               | Page type       | Icon             | Visible         | Question it answers                                                                                            |
| ----------------- | --------------- | ---------------- | --------------- | -------------------------------------------------------------------------------------------------------------- |
| **Details**       | `details`       | `pi-info-circle` | Always          | _"What are this entity's core fields (ID/GVK/namespace/name/tags/keys), and what filter can I add from them?"_ |
| **Relations**     | `relations`     | `pi-sitemap`     | Always          | _"What are the outgoing and incoming references of this entity?"_                                              |
| **Outgoing RBAC** | `outgoing-rbac` | `pi-shield` →    | Always          | _"What permissions does this entity (or the Roles/CRs it connects to) grant?"_                                 |
| **Incoming RBAC** | `incoming-rbac` | → `pi-shield`    | Always          | _"Who can access resources of this type?"_                                                                     |
| **Secrets**       | `secrets`       | `pi-key`         | Only for Secret | _"What keys does this secret contain and who uses them?"_                                                      |

### Navigation Model

- **Tab switch:** Changes the `page` parameter in the URL hash, keeps the same `node`.
- **Entity switch:** Changes the `node` parameter in the URL hash, keeps the same `page` (tab).
- **All entity links stay on the current tab.** Clicking an entity in a table replaces the current entity in the page without changing the tab.
- **Browser history:** Every tab or entity change creates a new `history.pushState()` entry, enabling back/forward navigation.

### Header

The header shows:

- A **breadcrumb-like entity identity path** (`group/version/kind/namespace/name`) with clickable filter segments
- A **"Show in Graph"** button that closes the page, selects the entity, and zooms to its parent group
- A **close button** (✕, also triggered by Escape)

---

## Entity Page Component

**Source file:** `src/components/EntityPage.tsx`

The `EntityPage` component is the unified shell. It:

1. Renders the header with entity ID
2. Renders the tab bar (conditional Secrets tab)
3. Delegates to the appropriate tab content component based on the active tab
4. Passes `onEntityChange` (not `onNavigateToIncoming` etc.) to all tabs — every link stays on the current tab

### Props

```typescript
type EntityPageProps = {
  entity: HydraEntity;
  activeTab: EntityTab; // "details" | "relations" | "outgoing-rbac" | "incoming-rbac" | "secrets"
  rbacRules: RbacRuleWithSource[];
  allEntities: Map<string, HydraEntity>;
  references: HydraReference[];
  reachability: ReachabilityMap;
  isDark: boolean;
  onClose: () => void;
  onTabChange: (tab: EntityTab) => void;
  onEntityChange: (entityId: string) => void;
  onJumpToGraph?: (entityId: string) => void;
  onZoomToNamespace?: (namespace: string) => void;
};
```

### Tab Content Components

All tab content components are defined inside `EntityPage.tsx`:

| Component                | Tab           | Description                                                                              |
| ------------------------ | ------------- | ---------------------------------------------------------------------------------------- |
| `DetailsTabContent`      | Details       | Entity metadata table with quick-add filter actions                                      |
| `RelationsTabContent`    | Relations     | Outgoing and incoming reference tables                                                   |
| `OutgoingRbacTabContent` | Outgoing RBAC | RBAC permissions table (flat for Role/ClusterRole, 3-level hierarchy for other entities) |
| `IncomingRbacTabContent` | Incoming RBAC | Incoming access analysis table                                                           |
| `SecretsTabContent`      | Secrets       | Secret keys and consumers                                                                |

---

## Details Tab

### Use Case

> _"I clicked on this Deployment — what are its key metadata fields, and can I quickly add any of them as a filter?"_

The Details tab renders a field/value table for the selected entity. It includes core identity fields plus dynamic grouping-key values (if configured), template source info, tags, and secret keys.

### Rows

Typical rows include:

- `ID`, `Group`, `Version`, `Kind`, `API Version`, `GVK`, `GVKN`, `Namespace`, `Name`
- One row per configured grouping key (resolved `gk:` value)
- `Template` (path + optional index) when available
- `Tags`
- `Secret Keys` when present

### Interaction

- Most rows expose a small filter icon.
- Clicking the icon calls `onAddFilter(field, value)` and appends that criterion to the active filter UI.
- Grouping-key rows map to `gk:<keyName>` fields.

---

## Shared RBAC Display Model (`model.ts`)

All RBAC display types are centralised in `model.ts`. Both panels import from this single source, eliminating duplication and ensuring type consistency.

### Constants

```typescript
const STANDARD_VERBS = [
  "get",
  "list",
  "watch",
  "create",
  "update",
  "patch",
  "delete",
  "deletecollection",
] as const;
```

Used by both RBAC panels as the fixed set of verb columns.

### Shared Base Types

```text
RbacRuleSource           ← identifies the originating Role/ClusterRole
  ├─ entityId
  ├─ entityName
  ├─ entityKind          ("Role" | "ClusterRole")
  └─ entityNamespace     ("" for ClusterRoles)

RbacRuleWithSource       ← an RBAC rule annotated with its source
  = RbacRule & { source: RbacRuleSource }

RbacRoleVerbEntry        ← base for any Role entry with granted verbs
  ├─ source: RbacRuleSource
  ├─ verbs: string[]
  └─ resourceNames: string[]

RbacDisplayScope         ← base for scope grouping
  ├─ namespace: string   ("" for cluster-scoped)
  └─ isClusterScoped: boolean
```

### Type Hierarchy

```text
model.ts
├── RbacRuleSource                (shared)
├── RbacRuleWithSource            (shared)
├── STANDARD_VERBS                (shared)
├── RbacRoleVerbEntry             (shared base)
├── RbacDisplayScope              (shared base)
│
├── Outgoing RBAC types
│   ├── OutgoingRbacSource        = RbacRoleVerbEntry  (per-source verb contribution)
│   ├── OutgoingRbacScope         = RbacDisplayScope + { verbs, sources: OutgoingRbacSource[] }
│   └── OutgoingRbacEntry         (apiGroup, resource) → scopes: OutgoingRbacScope[]
│
└── Incoming RBAC types
    ├── IncomingRbacRoleMatch     = RbacRoleVerbEntry + { connectedEntities: HydraEntity[] }
    ├── IncomingRbacScope         = RbacDisplayScope + { roles: IncomingRbacRoleMatch[] }
    ├── GvkOption                 (for filter dropdown)
    └── IncomingRbacResult        { scopes, allVerbs, gvkOptions, targetKind/Resource/ApiGroup }
```

### Full Type Definitions

```typescript
// --- Shared ---

type RbacRuleSource = {
  entityId: string;
  entityName: string;
  entityKind: string; // "Role" | "ClusterRole"
  entityNamespace: string; // "" for ClusterRoles
};

type RbacRuleWithSource = RbacRule & { source: RbacRuleSource };

type RbacRoleVerbEntry = {
  source: RbacRuleSource;
  verbs: string[];
  resourceNames: string[];
};

type RbacDisplayScope = {
  namespace: string; // "" for cluster-scoped
  isClusterScoped: boolean;
};

// --- Outgoing ---

type OutgoingRbacSource = RbacRoleVerbEntry;

type OutgoingRbacScope = RbacDisplayScope & {
  verbs: string[]; // union of all sources in this scope
  sources: OutgoingRbacSource[];
};

type OutgoingRbacEntry = {
  apiGroup: string;
  resource: string;
  verbs: string[]; // union of ALL scopes
  resourceNames: string[];
  scopes: OutgoingRbacScope[];
};

// --- Incoming ---

type IncomingRbacRoleMatch = RbacRoleVerbEntry & {
  connectedEntities: HydraEntity[];
};

type IncomingRbacScope = RbacDisplayScope & {
  roles: IncomingRbacRoleMatch[];
};

type GvkOption = {
  gvk: string;
  kind: string;
  count: number;
};

type IncomingRbacResult = {
  targetKind: string;
  targetResource: string;
  targetApiGroup: string;
  scopes: IncomingRbacScope[];
  allVerbs: string[];
  gvkOptions: GvkOption[];
};
```

---

## Data Flow

```text
hydra-go                          hydra.yaml                    hydra-ui
─────────                         ──────────                    ────────
Role/ClusterRole                  entities:                     parseHydraYaml()
  spec.rules[] ──────────────►      - id: .../Role/.../x          → HydraEntity.rbacRules
  extractRbacRules()                  rbacRules:
  → IdModel.RbacRules                   - apiGroups: [...]        App.tsx (graphNodes useMemo)
                                        resources: [...]            → annotateRules()
                                        verbs: [...]                → RbacRuleWithSource[]
                                                                    → GraphNode.rbacRules

                                                                  GraphPanel.tsx
                                                                    context menu:
                                                                    "📋 Details" → EntityPage (Details tab)
                                                                    Tab bar → EntityPage (any tab)
```

### Rule Collection (App.tsx)

RBAC rules are collected differently depending on the clicked entity:

| Entity kind            | Rule source                                                                                                  |
| ---------------------- | ------------------------------------------------------------------------------------------------------------ |
| **Role / ClusterRole** | Own `rbacRules` directly from the parsed YAML                                                                |
| **All other entities** | Traverse outgoing `ReachabilityMap` → collect `rbacRules` from all transitively reachable Roles/ClusterRoles |

Every collected rule is annotated with its source entity via `annotateRules()`.

**Example traversal:** Clicking an Ingress → outgoing reachability finds: Service (1 hop) → Deployment (2 hops) → ServiceAccount (3 hops) → RoleBinding (4 hops) → Role (5 hops). The Role's `rbacRules` (with source annotation) are shown in the Outgoing RBAC tab.

---

## Outgoing RBAC Tab

### Use Case

> _"I clicked on this Deployment — what permissions do the connected Roles/ClusterRoles grant?"_

The Outgoing RBAC tab collects all RBAC rules from Roles/ClusterRoles reachable via **outgoing** graph edges and aggregates them into a hierarchical table. All scopes (cluster-scoped and all namespaces) are always shown, since outgoing permissions from different namespaces are always relevant.

### Flat Mode for Role/ClusterRole Entities

When the selected entity is a **Role** or **ClusterRole**, the table is rendered in **flat mode**: only Level 1 rows (API Group + Resource) are shown, without expand/collapse or sub-levels. The scope (cluster-scoped vs namespace) and source role name are redundant because they are already visible in the entity header. Verb columns use simple ✓/— (no partial coverage indicator needed since there is only one source).

### Normalisation Pipeline (`normaliseRbacRules`)

The raw `RbacRuleWithSource[]` is normalised into a **3-level hierarchy**:

```text
Level 1: (apiGroup, resource)             ← main row, sorted by apiGroup then resource
  Level 2: scope (cluster / namespace)    ← scope row, cluster first then alphabetical
    Level 3: source Role/ClusterRole      ← source row, sorted by kind then name
```

#### Step 1: Expand

Each rule may have multiple `apiGroups` and `resources`. These are expanded into individual `(apiGroup, resource)` pairs. A rule with 2 apiGroups and 3 resources produces 6 expanded entries.

```text
Input:   { apiGroups: ["", "apps"], resources: ["pods", "deployments"], verbs: ["get"] }
Output:  ("", "pods", ["get"]),  ("", "deployments", ["get"]),
         ("apps", "pods", ["get"]),  ("apps", "deployments", ["get"])
```

#### Step 2: Collect into Hierarchy

Each expanded entry is placed into the 3-level structure:

- **Level 1 key:** `(apiGroup, resource)` — the permission target
- **Level 2 key:** `(isClusterScoped, namespace)` — ClusterRoles go to the cluster-scoped group, Roles go to their namespace group
- **Level 3 key:** `source.entityId` — individual Role/ClusterRole

#### Step 3: Merge Verbs at Every Level

At each level, verbs are merged as a set-union (no duplicates):

```text
Role A (default):   { apiGroups: [""], resources: ["pods"], verbs: ["get"] }
Role B (default):   { apiGroups: [""], resources: ["pods"], verbs: ["delete"] }
ClusterRole admin:  { apiGroups: [""], resources: ["pods"], verbs: ["list"] }

Level 1 (apiGroup="", resource="pods"):
  verbs: ["get", "delete", "list"]              ← union of ALL scopes

  Level 2 (Cluster Scoped):
    verbs: ["list"]                             ← union within this scope
    Level 3: ClusterRole/admin → ["list"]

  Level 2 (Namespace: default):
    verbs: ["get", "delete"]                    ← union within this scope
    Level 3: Role/A → ["get"]
    Level 3: Role/B → ["delete"]
```

#### Step 4: Sort

- **Level 1:** apiGroup ascending (empty string first), then resource ascending
- **Level 2:** Cluster-scoped first, then namespaces alphabetically
- **Level 3:** Source kind ascending, then name ascending

### Outgoing RBAC Columns

| # | Column | Content |
| --- | --- | --- |
| 1 | **API Group** | API group string. `""` displayed as `(core)`. Includes expand/collapse arrow (▶/▼). |
| 2 | **Resource** | Resource type. ResourceNames shown as italic suffix: `configmaps (cm-a, cm-b)`. |
| 3–10 | **Verb columns** | One per standard verb: Get, List, Watch, Create, Update, Patch, Del, DCol. Three states at Level 1: green **✓** = granted in all scopes, orange **◐** = granted in some but not all scopes, grey **—** = not granted. Wildcard `*` counts as granting the verb. Level 2/3 rows use simple ✓/—. |
| 11 | **Extra** | Non-standard verbs as comma-separated italic text. Only shown if any entry has extra verbs. |

**Note:** The previous "Graph" column has been removed. Entity links (Level 3 source rows) now call `onEntityChange` to navigate to that entity on the same tab.

#### 3-Level Row Hierarchy (non-Role/ClusterRole entities only)

When the user clicks a Level 1 row, it expands to show Level 2 (scope) and Level 3 (source) rows:

```text
▶  (core)  pods          ◐  ◐  ◐  —  —  —  ◐  —         ← Level 1 (collapsed): ◐ = partial

▼  (core)  pods          ◐  ◐  ◐  —  —  —  ◐  —         ← Level 1 (expanded): partial coverage
      Cluster Scoped     ✓  —  ✓  —  —  —  —  —         ← Level 2: scope row (simple ✓/—)
        ↳ ClusterRole/admin   ✓  —  ✓  —  —  —  —  —   ← Level 3: source row (link → same tab)
      Namespace: default ✓  ✓  —  —  —  —  ✓  —         ← Level 2: scope row
        ↳ Role/editor   —  —  —  —  —  —  ✓  —         ← Level 3: source row (link → same tab)
        ↳ Role/viewer   ✓  ✓  —  —  —  —  —  —         ← Level 3: source row (link → same tab)
```

**Verb coverage symbols (Level 1 only):**

| Symbol | Colour | Meaning                                                    |
| ------ | ------ | ---------------------------------------------------------- |
| **✓**  | Green  | Verb granted in **all** scopes                             |
| **◐**  | Orange | Verb granted in **some** scopes (hover shows `n/m scopes`) |
| **—**  | Grey   | Verb not granted in any scope                              |

Level 2 and Level 3 rows always use simple ✓/— since they represent a single scope or source.

---

## Incoming RBAC Tab

### Use Case

> _"I clicked on this Secret — who can get/delete/list this secret?"_

The Incoming RBAC tab scans **all** Roles/ClusterRoles in the loaded data for rules that match the clicked entity's resource type and API group, then traverses the **incoming** reachability graph to find which entities (RoleBindings, ServiceAccounts, Workloads) are connected to those Roles.

### Analysis Pipeline (`analyseIncomingRbac`)

#### Step 1: Determine Target Resource

The clicked entity's Kind is mapped to a Kubernetes resource name:

```text
Kind → resource name
Secret → secrets
Deployment → deployments
Ingress → ingresses (exception)
NetworkPolicy → networkpolicies (exception)
```

The mapping uses `kindToResourceName()`: well-known exceptions first, then `kind.toLowerCase() + "s"` as fallback.

#### Step 2: Scan for Matching Roles/ClusterRoles

All Roles and ClusterRoles in the loaded data are scanned. A rule matches if:

- `rule.apiGroups` contains the target's API group (or `"*"`)
- `rule.resources` contains the target's resource name (or `"*"`)

All matching verbs from all matching rules within a Role/CR are collected.

#### Step 3: Find Connected Entities

For each matching Role/CR, the **incoming** `ReachabilityMap` is traversed to find entities that reference it. This reveals the access chain:

```text
Role ← RoleBinding ← ServiceAccount ← Deployment
                                     ← StatefulSet
```

Connected Roles/ClusterRoles are excluded (only non-RBAC entities are shown).

#### Step 4: Group by Scope

Matching Roles/CRs are grouped by scope:

- **Cluster Scoped** — ClusterRoles (sorted first)
- **Namespace: X** — Roles in specific namespaces (alphabetical)

Within each scope, Roles are sorted by kind then name.

### Incoming RBAC Controls

Above the table:

- **Resource/apiGroup info** line
- **"All namespaces" checkbox** — unchecked by default (restricts to relevant scopes)
- **GVK filter** chip buttons for connected entity types

### Namespace Filtering

By default, the "All namespaces" checkbox is **unchecked**. This restricts the displayed scopes to only those relevant to the selected entity:

| Scope type                    | Shown when unchecked | Rationale                                                            |
| ----------------------------- | -------------------- | -------------------------------------------------------------------- |
| **Cluster-scoped**            | Always               | ClusterRoles apply to all namespaces, always relevant                |
| **Same namespace**            | Always               | Roles in the entity's own namespace directly affect it               |
| **All namespaces** (empty ns) | Always               | Rules without namespace restriction apply everywhere                 |
| **Other namespaces**          | Only when checked    | A Role in namespace `prod` cannot access a Secret in namespace `dex` |

### GVK Filter (Multi-Select)

Chip buttons let users filter which connected entity types are shown:

- When **no chip is selected** (default): all types are shown
- Clicking a chip **toggles** its active state
- **Multiple chips** can be active simultaneously

### Incoming RBAC Row Hierarchy

```text
┌─ Cluster Scoped ─────────────────────────────────────────────────┐
│  ▶ ClusterRole/admin               ✓  ✓  ✓  ✓  ✓  ✓  ✓  ✓     │
│  ▼ ClusterRole/secret-reader       ✓  ✓  ✓  —  —  —  —  —     │
│    ↳ ClusterRoleBinding/default/crb1         (link → same tab)   │
│    ↳ ServiceAccount/kube-system/monitoring   (link → same tab)   │
├─ Namespace: default ─────────────────────────────────────────────┤
│  ▼ Role/secret-editor              ✓  ✓  —  —  ✓  ✓  ✓  —     │
│    ↳ RoleBinding/default/rb1                 (link → same tab)   │
│    ↳ ServiceAccount/default/app-sa           (link → same tab)   │
│    ↳ Deployment/default/my-app               (link → same tab)   │
└──────────────────────────────────────────────────────────────────┘
```

**Note:** The previous "Graph" column has been removed. All entity links call `onEntityChange` to navigate to that entity on the same tab.

---

## Secrets Tab

### Use Case

> _"I clicked on this Secret — what keys does it contain and who uses them?"_

The Secrets tab is only visible for entities with `kind === "Secret"`.

### Content

1. **Entity info** — Namespace, Tags
2. **Produced by** — SopsSecrets that generate this Secret (if any)
3. **Secret Keys table** — Lists each key with all entities that reference this Secret
4. **Consumed by** — All entities that reference this Secret

All entity links call `onEntityChange` to navigate to that entity on the same tab.

---

## Graph Edge Labels

Reference labels (relation names) are displayed as text on graph edges:

- **Label source:** `edge.data("label")` — set from `HydraReference.labels.join(", ")` during graph model computation
- **Text style:** Auto-rotated along edge, 9px font, semi-transparent background for readability
- **Dark mode:** Label text colour and background adapted (#999 on #1e1e1e)

---

## Mouse Interaction

| Interaction                                          | Effect                                                                 |
| ---------------------------------------------------- | ---------------------------------------------------------------------- |
| **Row hover**                                        | Entire row highlighted                                                 |
| **Column hover** (verb columns only)                 | Entire column highlighted (including header)                           |
| **Cell at row+column intersection**                  | Stronger highlight                                                     |
| **Click expandable row**                             | Toggle expand/collapse                                                 |
| **Click entity link**                                | Navigate to that entity (same tab)                                     |
| **Escape key**                                       | Close page                                                             |
| **Click "Show in Graph"**, then click an entity node | First click selects the node, clicking the selected node opens Details |
| **Click tab**                                        | Switch to that tab (same entity)                                       |

Hover colours adapt to light/dark mode.

---

## Context Menu

| Label | Available on | Description |
| --- | --- | --- |
| **📋 Details** | All entity nodes | Open Entity Page (Details tab) |
| **⇄ Show/Hide Deps** | All entity nodes | Toggle dependency highlighting |
| **⧉ Clone** | Clone nodes only | Open Clone Page |

The context menu opens on right-click (cxttapstart) or long-press (taphold). RBAC and Secrets views are accessible as tabs within the Entity Page.

---

## hydra.yaml Format

RBAC rules are stored only on Role and ClusterRole entities:

```yaml
entities:
  - id: rbac.authorization.k8s.io/v1/Role/default/demo-role
    templatePath: my-chart/templates/rbac.yaml
    templateIndex: 1
    rbacRules:
      - apiGroups: [""]
        resources: ["pods"]
        verbs: ["get", "list", "watch"]
      - apiGroups: ["apps"]
        resources: ["deployments"]
        verbs: ["get"]
  - id: rbac.authorization.k8s.io/v1/ClusterRole/admin
    rbacRules:
      - apiGroups: ["*"]
        resources: ["*"]
        verbs: ["*"]
```

Secret keys are stored on Secret entities and SopsSecret-produced Secrets:

```yaml
entities:
  - id: v1/Secret/default/my-secret
    tags:
      - controller:sops-secrets-operator
    secretKeys:
      - API_KEY
      - DB_PASSWORD
```

### RBAC Rule Fields

| Field           | Type     | Required | Description                                                     |
| --------------- | -------- | -------- | --------------------------------------------------------------- |
| `apiGroups`     | string[] | yes      | Kubernetes API groups (`""` = core API, `"*"` = all)            |
| `resources`     | string[] | yes      | Resource types (e.g. `pods`, `deployments`, `"*"` = all)        |
| `verbs`         | string[] | yes      | Allowed operations (e.g. `get`, `list`, `"*"` = all)            |
| `resourceNames` | string[] | no       | Specific resource names (restricts the rule to named resources) |

## hydra-go Implementation

The Go backend extracts RBAC rules in `core/view/dependencies.go`:

- **`RbacRuleModel`** struct mirrors the YAML format
- **`extractRbacRules(entity)`** checks if the entity is a Role or ClusterRole, then reads `rules[]` from the unstructured Kubernetes object using `NestedSlice`
- **`extractSecretKeys(entity)`** extracts key names from `data` and `stringData` fields of Secret entities
- **`extractSopsSecretKeys(entity)`** extracts key names from `spec.secretTemplates[].data` and `spec.secretTemplates[].stringData` for SopsSecret entities
- Only entities matching `rbac.authorization.k8s.io/v1/Role` or `rbac.authorization.k8s.io/v1/ClusterRole` produce RBAC rules
- The `IdModel.RbacRules` field is `omitempty` — non-RBAC entities have no `rbacRules` key in the YAML
- The `IdModel.SecretKeys` field is `omitempty` — non-Secret entities have no `secretKeys` key in the YAML

---

## Performance

- The Entity Page uses an **opaque** background (`#1e1e1e` / `#ffffff`), completely hiding the graph.
- When any page is open, the Cytoscape container is removed from the DOM (`display: none`) so the browser does not paint or run layout on the graph.
- Extensions (`nodeHtmlLabel`, `cxtmenu`) are registered with guard checks to prevent duplicate registration during Hot Module Replacement.

---

## Files

| File                                   | Purpose                                                                                                                                                                                                                                                                                                                                    |
| -------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `src/model.ts`                         | All RBAC types: `RbacRule`, `RbacRuleSource`, `RbacRuleWithSource`, `STANDARD_VERBS`, shared bases (`RbacRoleVerbEntry`, `RbacDisplayScope`), Outgoing types (`OutgoingRbacSource/Scope/Entry`), Incoming types (`IncomingRbacRoleMatch/Scope/Result`, `GvkOption`), `HydraEntity.secretKeys`                                              |
| `src/incomingRbac.ts`                  | `kindToResourceName()`, `analyseIncomingRbac()` pure functions                                                                                                                                                                                                                                                                             |
| `src/components/EntityPage.tsx`        | **Unified Entity Page** with tabs: Details, Outgoing RBAC, Incoming RBAC, Secrets. Contains `EntityPage`, `DetailsTabContent`, `OutgoingRbacTabContent`, `IncomingRbacTabContent`, `SecretsTabContent`. Imports `normaliseRbacRules` and `computeVerbCoverage` from `RbacInfoPanel.tsx`, and `analyseIncomingRbac` from `incomingRbac.ts`. |
| `src/components/RbacInfoPanel.tsx`     | Pure logic exports: `normaliseRbacRules()`, `computeVerbCoverage()`. Standalone RBAC panel component.                                                                                                                                                                                                                                      |
| `src/components/IncomingRbacPanel.tsx` | Standalone incoming RBAC panel component (logic used via `analyseIncomingRbac`).                                                                                                                                                                                                                                                           |
| `src/components/SecretsPanel.tsx`      | Standalone legacy/alternate secrets panel component.                                                                                                                                                                                                                                                                                       |
| `src/components/GraphPanel.tsx`        | Context menu (📋 Details), page state, renders `EntityPage`                                                                                                                                                                                                                                                                                |
| `src/App.tsx`                          | Rule collection, `annotateRules()`, passes `hydraEntities` and `hydraReferences` to GraphPanel                                                                                                                                                                                                                                             |
| `src/parseHydra.ts`                    | Parses `rbacRules` and `secretKeys` from hydra.yaml                                                                                                                                                                                                                                                                                        |
| `src/__tests__/rbacNormalise.test.ts`  | Unit tests for outgoing RBAC normalisation (26 tests)                                                                                                                                                                                                                                                                                      |
| `src/__tests__/incomingRbac.test.ts`   | Unit tests for incoming RBAC analysis (12 tests)                                                                                                                                                                                                                                                                                           |
| `hydra-go/core/view/dependencies.go`   | `extractRbacRules()`, `extractSecretKeys()`, `extractSopsSecretKeys()`, `RbacRuleModel`, `toStringSlice()`                                                                                                                                                                                                                                 |

---

## Tests

### Outgoing RBAC (`rbacNormalise.test.ts` — 26 tests)

| Category            | What is tested                                                                                                        |
| ------------------- | --------------------------------------------------------------------------------------------------------------------- |
| **Expansion**       | Multiple resources/apiGroups → separate entries, empty arrays                                                         |
| **Sorting**         | Entries sorted by apiGroup then resource                                                                              |
| **Verb merging**    | Across roles, no duplication, wildcard `*`, resourceNames union                                                       |
| **Scope grouping**  | Cluster vs namespaced, cluster first, alphabetical namespaces                                                         |
| **Source tracking** | Per-source verbs, same-source merging, sorting by kind+name                                                           |
| **Complex**         | 5 rules, 3 scopes, correct hierarchy and verb distribution                                                            |
| **Verb coverage**   | `computeVerbCoverage()`: none/all/partial detection, single scope, multi-scope, wildcard `*`, mixed cluster+namespace |

### Incoming RBAC (`incomingRbac.test.ts` — 12 tests)

| Category               | What is tested                                                      |
| ---------------------- | ------------------------------------------------------------------- |
| **kindToResourceName** | Simple kinds, well-known exceptions (Ingress, NetworkPolicy, etc.)  |
| **No matches**         | Non-matching Roles return empty scopes                              |
| **Matching**           | Role with matching resource found, correct verbs                    |
| **Wildcards**          | `apiGroups: ["*"]` and `resources: ["*"]` match any target          |
| **Scope separation**   | ClusterRole vs Role in separate scopes, correct ordering            |
| **Connected entities** | Incoming reachability finds RoleBindings, SAs; excludes other Roles |
| **GVK options**        | Correct kinds, counts, sorted by count descending                   |
| **Verb merging**       | Multiple matching rules in same Role are merged                     |
| **allVerbs**           | Union across all matching Roles/CRs                                 |
| **Target metadata**    | Correct targetKind, targetResource, targetApiGroup                  |
