# Entity Page & RBAC Display

The Entity Page is a unified, tabbed page providing detailed information about any entity in the graph. It includes entity details, relations, outgoing/incoming RBAC analysis, and secret key inspection.

## Key Concepts

- **Unified entity page** — Single shell with tab bar: Details, Relations, Outgoing RBAC, Incoming RBAC, Secrets (Secret entities only)
- **Outgoing RBAC** — Collects RBAC rules from transitively reachable Roles/ClusterRoles via outgoing edges; 3-level hierarchy (apiGroup/resource → scope → source)
- **Incoming RBAC** — Scans all Roles/ClusterRoles for rules matching the entity's resource type; traverses incoming reachability to find connected entities
- **Verb coverage** — Three-state indicators at Level 1: green (all scopes), orange (partial), grey (none)
- **Shared RBAC types** — Centralised in `model.ts`: `RbacRuleSource`, `RbacRuleWithSource`, `STANDARD_VERBS`, display scope/entry types
- **Secrets tab** — Shows secret keys, producers (SopsSecrets), and consumers for Secret entities
- **Navigation model** — All entity links navigate within the same tab; tab/entity changes create history entries

## Source Files

| File                                   | Purpose                                             |
| -------------------------------------- | --------------------------------------------------- |
| `src/model.ts`                         | All RBAC types, `HydraEntity.secretKeys`            |
| `src/incomingRbac.ts`                  | `kindToResourceName()`, `analyseIncomingRbac()`     |
| `src/components/EntityPage.tsx`        | Unified entity page with all tab content components |
| `src/components/RbacInfoPanel.tsx`     | `normaliseRbacRules()`, `computeVerbCoverage()`     |
| `src/components/IncomingRbacPanel.tsx` | Incoming RBAC panel component                       |
| `src/App.tsx`                          | Rule collection, `annotateRules()`                  |

→ **Full details:** [details/rbac-display.md](details/rbac-display.md)
