# Hands-On Tutorial

This tutorial takes you from zero Kubernetes knowledge to understanding how this project works, step by step. Each chapter builds on the previous one.

## Prerequisites

Before starting, make sure you have:

- **Docker Desktop** installed on your Mac with **Kubernetes enabled**
  - Open Docker Desktop -> Settings -> Kubernetes -> Enable Kubernetes -> Apply & Restart
  - Wait until the Kubernetes status indicator (bottom-left) shows green
- **kubectl** installed (`brew install kubectl` if not already present)
- **Helm** installed (`brew install helm`)
- **Go** 1.26+ installed (`brew install go`) — needed for chapter 5
- A **terminal** application

### Verify Your Setup

Run these commands to confirm everything is ready:

```bash
# Docker is running
docker version

# Kubernetes is running (Docker Desktop provides a single-node cluster)
kubectl cluster-info

# You should see something like:
# Kubernetes control plane is running at https://127.0.0.1:6443

# Check the node
kubectl get nodes
# NAME             STATUS   ROLES           AGE   VERSION
# docker-desktop   Ready    control-plane   ...   ...

# Helm is installed
helm version
```

If `kubectl get nodes` shows `docker-desktop` with status `Ready`, you're good to go.

## Tutorial Chapters

| # | Chapter | What You'll Learn | Time |
| --- | --- | --- | --- |
| 1 | [Kubernetes Basics](01-kubernetes-basics.md) | Pods, Deployments, Services, namespaces, kubectl | ~30 min |
| 2 | [Helm Basics](02-helm-basics.md) | Charts, values, templates, install/upgrade/uninstall | ~30 min |
| 3 | [ArgoCD Setup](03-argocd-setup.md) | Install ArgoCD, create Applications, see GitOps in action | ~45 min |
| 4 | [App of Apps Pattern](04-app-of-apps.md) | The pattern this project is built on | ~30 min |
| 5 | [Hydra Introduction](05-hydra-introduction.md) | Build the CLI, explore the directory structure, run commands | ~45 min |

### Total estimated time: ~3 hours

## How to Use This Tutorial

1. Read the [documentation](../getting-started/what-is-hydra.md) first for context
2. Follow each chapter in order — they build on each other
3. Type the commands yourself (don't just read them)
4. When something doesn't work, read the error message carefully — they're usually helpful
5. Use `kubectl describe <resource>` and `kubectl logs <pod>` to investigate problems

## Cleaning Up

After finishing the tutorial, you can clean up everything:

```bash
# Delete all tutorial resources
kubectl delete namespace tutorial
kubectl delete namespace argocd

# Or simply reset Docker Desktop Kubernetes:
# Docker Desktop -> Settings -> Kubernetes -> Reset Kubernetes Cluster
```

## Recommended Reading Order

```text
1. getting-started/what-is-hydra.md        ← What is this project?
2. getting-started/concepts.md             ← Learn the vocabulary
3. tutorial/01-kubernetes-basics.md        ← Hands-on: Kubernetes
4. tutorial/02-helm-basics.md              ← Hands-on: Helm
5. tutorial/03-argocd-setup.md             ← Hands-on: ArgoCD
6. tutorial/04-app-of-apps.md              ← Hands-on: The pattern
7. concepts/gitops-repository.md             ← How this repo uses the pattern
8. concepts/charts-repository.md             ← Where the apps live
9. operations/cluster-lifecycle.md         ← The full story
10. tutorial/05-hydra-introduction.md      ← Hands-on: Hydra CLI
```
