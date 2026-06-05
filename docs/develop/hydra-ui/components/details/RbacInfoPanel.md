# RbacInfoPanel

**File:** `src/components/RbacInfoPanel.tsx`

## Function

Displays outgoing RBAC rules for an entity in a hierarchical view. Rules are grouped by API group/resource, then by scope (namespace/cluster), then by source Role/ClusterRole. Shows a verb coverage matrix indicating which RBAC verbs (get, list, watch, create, update, patch, delete, deletecollection) are granted.

Also exports utility functions for RBAC analysis that are used by other components.

## Exports

- `RbacInfoPanel` — The panel component
- `normaliseRbacRules(rules)` — Normalises and deduplicates RBAC rules
- `computeVerbCoverage(rules)` — Computes verb coverage matrix for a set of rules
- `STANDARD_VERBS` — Array of standard Kubernetes RBAC verbs

## Props

| Prop                   | Type                          | Description                                  |
| ---------------------- | ----------------------------- | -------------------------------------------- |
| `entityId`             | `string`                      | The entity whose RBAC rules to display       |
| `rbacRules`            | `RbacRuleWithSource[]`        | RBAC rules with source Role/ClusterRole info |
| `isDark`               | `boolean`                     | Current dark mode state                      |
| `onClose`              | `() => void`                  | Called when the panel is closed              |
| `onNavigateToIncoming` | `(entityId: string) => void`  | Navigate to incoming RBAC for an entity      |
| `onJumpToGraph`        | `(entityId: string) => void`  | Jump to entity in graph view                 |
| `onZoomToNamespace`    | `(namespace: string) => void` | Zoom graph to a namespace                    |

## Used by

- `src/components/EntityPage.tsx` — imports `normaliseRbacRules`, `computeVerbCoverage`, and `RbacInfoPanelProps` type (the component itself is re-implemented inline as tab content)
