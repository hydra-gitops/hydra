# EntityPage

**File:** `src/components/EntityPage.tsx`

## Function

Full-page detail overlay for a single entity, rendered on top of the graph panel. Contains a breadcrumb header and multiple tabs:

- **Details** — Key-value table of entity metadata (namespace, kind, name, group, version, appId, etc.)
- **Relations** — Paginated DataTable of incoming/outgoing references with distance, direction, ref-parser labels, and an optional **Role** column (**in** / **out**): ref labels first, then kind fallbacks (**Secret**/**ConfigMap** → in, **Pod**/**ReplicaSet** → out), without using `reverse`
- **Outgoing RBAC** — RBAC rules analysis (normalised, grouped by API group/resource, verb coverage matrix)
- **Incoming RBAC** — Reverse RBAC analysis showing which ServiceAccounts/Roles can access this entity's type
- **Secrets** — Secret key analysis with producers, consumers, and references
- **Manifest** — Raw manifest YAML loaded on demand from the cluster archive
- **Template** — Go template source loaded on demand from the cluster archive
- **Values** — Unified values view (delegates to `ValuesView`) when chart info is available

Includes a breadcrumb bar with clickable segments for namespace, kind, etc. that set selection filters. Navigation between entities is supported via clickable references.

## Props

| Prop                      | Type                                    | Description                                   |
| ------------------------- | --------------------------------------- | --------------------------------------------- |
| `entity`                  | `HydraEntity`                           | The entity to display                         |
| `activeTab`               | `EntityTab`                             | Currently active tab                          |
| `rbacRules`               | `RbacRuleWithSource[]`                  | Pre-computed RBAC rules for this entity       |
| `allEntities`             | `Map<string, HydraEntity>`              | All entities (for reference resolution)       |
| `references`              | `HydraReference[]`                      | All references (for relation display)         |
| `reachability`            | `ReachabilityMap`                       | Pre-computed reachability data                |
| `isDark`                  | `boolean`                               | Current dark mode state                       |
| `loader`                  | `ClusterLoader`                         | Loader for manifest/template/values data      |
| `onClose`                 | `() => void`                            | Called when the overlay is closed             |
| `onTabChange`             | `(tab: EntityTab) => void`              | Called when the active tab changes            |
| `onEntityChange`          | `(entityId: string) => void`            | Called when navigating to a different entity  |
| `onJumpToGraph`           | `(entityId: string) => void`            | Called to zoom the graph to this entity       |
| `onZoomToNamespace`       | `(namespace: string) => void`           | Called to zoom the graph to a namespace group |
| `onBreadcrumbFilter`      | `(filters: BreadcrumbFilter[]) => void` | Called when a breadcrumb segment is clicked   |
| `onNavigateToChartValues` | `() => void`                            | Called to navigate to the chart values page   |
| `groupingKeyMaps`         | `GroupingKeyMaps`                       | Resolved grouping key maps                    |
| `groupingKeys`            | `GroupingKeyDefinition[]`               | Grouping key definitions                      |
| `valuesConfig`            | `ValuesViewConfig`                      | Configuration for the values tab              |

## Exported Types

- `EntityTab` — `"details" | "relations" | "outgoing-rbac" | "incoming-rbac" | "secrets" | "manifest" | "template" | "values"`
- `BreadcrumbFilter` — `{ field: string; value: string }`

## Used by

- `src/components/GraphPanel.tsx` — rendered as an overlay when `page === "details"`
