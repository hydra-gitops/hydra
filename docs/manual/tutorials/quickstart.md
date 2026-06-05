# Hydra Quickstart

Use this page when you want the shortest safe path from "I have a Hydra context" to "I can operate a cluster".

For detailed command behavior, follow the links in each step.

## Prerequisites

You need:

- `kubectl` access to the target cluster
- A valid Hydra context directory
- `sops` configured if your context contains encrypted secrets

## 1. Set The Hydra Context

Hydra needs a Hydra context directory that contains the cluster definitions, charts, values, and secrets.

You can pass it explicitly with `--hydra-context <path>`, but in daily use it is usually more convenient to set `HYDRA_CONTEXT` once so you do not need to repeat the parameter on every command:

```bash
export HYDRA_CONTEXT=/path/to/hydra-context
```

Use `--hydra-context` when you need to override the default for a single command.

See [`README.md`](README.md#hydra-context).

## Command Modes

- `hydra local` works only on the local Hydra definitions in your workspace.
- `hydra gitops` uses those local definitions and also talks to the Kubernetes cluster.
- `hydra cluster` is reserved for future cluster-only workflows when the local Hydra state is not available.

## 2. Validate The Target Cluster

All `hydra gitops` commands validate the kubeconfig context automatically before connecting.

Warning: this validation is specific to `hydra gitops`. Because the future `hydra cluster` command surface will not use a local Hydra context, `HYDRA_CONTEXT` cannot be considered there.

The allowed kubeconfig contexts must be configured first in the Hydra values. Use the explicit validation command as a quick check that the current context is correctly recognized for the target cluster:

```bash
hydra gitops validate-current-context <cluster>
```

See [`hydra gitops validate-current-context`](cluster/validate-current-context.md).

## 3. Install ArgoCD On The Cluster

ArgoCD itself can be managed as a Hydra app:

```bash
hydra gitops apply in-cluster.argocd
```

Use [`hydra gitops apply`](cluster/apply.md) for details.

## 4. Inspect One App Before Applying

For a first pass, inspect values, render manifests locally, and diff against the cluster:

```bash
hydra local values <appId>
hydra local template <appId>
hydra gitops diff <appId>
```

See [`hydra local values`](local/values.md), [`hydra local template`](local/template.md), and [`hydra gitops diff`](cluster/diff.md).

## 5. Apply A Change

Once the diff looks correct, apply the desired state:

```bash
hydra gitops apply <appId>
```

For broader updates, use selectors such as `<cluster>.<rootApp>.*` or `<cluster>.**` as appropriate.

After the apply, use `hydra gitops status <appId>` for a cluster-based in-sync check and `hydra argocd status <appId>` for the real sync state that ArgoCD currently reports.

## 6. Pause ArgoCD During Maintenance

If you need temporary drift for maintenance, freeze reconciliation first:

```bash
hydra argocd sync prevent <appId>
# ... maintenance work ...
hydra argocd sync auto <appId>
```

See [`hydra argocd`](argocd/README.md).

## 7. Uninstall ArgoCD If Needed

If you need to remove ArgoCD from the cluster again:

```bash
hydra gitops uninstall in-cluster.argocd
```

See [`hydra gitops uninstall`](cluster/uninstall.md).

## Next Reading

- [`README.md`](README.md) for the full CLI reference and mental model
- [`hydra gitops apply`](cluster/apply.md) for normal deployment workflows
- [`hydra argocd`](argocd/README.md) for ArgoCD status and sync
- [`hydra gitops status`](cluster/status.md) for per-app cluster-based in-sync checks
- [`hydra gitops uninstall`](cluster/uninstall.md) for destructive operations
