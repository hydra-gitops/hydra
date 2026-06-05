# Tutorial 3: Deploy an App

Learn the standard apply workflow: dry-run, apply, verify.

## The Apply Workflow

Always follow this sequence:

1. **Diff** — See what would change
2. **Dry-run** — Validate the apply would succeed
3. **Apply** — Execute the deployment
4. **Verify** — Confirm the result

## Step 1: Review the Diff

```bash
hydra gitops diff prod.cluster-infra.ingress-nginx
```

Understand what will change before proceeding.

## Step 2: Dry-Run

```bash
hydra gitops apply prod.cluster-infra.ingress-nginx --dry-run
```

This performs server-side dry-run validation without actually modifying resources. It catches:
- Schema validation errors
- Admission webhook rejections
- Conflict detection

## Step 3: Apply

```bash
hydra gitops apply prod.cluster-infra.ingress-nginx
```

Hydra applies resources using **server-side apply** in topological order based on the dependency graph.

### Apply Options

```bash
# Replace instead of merge (for resources with immutable fields)
hydra gitops apply prod.cluster-infra.ingress-nginx --replace

# Control orphan handling
hydra gitops apply prod.cluster-infra.ingress-nginx --orphan-scale-down
hydra gitops apply prod.cluster-infra.ingress-nginx --no-orphan-scale-down
```

## Step 4: Verify

```bash
# Should show no diff
hydra gitops diff prod.cluster-infra.ingress-nginx

# Should show in-sync
hydra gitops status prod.cluster-infra.ingress-nginx
```

## Deploying Multiple Apps

```bash
# All cluster-infra apps
hydra gitops apply 'prod.cluster-infra.*'

# Everything on prod
hydra gitops apply 'prod.**'
```

Hydra respects topological order across apps.

## Next Steps

- [Tutorial 4: Uninstall an App](04-uninstall-app.md)
- [Workflow: Cluster Apply](../workflows/cluster-apply.md)
- [Commands: cluster apply](../commands/cluster/apply.md)
