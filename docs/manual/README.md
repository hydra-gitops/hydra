# Hydra User Manual

Hydra is a dependency-aware GitOps CLI for managing Kubernetes clusters at scale. It models relationships between resources, orchestrates deployments in the correct order, and provides full visibility into cluster state.

## Who Is This Manual For?

This manual is written for **Kubernetes administrators** who want to learn Hydra. It assumes familiarity with:

- Kubernetes resources (Deployments, Services, ConfigMaps, Secrets, CRDs, …)
- kubectl and kubeconfig
- Helm (charts, values, templating)
- ArgoCD (Applications, sync, reconciliation) if you want to use Hydra together with ArgoCD (this is optional)

If you are already managing clusters with kubectl and Helm, you are ready to start.

## Quick Navigation


| Goal                            | Start Here                                        |
| ------------------------------- | ------------------------------------------------- |
| Understand what Hydra is        | [Introduction](introduction/)                     |
| Learn by doing                  | [Tutorials](tutorials/)                           |
| Look up a command               | [Command Reference](commands/)                    |
| Understand a concept            | [Concepts](concepts/)                             |
| Follow a workflow step-by-step  | [Workflows](workflows/)                           |
| Configure Hydra                 | [Configuration](configuration/)                   |
| Understand refs / presets / CEL | [Refs](refs/) · [Presets](presets/) · [CEL](cel/) |
| See all value options           | [Values](values/)                                 |
| Migrate from kubectl/Helm       | [Migration](migration/)                           |


## Chapters

1. **[Introduction](introduction/)** — What Hydra is, architecture, installation
2. **[Tutorials](tutorials/)** — Hands-on guided learning (first steps → CI pipeline)
3. **[Concepts](concepts/)** — Core concepts: context, clusters, apps, repos, dependency graph
4. **[Values](values/)** — All configurable options in `global.hydra`
5. **[Refs](refs/)** — Dependency edges: types, parsers, tags, labels, attributes
6. **[Presets](presets/)** — Cluster infrastructure recognition
7. **[CEL](cel/)** — Common Expression Language in Hydra
8. **[Commands](commands/)** — Complete command reference (40+ commands)
9. **[Workflows](workflows/)** — Step-by-step operational procedures
10. **[Configuration](configuration/)** — HYDRA_CONTEXT, config.yaml, kubernetes contexts
11. **[Migration](migration/)** — Switching from kubectl and Helm

## Conventions

- `monospace` denotes commands, flags, file paths, or resource identifiers
- `<placeholder>` denotes values you must replace
- `[optional]` denotes optional arguments
- Examples use a fictional cluster named `prod` unless stated otherwise

