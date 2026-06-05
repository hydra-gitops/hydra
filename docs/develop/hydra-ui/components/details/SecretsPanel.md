# SecretsPanel

**File:** `src/components/SecretsPanel.tsx`

## Function

Displays Secret-related information for an entity:

- **Secret keys** — Lists all keys in the Secret data
- **Producers** — Shows resources that produce this Secret (edges whose **attributes** include **`"origin:generated": job`** or **`"origin:generated": controller`**, matching Hydra ref-parser semantics; ref `labels` are display-only)
- **Consumers** — Shows Deployments, StatefulSets, etc. that mount or reference this Secret
- **References** — Shows direct references to/from this Secret entity

Supports navigation to related entities in the graph.

## Props

| Prop                 | Type                         | Description                                    |
| -------------------- | ---------------------------- | ---------------------------------------------- |
| `entity`             | `HydraEntity`                | The Secret entity to analyse                   |
| `allEntities`        | `Map<string, HydraEntity>`   | All entities for reference resolution          |
| `references`         | `HydraReference[]`           | All references for finding producers/consumers |
| `isDark`             | `boolean`                    | Current dark mode state                        |
| `onClose`            | `() => void`                 | Called when the panel is closed                |
| `onJumpToGraph`      | `(entityId: string) => void` | Jump to entity in graph view                   |
| `onNavigateToEntity` | `(entityId: string) => void` | Navigate to another entity's details           |

## Used by

Not currently directly imported. The secrets functionality is implemented inline in `EntityPage.tsx` as a dedicated tab.
