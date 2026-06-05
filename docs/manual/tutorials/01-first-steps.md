# Tutorial 1: First Steps

Learn to set up your Hydra context, render templates locally, and understand App IDs.

## Set Your Context

Hydra needs to know where your cluster configurations live:

```bash
export HYDRA_CONTEXT=/path/to/gitops-repository/clusters
```

Verify the context is valid by listing available clusters:

```bash
ls $HYDRA_CONTEXT
# prod  test  cicd
```

## Render Templates Locally

Render all templates for a single app without connecting to a cluster:

```bash
hydra local template prod.cluster-infra.ingress-nginx
```

This outputs the rendered Kubernetes manifests that Hydra would apply.

## Understand App IDs

Every application in Hydra has a unique **App ID** with the format:

```
<cluster>.<group>.<app>
```

Examples:
- `prod.cluster-infra.ingress-nginx` — ingress-nginx in the cluster-infra group on prod
- `test.demo.service-auth` — service-auth in the demo group on test

## Use Glob Patterns

Select multiple apps with wildcards:

```bash
# All root apps on prod
hydra local template 'prod.*'

# All apps in cluster-infra on prod
hydra local template 'prod.cluster-infra.*'

# Everything on prod (recursive)
hydra local template 'prod.**'
```

See [App ID Patterns](../commands/app-id-patterns.md) for the full pattern syntax.

## Inspect Rendered Values

See the computed Helm values for an app:

```bash
hydra local values prod.cluster-infra.ingress-nginx
```

## List Resource IDs

List all Kubernetes resource IDs that an app renders:

```bash
hydra local list prod.cluster-infra.ingress-nginx
```

## Next Steps

- [Tutorial 2: Inspect a Cluster](02-inspect-cluster.md)
- [Concepts: Context and Clusters](../concepts/context-and-clusters.md)
