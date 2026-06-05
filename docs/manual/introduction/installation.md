# Installation

## Prerequisites

- **Go** (1.21+) — Required to build the CLI
- **kubectl** — Kubernetes CLI, configured with cluster access
- **Helm** (3.x) — Used internally for template rendering
- **ArgoCD** — Deployed on the target cluster (for GitOps workflows)

## Building the Hydra CLI

```bash
cd hydra-go
go build -o hydra ./cli
```

Move the binary to a directory in your `$PATH`:

```bash
mv hydra /usr/local/bin/
hydra version
```

## Setting Up HYDRA_CONTEXT

Hydra needs to know where your cluster configurations live. Set the `HYDRA_CONTEXT` environment variable to point at the root of your GitOps repository's cluster directory:

```bash
export HYDRA_CONTEXT=/path/to/gitops-repository/clusters
```

Each subdirectory in this path represents one cluster (e.g., `prod`, `test`, `cicd`).

See [Configuration → HYDRA_CONTEXT](../configuration/hydra-context.md) for details.

## Configuring Kubeconfig Mapping

If you manage multiple clusters, create `~/.config/hydra/config.yaml` to map cluster directories to specific kubeconfig files:

```yaml
contexts:
  - path: /path/to/gitops-repository/clusters/prod
    config: ~/.kube/prod.conf
    name: prod-admin
  - path: /path/to/gitops-repository/clusters/test
    config: ~/.kube/test.conf
    name: test-admin
```

See [Configuration → config.yaml](../configuration/config-yaml.md) for the full reference.

## Verifying Connectivity

```bash
hydra gitops validate-current-context prod
```

This verifies that your current kubectl context matches the allowed contexts configured for the cluster. If successful, Hydra can communicate with the cluster API.

## Next Steps

- [Tutorial: First Steps](../tutorials/01-first-steps.md) — Start using Hydra immediately
- [Concepts: Context and Clusters](../concepts/context-and-clusters.md) — Understand the context model
