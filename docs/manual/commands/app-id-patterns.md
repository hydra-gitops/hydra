# App ID Patterns

How to select one or more apps using glob patterns.

## Format

App IDs have the format: `<cluster>.<group>.<app>`

- **cluster** — The cluster name (subdirectory in HYDRA_CONTEXT)
- **group** — The app group (e.g., `cluster-infra`, `demo`, `cicd`)
- **app** — The child app name (e.g., `ingress-nginx`, `service-auth`)

Root apps use two segments: `<cluster>.<group>`

## Wildcards

| Pattern | Meaning |
|---------|---------|
| `*` | Matches exactly one segment |
| `**` | Matches one or more segments |

## Examples

| Pattern | Selects |
|---------|---------|
| `prod.cluster-infra.ingress-nginx` | One specific app |
| `prod.*` | All root apps on prod |
| `prod.cluster-infra.*` | All child apps in cluster-infra on prod |
| `prod.**` | All apps on prod (root + child, recursive) |
| `*.cluster-infra.*` | All cluster-infra child apps across all clusters |
| `*.*` | All root apps across all clusters |
| `*.**` | Everything |

## Excluding Apps

Use `--exclude-app` to remove specific apps from the selection:

```bash
# All apps on prod except cert-manager
hydra gitops apply 'prod.**' --exclude-app 'prod.cluster-infra.cert-manager'

# All cluster-infra except all longhorn apps
hydra gitops diff 'prod.cluster-infra.*' --exclude-app '*.*.longhorn*'
```

Multiple `--exclude-app` flags can be combined.

## Quoting

Always quote patterns containing `*` to prevent shell expansion:

```bash
# Correct
hydra local template 'prod.**'

# Wrong — shell expands * to filesystem matches
hydra local template prod.**
```

## Commands Accepting App IDs

Most commands accept one or more App ID patterns as positional arguments:

```bash
hydra local template <appId...>
hydra gitops apply <appId...>
hydra gitops diff <appId...>
hydra gitops status <appId...>
```

Some commands accept a cluster name instead:

```bash
hydra gitops system <cluster>
hydra gitops untracked <cluster>
hydra gitops dump <cluster>
hydra gitops inspect <cluster>
```

## See Also

- [Concepts: App Model](../concepts/app-model.md)
