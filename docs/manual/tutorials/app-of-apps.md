# Chapter 4: App of Apps Pattern

In chapter 3 you created ArgoCD Applications one by one. That works for 2-3 apps, but this project has 30+ applications per cluster. Creating them manually would be tedious and error-prone.

The **App of Apps pattern** solves this: you create one "root" Application that uses a Helm chart to generate all the other Applications.

## The Concept

```text
Without App of Apps:
  You manually create:  App 1, App 2, App 3, ... App 30

With App of Apps:
  You create one root app → it generates all child apps automatically

  Root App (a Helm chart)
  └── templates/apps.yaml
      ├── generates → cert-manager Application
      ├── generates → ingress-nginx Application
      ├── generates → dex Application
      └── generates → ... (as many as you need)
```

## Step 1: Create the Root Chart Structure

Make sure ArgoCD is still running from chapter 3. Let's build a root app that generates child Applications.

```bash
mkdir -p app-of-apps/templates
```

Create `app-of-apps/Chart.yaml`:

```yaml
apiVersion: v2
name: my-root-app
version: 0.1.0
description: A root application that generates child applications
```

Create `app-of-apps/values.yaml`:

```yaml
# List of child apps to create
apps:
  nginx-one:
    enabled: true
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    namespace: nginx-one

  nginx-two:
    enabled: true
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: helm-guestbook
    namespace: nginx-two

  disabled-app:
    enabled: false
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    namespace: disabled-app
```

Create the template that generates ArgoCD Applications. This is the key file — `app-of-apps/templates/apps.yaml`:

```yaml
{{- range $name, $app := .Values.apps }}
{{- if $app.enabled }}
---
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: {{ $name }}
  namespace: argocd
spec:
  project: default
  source:
    repoURL: {{ $app.repoURL }}
    targetRevision: HEAD
    path: {{ $app.path }}
  destination:
    server: https://kubernetes.default.svc
    namespace: {{ $app.namespace }}
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated:
      selfHeal: true
      prune: true
{{- end }}
{{- end }}
```

This template **loops over the apps** in `values.yaml` and generates an ArgoCD Application for each one that has `enabled: true`.

## Step 2: Preview What It Generates

Before deploying, let's see what the template produces:

```bash
helm template my-root app-of-apps/
```

You should see two ArgoCD Application YAML documents (nginx-one and nginx-two). The `disabled-app` is skipped because `enabled: false`.

This is exactly what `hydra local template` does — render charts and show you the result.

## Step 3: Deploy the Root App

Instead of applying the rendered YAML directly, we'll create the root app as an ArgoCD Application that points to our local directory. For this tutorial, we'll apply the rendered output directly:

```bash
helm template my-root app-of-apps/ | kubectl apply -f -
```

Check ArgoCD:

```bash
# You should see the two child applications
kubectl get applications -n argocd

# Check the ArgoCD UI at https://localhost:8080
# You should see nginx-one and nginx-two applications
```

ArgoCD now manages both child applications. They'll sync from Git, self-heal, and show up in the UI.

## Step 4: Add a New App by Changing Values

The power of this pattern: to add a new app, you just add it to the values file.

Edit `app-of-apps/values.yaml` and add:

```yaml
apps:
  nginx-one:
    enabled: true
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    namespace: nginx-one

  nginx-two:
    enabled: true
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: helm-guestbook
    namespace: nginx-two

  disabled-app:
    enabled: false
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    namespace: disabled-app

  nginx-three:
    enabled: true
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    path: guestbook
    namespace: nginx-three
```

Re-render and apply:

```bash
helm template my-root app-of-apps/ | kubectl apply -f -
```

Check: a third Application (`nginx-three`) now appears in ArgoCD. You added a whole new application just by editing a YAML values file.

## Step 5: Disable an App

To remove an app, set `enabled: false` or remove it from values:

Edit `app-of-apps/values.yaml` and change `nginx-two` to `enabled: false`:

```yaml
  nginx-two:
    enabled: false
    # ...
```

Re-render and apply:

```bash
helm template my-root app-of-apps/ | kubectl apply -f -
```

The nginx-two Application is no longer generated. You'd need to manually delete it (or use `kubectl delete application nginx-two -n argocd`), since we're applying directly. In a real GitOps setup with ArgoCD managing the root app, it would be automatically pruned.

## Step 6: How This Maps to the Real Project

In the real project, the pattern is the same but scaled up:

| Tutorial concept | Real project equivalent |
| --- | --- |
| `app-of-apps/` directory | `gitops-repository/clusters/<group>/<cluster>/in-cluster/<root-app>/` |
| `values.yaml` with `apps:` | The root app's `values.yaml` with enabled/disabled apps |
| `templates/apps.yaml` | The `infra_library` template that generates ArgoCD Applications |
| Adding an app to values | Enable a child app in the cluster's `values.yaml` |
| `helm template` to preview | `hydra local template <appId>` |

The real `values.yaml` for a root app like `cluster-infra` looks like:

```yaml
cert-manager:
  enabled: true
ingress-nginx:
  enabled: true
kube-prometheus-stack:
  enabled: true
dex:
  enabled: true
fluent-bit:
  enabled: false
```

And the `templates/apps.yaml` (generated by `infra_library`) creates an ArgoCD Application for each enabled app, pointing to the corresponding chart in `charts-repository/`.

## Step 7: Understanding the Full Chain

Here's how it all connects in the real project:

```text
gitops-repository/clusters/test/example-dev/in-cluster/cluster-infra/
├── Chart.yaml          ← Points to charts-repository/apps/cluster-infra/root/dev/
├── values.yaml         ← Enables/disables child apps for this cluster
└── templates/
    └── apps.yaml       ← Generated by infra_library: creates ArgoCD Applications

Each generated ArgoCD Application points to:
    charts-repository/apps/cluster-infra/<child-app>/dev/
    ├── Chart.yaml      ← Points to upstream chart (e.g., jetstack/cert-manager)
    ├── values.yaml     ← Default values for the child app
    └── templates/      ← Optional extra templates
```

The values merge chain:

```text
charts-repository defaults  (what the chart looks like)
        ↓ overridden by
gitops-repository cluster values  (what this specific cluster needs)
        ↓ merged together
Final values used to render the child app
```

## Cleanup

```bash
# Delete the applications
kubectl delete application nginx-one nginx-two nginx-three -n argocd 2>/dev/null

# Delete the namespaces
kubectl delete namespace nginx-one nginx-two nginx-three 2>/dev/null

# Clean up local files
rm -rf app-of-apps
```

Keep ArgoCD running for the next chapter.

## What You Learned

- The **App of Apps pattern** uses one root app to generate many child apps
- A simple Helm template that **loops over values** creates ArgoCD Applications
- To add or remove apps, you just **edit the values file**
- This is **exactly how this project works** — scaled to 30+ apps across multiple clusters
- Hydra adds value on top: automatic value merging, dependency tracking, backup/restore

## Next Chapter

[Chapter 5: Hydra Introduction](05-hydra-introduction.md) — build the Hydra CLI and use it to explore the real project structure.
