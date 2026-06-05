# Chapter 2: Helm Basics

In chapter 1 you wrote Kubernetes YAML by hand. That works for simple cases, but real applications have dozens of YAML files with interconnected settings. Helm packages them into reusable **charts** that you can install, configure, upgrade, and uninstall with single commands.

## What is Helm?

Helm is the package manager for Kubernetes. Just like:

- `brew install nginx` installs nginx on your Mac
- `helm install ingress ingress-nginx/ingress-nginx` installs the ingress controller on your cluster

A Helm **chart** is a package containing:

- Templates (Kubernetes YAML with placeholders)
- Default values (settings you can override)
- Metadata (name, version, dependencies)

## Step 1: Add a Helm Repository

Helm repositories are like app stores. Add the official ingress-nginx chart repository
(maintained upstream, not the deprecated Bitnami catalog):

```bash
helm repo add ingress-nginx https://kubernetes.github.io/ingress-nginx
helm repo update
```

See what's available:

```bash
# Search for charts
helm search repo ingress-nginx/ingress-nginx
```

## Step 2: Install a Chart

Let's install the ingress-nginx controller using Helm:

```bash
# Create a namespace
kubectl create namespace helm-tutorial

# Install ingress-nginx
helm install my-ingress ingress-nginx/ingress-nginx -n helm-tutorial
```

What just happened:

1. Helm downloaded the ingress-nginx chart from the official repository
2. Rendered the templates with default values
3. Applied the resulting Kubernetes resources to the cluster

Check what was created:

```bash
# See the Helm release
helm list -n helm-tutorial

# See the Kubernetes resources it created
kubectl get all -n helm-tutorial
```

You should see a Deployment, ReplicaSet, Pods, and a Service — all created automatically by Helm.

## Step 3: Understand Values

Every chart has default values. You can see them:

```bash
helm show values ingress-nginx/ingress-nginx | head -50
```

This shows hundreds of settings. Key values live under `controller.*` (replicas, service type, image tag, etc.).

### Override Values on the Command Line

```bash
# Upgrade with custom values
helm upgrade my-ingress ingress-nginx/ingress-nginx -n helm-tutorial \
  --set controller.replicaCount=3 \
  --set controller.service.type=NodePort \
  --set controller.service.nodePorts.http=30081

# Check: you should now have 3 pods
kubectl get pods -n helm-tutorial
```

### Override Values with a File

For more complex configurations, use a values file. Create `my-values.yaml`:

```yaml
controller:
  replicaCount: 2
  service:
    type: NodePort
    nodePorts:
      http: 30081

resources:
  limits:
    cpu: 200m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Apply it:

```bash
helm upgrade my-ingress ingress-nginx/ingress-nginx -n helm-tutorial -f my-values.yaml
```

This is exactly what Hydra does — it manages values files at multiple levels and merges them together.

## Step 4: See What Helm Renders

The most important Helm command for understanding what's happening:

```bash
# Render templates WITHOUT installing (just show the YAML)
helm template my-ingress ingress-nginx/ingress-nginx -f my-values.yaml
```

This shows you the exact Kubernetes YAML that Helm would apply. This is what `hydra local template` does — it renders charts and shows you the result.

Compare with and without your values:

```bash
# Default values
helm template my-ingress ingress-nginx/ingress-nginx > default.yaml

# Your custom values
helm template my-ingress ingress-nginx/ingress-nginx -f my-values.yaml > custom.yaml

# See the differences
diff default.yaml custom.yaml
```

## Step 5: Inspect an Installed Release

```bash
# See the values used by the current installation
helm get values my-ingress -n helm-tutorial

# See ALL values (defaults + overrides)
helm get values my-ingress -n helm-tutorial --all

# See the rendered manifests that are currently deployed
helm get manifest my-ingress -n helm-tutorial

# See the release history
helm history my-ingress -n helm-tutorial
```

## Step 6: Create Your Own Chart

Let's create a minimal chart to understand the structure:

```bash
# Create a chart skeleton
helm create my-chart
```

This creates:

```text
my-chart/
├── Chart.yaml              # Chart metadata
├── values.yaml             # Default values
├── charts/                 # Dependencies (other charts)
├── templates/              # Kubernetes YAML templates
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   ├── hpa.yaml
│   ├── ingress.yaml
│   ├── NOTES.txt           # Post-install message
│   ├── _helpers.tpl        # Template helpers
│   └── tests/
│       └── test-connection.yaml
└── .helmignore
```

### Look at Chart.yaml

```bash
cat my-chart/Chart.yaml
```

```yaml
apiVersion: v2
name: my-chart
description: A Helm chart for Kubernetes
type: application
version: 0.1.0       # Chart version
appVersion: "1.16.0"  # Version of the app being deployed
```

### Look at a Template

```bash
cat my-chart/templates/deployment.yaml
```

You'll see Go template syntax: `{{ .Values.replicaCount }}`, `{{ .Release.Name }}`, etc. These placeholders are replaced with actual values when Helm renders the chart.

### Look at values.yaml

```bash
cat my-chart/values.yaml
```

The defaults. Everything under `.Values` in the templates comes from here (or from your override file).

### Render Your Chart

```bash
helm template my-release my-chart/
```

See how the template placeholders are replaced with values from `values.yaml`.

### Install Your Chart

```bash
helm install my-release my-chart/ -n helm-tutorial
kubectl get all -n helm-tutorial
```

## Step 7: Chart Dependencies

Charts can depend on other charts. This is how the project works — the root app chart depends on `infra_library`, and child app charts depend on upstream charts (like cert-manager depending on the official Jetstack chart).

Edit `my-chart/Chart.yaml` to add a dependency:

```yaml
apiVersion: v2
name: my-chart
version: 0.1.0
dependencies:
  - name: ingress-nginx
    version: "4.11.8"
    repository: "https://kubernetes.github.io/ingress-nginx"
    condition: ingress-nginx.enabled
```

Then:

```bash
cd my-chart
helm dependency update
cd ..
```

This downloads the nginx chart into `my-chart/charts/`. Now your chart includes nginx as a sub-chart. Values for the sub-chart go under the dependency name:

```yaml
# In your values.yaml
nginx:
  enabled: true
  replicaCount: 2
```

This is the **wrapper chart pattern** used throughout this project — each child app chart in `charts-repository/` wraps an upstream chart and adds defaults.

## Step 8: Uninstall

```bash
# Uninstall the releases
helm uninstall my-ingress -n helm-tutorial
helm uninstall my-release -n helm-tutorial

# Delete the namespace
kubectl delete namespace helm-tutorial

# Clean up local files
rm -rf my-chart my-values.yaml default.yaml custom.yaml deployment.yaml service.yaml
```

## What You Learned

- **Helm charts** package Kubernetes applications for easy installation
- **Values** configure charts — defaults can be overridden with files or `--set`
- **`helm template`** renders charts locally without deploying (like `hydra local template`)
- **Chart dependencies** let you compose applications from smaller pieces
- The **wrapper chart pattern** is how this project structures its apps

## Key Takeaways for Understanding Hydra

Hydra is essentially a sophisticated Helm wrapper that:

1. Manages **multiple values files** in a hierarchy and merges them automatically
2. Renders charts using `helm template` internally
3. Applies the rendered YAML to the cluster (like `kubectl apply`)
4. Adds dependency tracking, backup/restore, and ArgoCD integration on top

## Next Chapter

[Chapter 3: ArgoCD Setup](03-argocd-setup.md) — install ArgoCD on your cluster and see GitOps in action.
