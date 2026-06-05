# Grouping Keys

## Overview

Grouping Keys are user-defined entity categories. Each key independently evaluates every entity and produces a resolved key string (e.g. `"true"`, `"false"`, `"core"`). They enable flexible categorisation, coloring, and filtering — independent of the Kubernetes resource hierarchy.

Each Grouping Key appears as its own column in the entity list (prefixed `gk:` internally, displayed by name). There is no generic "Grouping Key" column.

**Source:** `GroupingKeyDefinition` type in `src/model.ts`, resolution logic in `src/groupingKeyLogic.ts`.

## Data Model

```typescript
type GroupingKeyEntry = {
  key: string; // resolved output key (e.g. "true", "core")
  field: string; // source field for THIS entry (e.g. "group", "namespace", "gk:OtherKey")
  values: string[]; // field values that map to this key
  pathLevel?: number; // persisted metadata for templatePath entries (not used in runtime matching)
};

type GroupingKeyDefinition = {
  name: string; // unique name (e.g. "Type")
  entries: GroupingKeyEntry[]; // key/value pairs — first matching entry wins
  fallbackKey: string; // key for entities that match no entry (e.g. "false")
};
```text

### Properties

#### GroupingKeyDefinition

| Property      | Type                 | Required | Description                                                                                      |
| ------------- | -------------------- | -------- | ------------------------------------------------------------------------------------------------ |
| `name`        | `string`             | yes      | Unique identifier. Shown as column header and in dropdowns.                                      |
| `entries`     | `GroupingKeyEntry[]` | yes      | Ordered list of key/value pairs. Each entry has its own source field. First matching entry wins. |
| `fallbackKey` | `string`             | yes      | Output key for entities that match no entry.                                                     |

#### GroupingKeyEntry

| Property    | Type       | Required | Description                                                                                                                                         |
| ----------- | ---------- | -------- | --------------------------------------------------------------------------------------------------------------------------------------------------- |
| `key`       | `string`   | yes      | The output key when this entry matches.                                                                                                             |
| `field`     | `string`   | yes      | Source field for this entry: a built-in `GroupingField` (e.g. `"group"`, `"namespace"`, `"kind"`), `"templatePath"`, or `"gk:<name>"` for chaining. |
| `values`    | `string[]` | yes      | Field values that match this entry.                                                                                                                 |
| `pathLevel` | `number`   | no       | Metadata for template-path grouping entries. Persisted in state; currently not used by `buildGroupingKeyMaps()` for matching.                       |

## Resolution Algorithm

Each grouping key is evaluated **independently** for every entity:

For a given entity and key definition:

1. Walk through `entries` in order
2. For each entry, resolve the entity's value for that entry's `field`
3. If the entity's field value is in the entry's `values`, the resolved key is the entry's `key` → stop
4. If no entry matches, the resolved key is `fallbackKey`

Each entry can use a **different source field**, allowing a single grouping key to combine criteria from multiple entity properties (e.g. namespace AND kind).

### Result Structure

```typescript
type GroupingKeyMaps = Map<string, Map<string, string>>;
// outer key: key definition name
// inner map: entity ID → resolved key string
```

### Entry Field Resolution

| Field type                                                | Behavior                                                                    |
| --------------------------------------------------------- | --------------------------------------------------------------------------- |
| Built-in field (`"group"`, `"namespace"`, `"kind"`, etc.) | Resolves the entity's field value via `getFieldValue()`                     |
| `"templatePath"`                                          | Uses `entity.templatePath` directly (empty → skip this entry, fall through) |
| `"gk:<name>"`                                             | Chains from a previously resolved grouping key's result                     |
| Unknown field                                             | Skip this entry (field value is undefined, entry cannot match)              |

## Default Keys

The effective defaults (loaded from `public/hydra-ui-defaults.yaml`) include one default key:

```yaml
groupingKeys:
  - name: Type
    entries:
      - key: Kubernetes
        field: group
        values:
          - "" # Core API (Pods, Services, ConfigMaps, ...)
          - apps
          - batch
          - autoscaling
          - networking.k8s.io
          - rbac.authorization.k8s.io
          - policy
          - storage.k8s.io
          - apiextensions.k8s.io
          - admissionregistration.k8s.io
          - certificates.k8s.io
          - coordination.k8s.io
          - discovery.k8s.io
          - events.k8s.io
          - scheduling.k8s.io
          - node.k8s.io
          - flowcontrol.apiserver.k8s.io
      - key: ArgoCD
        field: group
        values:
          - argoproj.io
      - key: Cluster Infra
        field: group
        values:
          - cert-manager.io
          - isindir.github.com
          - kyverno.io
          - monitoring.coreos.com
      - key: Demo Infra
        field: group
        values:
          - clickhouse.altinity.com
          - kafka.strimzi.io
    fallbackKey: other
```text

Entities are classified by API group into `"Kubernetes"`, `"ArgoCD"`, `"Cluster Infra"`, `"Demo Infra"`, or `"other"` (fallback).

## Examples

### Example 1: Multi-Category API Group Classification

```yaml
groupingKeys:
  - name: Type
    entries:
      - key: Kubernetes
        field: group
        values: ["", apps, batch, networking.k8s.io]
      - key: ArgoCD
        field: group
        values: [argoproj.io]
    fallbackKey: other
```

Entities are categorised into `"Kubernetes"`, `"ArgoCD"`, or `"other"`.

### Example 2: Multi-Category Classification

```yaml
groupingKeys:
  - name: Category
    entries:
      - key: core
        field: group
        values: [""]
      - key: apps
        field: group
        values: [apps]
      - key: network
        field: group
        values: [networking.k8s.io]
    fallbackKey: custom
```text

Each entity gets one of: `"core"`, `"apps"`, `"network"`, or `"custom"`.

### Example 3: Mixed-Field Classification

```yaml
groupingKeys:
  - name: Classification
    entries:
      - key: system
        field: namespace
        values: [kube-system, kube-public]
      - key: workload
        field: kind
        values: [Deployment, StatefulSet, DaemonSet]
    fallbackKey: other
```

System namespace entities → `"system"`, workload kinds → `"workload"`, everything else → `"other"`. Each entry uses its own source field.

### Example 4: Namespace Classification

```yaml
groupingKeys:
  - name: Environment
    entries:
      - key: system
        field: namespace
        values: [kube-system, kube-public]
      - key: prod
        field: namespace
        values: [production]
    fallbackKey: dev
```text

## UI

The Grouping Keys editor is a dedicated tab in the Settings Page (`"Grouping Keys"` tab). Direct URL: `#<cluster>?page=settings&tab=groupingKeys`.

Key UI behavior:

- All entries are **always in edit mode** (no toggle between view/edit)
- Each key shows: name field, entries list, fallback key field
- Each entry shows: key string field, **source field dropdown** (per entry), values list with add/remove
- The source field dropdown allows selecting built-in fields, `templatePath`, or previously defined grouping keys (`gk:` prefix)
- Available values in the "add value" dropdown are computed from the selected source field
- Actions: **Save/Apply/Revert/Cancel** via the edit-copy action bar, **Delete** (per key and per entry), **Add entry**, **Add key**
- Values can be added from a dropdown of available field values not yet assigned

### Backwards Compatibility

Old state data with `field` on the `GroupingKeyDefinition` level (instead of on each entry) is automatically migrated during deserialization: the definition-level `field` is applied to all entries that don't have their own `field`.

## Integration

Resolved grouping key values are available via the `gk:` field prefix:

- **Entity list columns**: Each key appears as a selectable column (e.g. column field `"gk:Type"`)
- **Color rules**: Color entities/groups by `gk:KeyName` field
- **Filters**: Filter entities by `gk:KeyName` in the entity list

The internal column/field identifier format is `gk:<key name>` (e.g. `"gk:Type"`).

### entityListColumns ↔ groupingKeys Sync

When grouping keys are renamed or deleted, `entityListColumns` is automatically kept in sync:

- **Rename detection**: Old and new key arrays are compared index-by-index. If a name changed at the same index, `gk:<oldName>` columns are rewritten to `gk:<newName>`.
- **Orphan removal**: Any `gk:` column referencing a non-existent key is automatically removed.
- **On settings apply**: `syncEntityListColumns()` is called when the settings page applies new grouping keys.
- **On deserialization**: `cleanEntityListColumns()` removes stale `gk:` references when loading persisted state.

Both functions are defined in `src/state.ts`.
