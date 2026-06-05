# IncomingRbacPanel

**File:** `src/components/IncomingRbacPanel.tsx`

## Function

Analyses and displays incoming RBAC permissions for a target entity. Shows which Roles and ClusterRoles grant access to the entity's resource type (kind), and which ServiceAccounts are bound to those roles. Uses `analyseIncomingRbac()` to compute the permission chain.

Supports navigation to outgoing RBAC view of a specific entity, jumping to the graph, and zooming to a namespace.

## Props

| Prop                   | Type                          | Description                               |
| ---------------------- | ----------------------------- | ----------------------------------------- |
| `targetEntity`         | `HydraEntity`                 | The entity whose incoming RBAC to analyse |
| `allEntities`          | `Map<string, HydraEntity>`    | All entities for reference resolution     |
| `reachability`         | `ReachabilityMap`             | Reachability data for RBAC chain analysis |
| `isDark`               | `boolean`                     | Current dark mode state                   |
| `onClose`              | `() => void`                  | Called when the panel is closed           |
| `onNavigateToOutgoing` | `(entityId: string) => void`  | Navigate to outgoing RBAC for an entity   |
| `onJumpToGraph`        | `(entityId: string) => void`  | Jump to entity in graph view              |
| `onZoomToNamespace`    | `(namespace: string) => void` | Zoom graph to a namespace                 |

## Used by

Not currently directly imported. The incoming RBAC functionality is implemented inline in `EntityPage.tsx` as `IncomingRbacTabContent`.
