# gitops-repository Directory Structure

This is the annotated directory tree of the real `gitops-repository/clusters/` directory. Every file and folder is explained.

## Full Tree (using "example-dev" as the example cluster)

```text
gitops-repository/
├── clusters/
│   ├── test/                                    # Cluster group: test environments
│   │   ├── values.yaml                          # Values shared by ALL test clusters
│   │   │
│   │   ├── example-dev/                         # A specific cluster
│   │   │   ├── values.yaml                      # Cluster-level values (overrides test/ values)
│   │   │   │
│   │   │   ├── deprecated/                      # Old secrets no longer in use
│   │   │   │   ├── harbor-pull-secret.sops.yaml
│   │   │   │   └── ...
│   │   │   │
│   │   │   └── in-cluster/                      # Everything deployed to this cluster
│   │   │       ├── values.yaml                  # Values shared by all root apps
│   │   │       │
│   │   │       ├── argocd/                      ── ROOT APP: ArgoCD ──
│   │   │       │   ├── Chart.yaml               # Helm chart metadata + dependency on charts-repo
│   │   │       │   ├── .hydra/                  # Hydra local state (not applied to cluster; gitignore locally if needed)
│   │   │       │   │   └── cache/helm/          # Optional Helm render cache (root: cache.yaml + templates.yaml; child apps: cache-<name>.yaml + templates-<name>.yaml)
│   │   │       │   ├── values.yaml              # ArgoCD-specific values for this cluster
│   │   │       │   ├── backup-argocd-argocd-server-tls.sops.yaml  # Root-app TLS backup (SopsSecret; `hydra gitops backup`)
│   │   │       │   ├── apps/                    # Child-app static manifests only (not used for root-app backups)
│   │   │       │   └── templates/               # Kubernetes resources created by this root app
│   │   │       │       ├── argocd-client-secret.sops.yaml             # OAuth client secret
│   │   │       │       ├── github-gitops-private-key.sops.yaml        # Git SSH key
│   │   │       │       └── helm-repo-secret.sops.yaml                 # Helm registry credentials
│   │   │       │
│   │   │       ├── cluster-infra/               ── ROOT APP: Cluster Infrastructure ──
│   │   │       │   ├── Chart.yaml               # Depends on charts-repo cluster-infra chart
│   │   │       │   ├── values.yaml              # Enable/disable child apps, set versions
│   │   │       │   ├── apps/                    # Per-child-app secrets and config
│   │   │       │   │   ├── cert-manager/
│   │   │       │   │   │   ├── backup-cert-manager-letsencrypt-prod.sops.yaml  # Backed-up cert
│   │   │       │   │   │   └── hetzner-credentials.sops.yaml                   # DNS credentials
│   │   │       │   │   ├── dex/
│   │   │       │   │   │   ├── argocd-secret.sops.yaml
│   │   │       │   │   │   ├── backup-dex-dex-tls.sops.yaml
│   │   │       │   │   │   └── keycloak-client-secret.sops.yaml
│   │   │       │   │   └── sops-secrets-operator/
│   │   │       │   │       ├── clusterSecret.sops.yaml      # The operator's own encryption key
│   │   │       │   │       └── imagePullSecret.sops.yaml    # Credentials to pull images
│   │   │       │   └── templates/
│   │   │       │       └── apps.yaml            # Template that generates child ArgoCD Applications
│   │   │       │
│   │   │       ├── demo-infra/                   ── ROOT APP: Demo Infrastructure ──
│   │   │       │   ├── Chart.yaml
│   │   │       │   ├── values.yaml
│   │   │       │   ├── apps/
│   │   │       │   │   ├── demo-ingress/
│   │   │       │   │   │   ├── backup-demo-devicetunnel-tls.sops.yaml
│   │   │       │   │   │   └── backup-demo-ui-and-api-tls.sops.yaml
│   │   │       │   │   └── demo-secrets/
│   │   │       │   │       ├── clickhouse-user-clickhouse-operator.sops.yaml
│   │   │       │   │       ├── clickhouse-user-demo.sops.yaml
│   │   │       │   │       ├── dbsecret.sops.yaml
│   │   │       │   │       └── device-api.sops.yaml
│   │   │       │   └── templates/
│   │   │       │       └── apps.yaml
│   │   │       │
│   │   │       ├── demo/                         ── ROOT APP: Demo Application Services ──
│   │   │       │   ├── Chart.yaml
│   │   │       │   ├── values.yaml
│   │   │       │   ├── apps/
│   │   │       │   │   └── shared/
│   │   │       │   │       ├── cert-svc-devices.yaml     # TLS cert for devices service
│   │   │       │   │       ├── cert-svc-pairing.yaml     # TLS cert for pairing service
│   │   │       │   │       ├── default-sa.yaml           # Default ServiceAccount
│   │   │       │   │       └── issuer-svc-devices.yaml   # Certificate issuer
│   │   │       │   └── templates/
│   │   │       │       └── apps.yaml
│   │   │       │
│   │   │       └── cicd/                        ── ROOT APP: CI/CD ──
│   │   │           ├── Chart.yaml
│   │   │           ├── values.yaml
│   │   │           └── templates/
│   │   │               └── apps.yaml
│   │   │
│   │   ├── example-prod/                        # Another test cluster (same structure)
│   │   ├── staging/
│   │   └── preview/
│   │
│   ├── cicd/                                    # Cluster group: CI/CD
│   │   └── build-cluster/                       # CI/CD cluster (same structure as above)
│   │
│   ├── management/                              # Cluster group: management environments
│   │   ├── mgmt-dev/
│   │   └── mgmt-staging/
│   │
│   └── cloud/                                   # Cluster group: cloud-hosted clusters
│       ├── values.yaml
│       ├── shared-values-prod.yaml
│       └── poc/
│           ├── values.yaml
│           └── in-cluster/
│               └── ...
```

## Key File Types

### `.hydra/` under a root app

Hydra may create a `.hydra/` directory inside each **root app** folder (next to `Chart.yaml`). It holds **local cache data** such as serialized Helm render inputs and rendered manifests under `.hydra/cache/helm/`. This directory is not part of what ArgoCD applies; teams typically add `.hydra/` to `.gitignore` if they do not want cache files in Git.

### Chart.yaml

Every root app has a `Chart.yaml` that declares which chart from `charts-repository/` it uses:

```yaml
apiVersion: v2
name: cluster-infra
version: 0.1.0
dependencies:
  - name: cluster-infra
    version: "*"
    repository: "file://../../../../charts-repository/apps/cluster-infra/root/dev"
```

The `repository: "file://..."` path is a symlink or relative path to the shared chart in `charts-repository/`.

### values.yaml

Each level has its own `values.yaml` with settings that get merged. Example cluster-level values:

```yaml
global:
  hydra:
    path: apps/cluster-infra/root/dev
    stage: dev
    repository: https://github.com/org/example-gitops-repo
    revision: main
```

Important: Use `global.hydra.type` to mark hierarchy levels (`group/context/cluster/root-app/child-app`). You can stop parent lookup with `global.hydra.parent: false` on a level.

### templates/apps.yaml

The root app template that generates child ArgoCD Applications. This is the heart of the App of Apps pattern — one template creates all the child applications.

### *.sops.yaml

Encrypted secret files. They look like normal YAML but with encrypted values. Only decryptable with the correct age or GPG key.

### backup-*.sops.yaml

Backed-up secrets. These are runtime-generated secrets (like TLS certificates from Let's Encrypt) that were saved with `hydra gitops backup create` before an uninstall. They're stored with `suspend: true` so the SOPS operator doesn't process them — Hydra manages their lifecycle directly.

## How Hydra Reads This Structure

When you run `hydra gitops apply example-dev.**`:

1. Hydra finds `example-dev` under `clusters/test/example-dev/`
2. It merges values from `test/values.yaml` → `example-dev/values.yaml` → `in-cluster/values.yaml`
3. For each root app (argocd, cluster-infra, demo-infra, demo), it adds the root app's `values.yaml`
4. It renders the Helm chart referenced in `Chart.yaml`
5. The rendered templates produce Kubernetes resources that get applied to the cluster

## How to Identify Cluster App IDs

The directory structure maps directly to Hydra App IDs:

| Directory path | App ID |
| --- | --- |
| `clusters/test/example-dev/in-cluster/argocd/` | `in-cluster.argocd` |
| `clusters/test/example-dev/in-cluster/cluster-infra/` | `in-cluster.cluster-infra` |
| child app cert-manager under cluster-infra | `in-cluster.cluster-infra.cert-manager` |
| `clusters/test/example-dev/in-cluster/demo/` | `in-cluster.demo` |

## Next Steps

- [What is the charts-repository?](../concepts/charts-repository.md) — where the shared application packages live
- [Cluster Lifecycle](../operations/cluster-lifecycle.md) — the full journey from empty VMs to running applications
