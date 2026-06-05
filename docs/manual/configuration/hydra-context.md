# Configuration: HYDRA_CONTEXT

The environment variable and flag that select which Hydra context directory Hydra reads from.

## Setting the context

```bash
export HYDRA_CONTEXT=/path/to/hydra/context
```

Or per command:

```bash
hydra --hydra-context /path/to/hydra/context local template 'prod.**'
```

When neither `HYDRA_CONTEXT` nor `--hydra-context` is set, Hydra uses the current working directory.

## What is a Hydra context?

The Hydra context is the root directory that contains cluster definitions, charts, values, and encrypted secrets. Almost every `hydra local` and `hydra gitops` command needs it.

In a typical layout, cluster definitions live under something like `gitops-repository/clusters/<cluster>/`, while charts live in a separate charts repository that Hydra resolves from the context configuration.

## Validation Rules

Hydra validates context and cluster values as follows:

- `HYDRA_CONTEXT/values.yaml` is optional, but if present it must **not** define `global.hydra.path`.
- Each cluster directory under `HYDRA_CONTEXT` must contain `<cluster>/values.yaml` with `global.hydra.path`.

`global.hydra.path` is cluster-specific and must be configured at cluster level only.

Hydra context detection no longer depends on an `in-cluster/argocd` directory.

## Relationship to Kubernetes context

`HYDRA_CONTEXT` is independent of your active `kubectl` context. You can point Hydra at the `prod` GitOps tree while your kubeconfig still targets another cluster.

Use [`hydra gitops validate-current-context`](../commands/cluster/validate-current-context.md) to verify that the live cluster matches the cluster name you intend to operate on.

Warning: this validation applies to `hydra gitops` only. The reserved `hydra cluster` command surface will not use local Hydra files, so `HYDRA_CONTEXT` cannot be part of that validation.

## See Also

- [config.yaml](config-yaml.md) — optional per-user kubeconfig mapping
- [Concepts: Context and Clusters](../concepts/context-and-clusters.md)
- [Kubernetes Context](kubernetes-context.md) — kubectl requirements
