# Grouping Keys

Grouping Keys are user-defined entity categories. Each key independently evaluates every entity using ordered entries with per-entry source fields and produces a resolved key string. They enable flexible categorisation, coloring, and filtering independent of the Kubernetes resource hierarchy.

Note: The grouping system is fully defined by grouping keys. There is no separate grouping configuration — all grouping behavior is driven through `GroupingKeyDefinition` entries. See also [details/grouping-keys.md](details/grouping-keys.md) for the resolution algorithm.

## Key Concepts

- **GroupingKeyDefinition** — Named key with ordered entries (each with its own source field) and a fallback key
- **Resolution algorithm** — Walk entries in order; first matching entry wins; each entry can use a different source field
- **Entry field types** — Built-in fields (`group`, `namespace`, `kind`, etc.), `templatePath`, or chained grouping keys (`gk:<name>`)
- **Default key** — `Type` key classifying entities by API group into Kubernetes, ArgoCD, Cluster Infra, Demo Infra, or other
- **Integration** — Resolved values available as entity list columns (`gk:<name>`), color rule fields, and filter criteria
- **Column sync** — `entityListColumns` automatically updated when grouping keys are renamed or deleted

## Source Files

| File                              | Description                                       |
| --------------------------------- | ------------------------------------------------- |
| `src/model.ts`                    | `GroupingKeyDefinition`, `GroupingKeyEntry` types |
| `src/groupingKeyLogic.ts`         | `buildGroupingKeyMaps()` resolution logic         |
| `src/state.ts`                    | Persistence, backwards-compatible migration       |
| `src/components/SettingsPage.tsx` | Grouping Keys editor tab                          |

→ **Full details:** [details/grouping-keys.md](details/grouping-keys.md)
