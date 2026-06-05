# Chapter 3: ArgoCD Setup

In this chapter you'll install ArgoCD on your Docker Desktop cluster, create Applications, and see GitOps in action — changes in Git automatically appear on the cluster.

## What is ArgoCD?

ArgoCD is a GitOps continuous delivery tool. It watches a Git repository and keeps your Kubernetes cluster in sync with whatever's defined in Git:

```text
You push a change to Git
        ↓
ArgoCD detects the change
        ↓
ArgoCD applies it to the cluster
        ↓
The cluster matches Git
```

## Step 1: Install ArgoCD

```bash
# Create the argocd namespace
kubectl create namespace argocd

# Install ArgoCD
kubectl apply -n argocd -f https://raw.githubusercontent.com/argoproj/argo-cd/stable/manifests/install.yaml

# Wait for all pods to be ready (this takes 1-2 minutes)
kubectl wait --for=condition=Ready pods --all -n argocd --timeout=120s

# Check that everything is running
kubectl get pods -n argocd
```

You should see pods like `argocd-server`, `argocd-repo-server`, `argocd-application-controller`, etc.

## Step 2: Access the ArgoCD UI

```bash
# Port-forward the ArgoCD server to localhost:8080
kubectl port-forward svc/argocd-server -n argocd 8080:443
```

Open your browser and go to: **<https://localhost:8080>**

Your browser will warn about the self-signed certificate — click through to continue.

### Get the Admin Password

```bash
# The initial admin password is stored in a secret
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```

Login with:

- **Username:** `admin`
- **Password:** (the output from the command above)

You should see the ArgoCD dashboard — currently empty because we haven't created any Applications.

## Step 3: Install the ArgoCD CLI (Optional)

```bash
brew install argocd

# Login to your ArgoCD instance
argocd login localhost:8080 --insecure --username admin --password $(kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d)
```

## Step 4: Create Your First Application

An ArgoCD Application tells ArgoCD: "Watch this Git path and deploy it to this cluster."

We'll use the ArgoCD example repository. Create a file called `guestbook-app.yaml`:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: guestbook
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: guestbook
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated:
      selfHeal: true
      prune: true
```

Let's understand each part:

| Field | Meaning |
| --- | --- |
| `source.repoURL` | The Git repository to watch |
| `source.path` | The directory within the repo containing the manifests |
| `source.targetRevision` | Which branch/tag to follow (HEAD = latest) |
| `destination.server` | The Kubernetes cluster to deploy to (in-cluster) |
| `destination.namespace` | The namespace to deploy into |
| `syncPolicy.automated` | Automatically sync when Git changes |
| `selfHeal: true` | If someone manually changes the cluster, revert to Git state |
| `prune: true` | Delete resources from the cluster that are no longer in Git |

Apply it:

```bash
kubectl apply -f guestbook-app.yaml
```

### Check in the UI

Go back to **<https://localhost:8080>** and refresh. You should see the `guestbook` application. Click on it to see:

- The sync status (Synced/OutOfSync)
- The health status (Healthy/Progressing/Degraded)
- All the Kubernetes resources it created
- A visual graph of the resources and their relationships

### Check with kubectl

```bash
# See the ArgoCD Application resource
kubectl get applications -n argocd

# See what was deployed in the guestbook namespace
kubectl get all -n guestbook
```

## Step 5: See GitOps in Action

ArgoCD is now watching the Git repository. If someone changes the manifests in that repository, ArgoCD will automatically sync the changes to your cluster.

Let's simulate a **drift** — a manual change on the cluster that doesn't match Git:

```bash
# Scale the guestbook deployment to 5 replicas manually
kubectl scale deployment guestbook-ui -n guestbook --replicas=5

# Check — you'll see 5 pods for a moment
kubectl get pods -n guestbook
```

Wait 10-20 seconds, then check again:

```bash
kubectl get pods -n guestbook
```

ArgoCD detected the drift (5 replicas vs the 1 replica defined in Git) and **self-healed** — it reverted back to 1 replica. This is the power of GitOps: the Git state is the truth, and ArgoCD enforces it.

## Step 6: Understand Application → Cluster Relationship

In the ArgoCD UI, click on the `guestbook` application and then click on "APP DETAILS." You'll see:

- **Source:** The Git repo and path
- **Destination:** The cluster and namespace
- **Sync Policy:** Automated with self-heal and prune
- **Sync Status:** Whether the cluster matches Git
- **Health Status:** Whether the application is running correctly

This is exactly what this project does at scale — but instead of one Application, it manages dozens of Applications across multiple clusters. That's where the App of Apps pattern comes in.

## Step 7: Create a Second Application

Let's create another application to see how ArgoCD handles multiple apps:

```yaml
# Save as helm-guestbook-app.yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: helm-guestbook
  namespace: argocd
spec:
  project: default
  source:
    repoURL: https://github.com/argoproj/argocd-example-apps.git
    targetRevision: HEAD
    path: helm-guestbook
  destination:
    server: https://kubernetes.default.svc
    namespace: helm-guestbook
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated:
      selfHeal: true
      prune: true
```

```bash
kubectl apply -f helm-guestbook-app.yaml
```

Now in the ArgoCD UI you'll see two applications. The `helm-guestbook` uses a Helm chart instead of plain YAML — ArgoCD handles both.

## Cleanup

```bash
# Delete the applications (ArgoCD will also delete the deployed resources)
kubectl delete application guestbook -n argocd
kubectl delete application helm-guestbook -n argocd

# Delete the namespaces
kubectl delete namespace guestbook
kubectl delete namespace helm-guestbook

# Clean up local files
rm -f guestbook-app.yaml helm-guestbook-app.yaml
```

Keep ArgoCD running — you'll need it for the next chapter.

## What You Learned

- **ArgoCD** watches Git and keeps the cluster in sync
- An **Application** resource connects a Git path to a cluster + namespace
- **Auto-sync** means changes in Git are applied automatically
- **Self-heal** means manual drift on the cluster is reverted automatically
- ArgoCD works with both **plain YAML** and **Helm charts**
- The ArgoCD **UI** provides a visual overview of all applications and their status

## Key Takeaways for Understanding This Project

In this project:

- ArgoCD runs on every cluster
- Instead of creating Applications manually (like we just did), they're generated automatically from the directory structure — this is the App of Apps pattern
- Hydra can install ArgoCD itself: `hydra gitops apply in-cluster.argocd`
- Hydra can control ArgoCD sync: `hydra gitops sync prevent/auto`

## Next Chapter

[Chapter 4: App of Apps Pattern](04-app-of-apps.md) — learn the pattern that this entire project is built on.
