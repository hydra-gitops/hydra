# Pipeline Foundations: Root App Structure

This page describes how root apps relate to child apps and how the charts repository is organized.

Back to [Pipeline: Foundations](../foundations.md).

## Root App and Child App Relationship

### Current State (Symlinks → to be replaced)

Each app group has a **root app** (e.g. `charts-repository/apps/demo/root/`) that
references all child apps. Currently symlinks to the `infra_library` exist:

```text
apps/demo/root/dev/charts/infra_library           -> shared/infra_library/dev
apps/cluster-infra/root/dev/charts/infra_library  -> shared/infra_library/dev
apps/cicd/root/dev/charts/infra_library           -> shared/infra_library/dev
apps/demo-infra/root/dev/charts/infra_library      -> shared/infra_library/dev
apps/argocd/root/dev/charts/infra_library         -> shared/infra_library/dev
```text

**Problem:** Symlinks are not correctly supported by all CI systems and Helm
tooling, and prevent clean version tracking.

### Target State (without Symlinks)

Symlinks will be removed. Instead, the `infra_library` is included as a
versioned dependency in the root app `Chart.yaml`:

```yaml
# apps/demo/root/dev/Chart.yaml (NEW)
apiVersion: v2
name: demo
type: application
version: 1.200.9-dev
dependencies:
  - name: libchart
    version: "1.0.0"
    repository: "file://../../../../shared/libchart/dev"
```

### Automatic Version Update in the Root App

When a child app is changed, the `release` pipeline must automatically:

1. Update the **child chart version number in the root app** `values.yaml`
2. Update the root app version
3. Commit everything and create Git tags
4. Only the build tag `build-<date><time>` triggers `publish`, which builds and uploads charts to the configured OCI registry
5. Tags must be signed by the CI pipeline

#### Example: nginx Is Updated in dev

The release pipeline detects changes in `charts-repository/apps/demo/ingress-nginx/dev/`:

1. ingress-nginx/dev/Chart.yaml: bump dependency version
2. ingress-nginx/dev/Chart.yaml: bump wrapper version to match dependency
3. root/dev/values.yaml update: set apps.ingress-nginx.version to new version number
4. root/dev/Chart.yaml update: bump version
5. Commit + Git tags:
   - demo-ingress-nginx-4.11.8-dev (child app, documentation only)
   - demo-root-1.0.1-dev (root app, documentation only)
   - build-202603051555 (sole trigger for `publish`)
6. `build-<date><time>` tags trigger `test` + `publish` → helm package + OCI upload

**Case A — new upstream (dependency / base change):** example tags
`demo-root-1.0.1-dev` when the root’s middle number moves.

1. `apps/demo/ingress-nginx/dev/Chart.yaml`:
   - `dependencies[].version`: "4.11.7" → "4.11.8"
   - `version`: "18.2.4-4-dev" → "4.11.8-dev"
2. `apps/demo/root/dev/values.yaml`:
   - `apps.ingress-nginx.version`: "18.2.4-4-dev" → "4.11.8-dev"
3. `apps/demo/root/dev/Chart.yaml`:
   - `version`: "1.0.0-dev" → "1.0.1-dev"

**Case B — re-release of the same wrapper base (extra counter only,** for
example `4.11.8-dev` → `18.2.5-1-dev` **):** the root `version` bumps the
**third** number, e.g. `1.0.0-dev` → `1.0.0-1-dev`, and the documentation
tag is `demo-root-1.0.0-1-dev` instead of `demo-root-1.0.1-dev`.

#### Root App values.yaml (Target Format)

```yaml
# apps/demo/root/dev/values.yaml (excerpt)
apps:
  nginx:
    namespace: demo
    enabled: true
    version: "4.11.8-dev"
  fluent-bit:
    namespace: demo
    enabled: true
    version: "0.55.0-dev"
  redis:
    namespace: demo
    enabled: true
    version: "5.2.452-dev"
  # ...
```text

### Release Flow with Root App

```text
Child app changed
       |
       v
  +-------------------------------+
  | 1. Set child app version      |  release
  |    version: x.y.z-<env>       |
  +---------------+---------------+
                  |
                  v
  +-------------------------------+
  | 2. Update root app            |  release
  |    values.yaml: child version |
  |    Chart.yaml: root version   |
  +---------------+---------------+
                  |
                  v
  +-------------------------------+
  | 3. Commit + Git tags          |  release
  |    <root>-<child>-<version>   |  (documentation)
  |    <root>-root-<version>      |  (documentation)
  |    build-<date><time>         |  (publish trigger)
  +---------------+---------------+
                  |
                  v
  +-------------------------------+
  | 4. test                       |  test (tag-triggered)
  |    helm lint + helm template  |
  +---------------+---------------+
                  |
                  v
  +-------------------------------+
  | 5. helm package               |  publish (tag-triggered)
  |    -> upload to Harbor        |
  +-------------------------------+
```

---

## Charts Repository Structure

### Current State

```text
charts-repository/
├── apps/
│   ├── argocd/                      # ArgoCD configuration
│   │   └── root/dev/
│   ├── cicd/                        # CI/CD infrastructure
│   │   ├── csi-driver-nfs/dev/
│   │   ├── gitlab-runner/dev/
│   │   └── root/dev/
│   ├── cluster-infra/               # Cluster infrastructure
│   │   ├── cert-manager/dev/
│   │   ├── dex/dev/
│   │   ├── external-dns/dev/
│   │   ├── ingress-nginx/dev/
│   │   ├── kube-prometheus-stack/dev/
│   │   ├── kyverno/dev/
│   │   ├── fluent-bit/dev/
│   │   ├── sops-secrets-operator/dev/
│   │   └── root/dev/
│   ├── demo/                         # Demo services
│   │   ├── service-auth/dev/
│   │   ├── service-ui/dev/
│   │   ├── service-config/dev/
│   │   ├── ...                      # (~30 services)
│   │   ├── root/dev/
│   │   └── shared/dev/
│   └── demo-infra/                   # Demo infrastructure
│       ├── postgres/dev/
│       ├── demo-kafka/dev/
│       ├── demo-clickhouse/dev/
│       ├── ...
│       └── root/dev/
└── shared/
    └── infra_library/dev/
```text

### Target Structure (with stage/prod, without symlinks)

```text
charts-repository/
├── .hydra-ci.yaml                   # Pipeline configuration
├── apps/
│   ├── demo/
│   │   ├── root/
│   │   │   ├── dev/
│   │   │   │   ├── Chart.yaml       # version: x.y.z-dev, dep: infra_library (OCI)
│   │   │   │   └── values.yaml      # apps with versions
│   │   │   ├── stage/
│   │   │   │   ├── Chart.yaml       # version: x.y.z-stage
│   │   │   │   └── values.yaml
│   │   │   └── prod/
│   │   │       ├── Chart.yaml       # version: x.y.z
│   │   │       └── values.yaml
│   │   ├── service-ui/
│   │   │   ├── dev/
│   │   │   │   ├── Chart.yaml       # version: 1.200.9-dev
│   │   │   │   ├── values.yaml
│   │   │   │   └── charts/
│   │   │   ├── stage/
│   │   │   │   ├── Chart.yaml       # version: 1.200.9-stage
│   │   │   │   ├── values.yaml
│   │   │   │   └── charts/
│   │   │   └── prod/
│   │   │       ├── Chart.yaml       # version: 1.200.9
│   │   │       ├── values.yaml
│   │   │       └── charts/
│   │   └── service-auth/
│   │       ├── dev/
│   │       ├── stage/
│   │       └── prod/
│   ├── cluster-infra/
│   │   ├── root/{dev,stage,prod}/
│   │   └── cert-manager/{dev,stage,prod}/
│   └── ...
└── shared/
    └── infra_library/{dev,stage,prod}/
```

### Chart Structure (Child App)

Typical **wrapper** chart with an OCI subchart dependency:

```yaml
# apps/demo/ingress-nginx/dev/Chart.yaml
apiVersion: v2
name: ingress-nginx
type: application
version: 4.11.8-dev
dependencies:
  - name: ingress-nginx
    version: "4.11.8"
    repository: "https://kubernetes.github.io/ingress-nginx"
```text

Charts with **no** `dependencies` (for example some cluster-infra overlays)
still use a semver `version` with the usual `-dev` / `-stage` suffix. The
`release` pipeline derives the next wrapper from that `version` when
`dependencies` is empty, using the same extra-counter rules as for dependency-driven charts.

### Chart Structure (Root App)

```yaml
# apps/demo/root/dev/Chart.yaml
apiVersion: v2
name: demo
type: application
version: 1.200.9-dev
dependencies:
  - name: libchart
    version: "1.0.0"
    repository: "file://../../../../shared/libchart/dev"
```

---
