# Tutorial 5: Bootstrap a Cluster

Learn to bootstrap a fresh Kubernetes cluster from scratch using Hydra.

## The Bootstrap Problem

A fresh cluster has no ArgoCD, no secrets operator, and no Hydra ConfigMaps. But these components depend on each other:

- ArgoCD needs TLS certificates (from cert-manager)
- Cert-manager needs CRDs applied first
- The secrets operator needs to be deployed before secrets can be injected

Hydra solves this with `--bootstrap` mode, which bypasses guards that normally prevent applying without full dependency resolution.

## Prerequisites

- A fresh Kubernetes cluster (e.g., provisioned via Talos/Terraform)
- kubectl access to the cluster
- HYDRA_CONTEXT configured
- Cluster configuration in the GitOps repository

## Step 1: Validate Context

```bash
hydra gitops validate-current-context prod
```

## Step 2: Bootstrap Apply

```bash
hydra gitops apply 'prod.**' --bootstrap
```

The `--bootstrap` flag:
- Skips bootstrap-guard validation
- Applies resources even when dependencies are not yet satisfied on the cluster
- Uses topological ordering to apply in the best possible sequence

## Step 3: Re-Apply Without Bootstrap

Once the core infrastructure is up, re-apply without the bootstrap flag:

```bash
hydra gitops apply 'prod.**'
```

This verifies that all dependencies are now satisfied.

## Step 4: Verify

```bash
# All apps should be in sync
hydra gitops status 'prod.**'

# System presets should match
hydra gitops system prod

# No unexpected untracked resources
hydra gitops untracked prod
```

## Step 5: Enable ArgoCD Sync

Once everything is stable, enable ArgoCD auto-sync:

```bash
hydra gitops sync auto 'prod.**'
```

From this point, ArgoCD takes over continuous reconciliation.

## Next Steps

- [Tutorial 6: CI Pipeline](06-ci-pipeline.md)
- [Workflow: Cluster Bootstrap](../workflows/cluster-bootstrap.md)
- [Concepts: Bootstrap](../concepts/bootstrap.md)
