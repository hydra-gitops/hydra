# Tutorial 2: Inspect a Cluster

Learn to validate your cluster connection, view diffs, check status, and review resources.

## Validate Your Connection

Before running any cluster command, verify that your kubectl context matches the expected cluster:

```bash
hydra gitops validate-current-context prod
```

If this fails, check your kubeconfig and [configuration](../configuration/kubernetes-context.md).

## View Diffs

Compare the rendered desired state against the live cluster state:

```bash
hydra gitops diff prod.cluster-infra.ingress-nginx
```

This shows what would change if you applied now. Resources in sync show no diff.

### Filter Diffs with CEL

Focus on specific resource types:

```bash
hydra gitops diff prod.cluster-infra.ingress-nginx --include 'kind == "Deployment"'
```

## Check Status

See which apps are in sync and which are out of sync:

```bash
# Single app
hydra gitops status prod.cluster-infra.ingress-nginx

# All apps on prod
hydra gitops status 'prod.**'
```

## Review References

Inspect the dependency graph for an app against live cluster state:

```bash
hydra gitops review app prod.cluster-infra.ingress-nginx
```

Review an entire cluster (including unassigned resources):

```bash
hydra gitops review cluster prod
```

## Dump Live State

Fetch all live Kubernetes resources visible to Hydra:

```bash
hydra gitops dump prod
```

## Interactive Inspection

Launch the interactive TUI to browse the reference graph with live cluster data:

```bash
hydra gitops inspect prod
```

Navigate with arrow keys, expand/collapse nodes, view resource details.

## Next Steps

- [Tutorial 3: Deploy an App](03-deploy-app.md)
- [Commands: cluster diff](../commands/cluster/diff.md)
