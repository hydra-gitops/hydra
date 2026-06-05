# Tutorial 4: Uninstall an App

Learn to safely remove Hydra-managed resources with backup, uninstall, and verification.

## The Uninstall Workflow

1. **Backup** — Save secrets before removal
2. **Uninstall** — Remove resources in reverse dependency order
3. **Verify** — Confirm nothing is left behind

## Step 1: Create a Backup

Before uninstalling, back up any secrets managed by the app:

```bash
hydra gitops backup create prod.cluster-infra.cert-manager
```

Backups are SOPS-encrypted and stored in the GitOps repository.

## Step 2: Uninstall

```bash
hydra gitops uninstall prod.cluster-infra.cert-manager
```

Hydra removes resources in **reverse topological order** based on the dependency graph. Resources tagged with `[uninstall]` refs are removed; resources tagged `[uninstall-safe]` are removed without confirmation.

### Understanding Uninstall Behavior

- Resources without uninstall tags are **not removed** (orphaned intentionally)
- `[uninstall-force]` resources require `--force` to delete, or `--keep` to leave them behind and continue
- Finalizers configured via `uninstall-finalizer` are respected

## Step 3: Verify

Check that no orphaned resources remain:

```bash
hydra gitops untracked prod
```

This lists resources on the cluster that are not managed by any app and not matched by any preset.

## Partial Uninstall

You can uninstall individual apps without affecting other apps in the same group:

```bash
hydra gitops uninstall prod.demo.service-auth
```

Hydra will warn if other apps depend on the one you are removing.

## Next Steps

- [Tutorial 5: Bootstrap a Cluster](05-bootstrap-cluster.md)
- [Workflow: Cluster Uninstall](../workflows/cluster-uninstall.md)
- [Refs: Ref Tags](../refs/ref-tags.md)
