# Chapter 5: Hydra Introduction

Now that you understand Kubernetes, Helm, ArgoCD, and the App of Apps pattern, let's use the actual Hydra CLI to explore this project.

In this chapter you'll build the Hydra CLI, run it against the local repository, and understand how it ties everything together.

## Step 1: Build the Hydra CLI

```bash
# Navigate to the hydra-go directory
cd hydra/hydra-go

# Sync the Go workspace
go work sync

# Build the CLI
cd cli
go build -o hydra .

# Verify it works
./hydra --help

# Move back to the repo root
cd ../../..
```

You should see the Hydra help output listing all available commands.

For convenience, you can add it to your PATH or create an alias:

```bash
# Option 1: Create an alias for this session
alias hydra="$(pwd)/hydra/hydra-go/cli/hydra"

# Option 2: Copy to a directory in your PATH
cp hydra/hydra-go/cli/hydra /usr/local/bin/hydra
```

## Step 2: Set the Hydra Context

The Hydra context tells Hydra where to find the cluster definitions. In this project, each cluster group has its own context:

```bash
# Set the context to the "test" cluster group
export HYDRA_CONTEXT=$(pwd)/gitops-repository/clusters/test
```

Alternatively, you can pass it on every command with `--hydra-context`.

## Step 3: Explore the Directory Structure

Let's see what Hydra finds in this context:

```bash
# See the available clusters
ls gitops-repository/clusters/test/

# Output: example-dev  example-prod  example-user-dev  example-cluster  example2  example-cluster-2  values.yaml
```

Each directory under `test/` is a cluster. Let's explore `example-dev` since it has the most complete setup:

```bash
# See the root apps for example-dev
ls gitops-repository/clusters/test/example-dev/in-cluster/

# Output: argocd  cicd  cluster-infra  demo  demo-infra  values.yaml
```

These are the five root applications installed on the example-dev cluster.

## Step 4: Use `hydra local values`

The `local values` command shows you the final merged values for any application after all hierarchy levels are combined:

```bash
# See the merged values for the cluster-infra root app on example-dev
hydra local values in-cluster.cluster-infra --hydra-context gitops-repository/clusters/test/example-dev
```

This shows you which child apps are enabled (`cert-manager: enabled: true`, etc.) and all their configuration values after merging from:

1. `test/values.yaml` (group-level)
2. `example-dev/values.yaml` (cluster-level)
3. `in-cluster/values.yaml` (shared)
4. `cluster-infra/values.yaml` (root app level)

To see values for a specific child app:

```bash
hydra local values in-cluster.cluster-infra.cert-manager --hydra-context gitops-repository/clusters/test/example-dev
```

## Step 5: Use `hydra local template`

The `local template` command renders the Helm charts and shows you the Kubernetes YAML that would be deployed:

```bash
# Render the cluster-infra root app templates
hydra local template in-cluster.cluster-infra --hydra-context gitops-repository/clusters/test/example-dev
```

This produces the ArgoCD Application resources that the root app creates — one Application per enabled child app.

To see what a specific child app renders:

```bash
# Render cert-manager's templates
hydra local template in-cluster.cluster-infra.cert-manager --hydra-context gitops-repository/clusters/test/example-dev
```

This shows the actual Kubernetes resources (Deployment, Service, etc.) that cert-manager would create on the cluster.

## Step 6: Use `hydra local config`

The `config` command shows effective Hydra configuration: Helm `global.hydra` and any Hydra ConfigMaps (`hydra-gitops.org/hydra-config`) from the rendered chart:

```bash
hydra local config in-cluster.cluster-infra --hydra-context gitops-repository/clusters/test/example-dev
```

Under `global.hydra`, typical fields include:

- `stage` — the deployment stage (dev, prod, etc.)
- `repository` — the Git repository URL
- `revision` — the Git branch
- `path` — the path within the repository

ConfigMaps that ship `data.hydra` are merged into the printed `global.hydra` tree (see the manual for merge order).

## Step 7: Use `hydra local find`

The `local find` command lets you query rendered resources using CEL filters and project results with `--pick`:

```bash
# List Deployment names across all apps on example-dev
hydra local find in-cluster.** --hydra-context gitops-repository/clusters/test/example-dev \
  --include 'kind == "Deployment"' --pick 'name' --uniq

# List resource names in the cert-manager namespace
hydra local find in-cluster.** --hydra-context gitops-repository/clusters/test/example-dev \
  --include 'namespace == "cert-manager"' --pick 'name' --uniq
```

## Step 8: Compare Clusters

One of Hydra's strengths is that you can compare what different clusters would get:

```bash
# Values for cert-manager on example-dev
hydra local values in-cluster.cluster-infra.cert-manager \
  --hydra-context gitops-repository/clusters/test/example-dev

# Values for cert-manager on example-dev (if it exists)
hydra local values in-cluster.cluster-infra.cert-manager \
  --hydra-context gitops-repository/clusters/test/example-dev
```

The differences come from the cluster-specific `values.yaml` overrides — the chart itself is the same.

## Step 9: Understanding App IDs

By now you've seen the pattern for App IDs:

```text
in-cluster.cluster-infra                    → the root app
in-cluster.cluster-infra.cert-manager       → a child app
in-cluster.cluster-infra.*                  → all child apps under cluster-infra
in-cluster.**                               → everything on the cluster
```

The first part (`in-cluster`) refers to the cluster target. Root apps live directly under it, and child apps are nested another level deep.

```text
in-cluster/                     ← cluster target
├── argocd                      ← in-cluster.argocd (root app)
├── cluster-infra               ← in-cluster.cluster-infra (root app)
│   ├── cert-manager            ← in-cluster.cluster-infra.cert-manager (child app)
│   ├── ingress-nginx           ← in-cluster.cluster-infra.ingress-nginx (child app)
│   └── ...
├── demo-infra                   ← in-cluster.demo-infra (root app)
└── demo                         ← in-cluster.demo (root app)
```

## Step 10: Cluster Commands (Requires a Live Cluster)

The following commands need a live Kubernetes cluster connection. On your Docker Desktop cluster, you can try some read-only commands, but they won't show Hydra-managed resources since we haven't deployed through Hydra.

For reference, here's what you would use on a real cluster:

```bash
# Verify your kubectl context matches the cluster
hydra gitops validate-current-context example-dev

# See differences between Git and the live cluster
hydra gitops diff in-cluster.cluster-infra.*

# Deploy changes
hydra gitops apply in-cluster.cluster-infra.*

# Dump cluster state
hydra gitops dump example-dev
```

## Step 11: Export for Hydra UI

Hydra can export the dependency model for visualization in the web UI:

```bash
# Export the dependency model
hydra export ./hydra-export \
  --hydra-context gitops-repository/clusters/test/example-dev \
  --helm-network-mode offline
```

This creates a directory with `.hydra.yaml` and other files that the Hydra UI can load.

To run the UI:

```bash
cd hydra/hydra-ui
yarn install
yarn dev --port 5173
```

Open **<http://localhost:5173>** and load the exported data. You'll see an interactive dependency graph showing all resources and their relationships.

## What You Learned

- The **Hydra CLI** works against the local filesystem — no cluster needed for many commands
- `hydra local values` shows the **final merged values** after all hierarchy levels
- `hydra local template` renders charts the same way Helm does, but with Hydra's value merging
- `hydra local config` shows Hydra metadata and Hydra ConfigMaps from the render
- `hydra local find` lets you query resources with CEL filters and `--pick` projections
- **App IDs** map directly to the directory structure
- Cluster commands (`diff`, `apply`, `dump`, etc.) require a live Kubernetes connection

## How Everything Connects

Now you've seen the complete picture:

```text
Chapter 1: Kubernetes
  You learned pods, deployments, services — the building blocks

Chapter 2: Helm
  You learned to package them into charts with configurable values

Chapter 3: ArgoCD
  You learned to keep the cluster in sync with Git automatically

Chapter 4: App of Apps
  You learned the pattern that generates applications from a values file

Chapter 5: Hydra (this chapter)
  You saw how Hydra combines all of this:
  - Helm charts (charts-repository/)
  - Values hierarchy (gitops-repository/)
  - ArgoCD integration (automatic sync management)
  - CLI tools (template, values, diff, apply)
  - Web UI (dependency visualization)
```

## Cleanup

```bash
# Remove the ArgoCD installation from earlier chapters
kubectl delete namespace argocd

# Remove Hydra export
rm -rf hydra-export

# Reset Docker Desktop Kubernetes if you want a clean slate:
# Docker Desktop -> Settings -> Kubernetes -> Reset Kubernetes Cluster
```

## Next Steps

- Read the [CLI Reference](../README.md) for detailed command documentation
- Explore the [gitops-repository structure](../gitops-repository/directory-structure.md) in depth
- Read about [cluster lifecycle](../operations/cluster-lifecycle.md) to understand the full production workflow
- Try the [Hydra UI](../../hydra-ui/README.md) to visualize dependencies
