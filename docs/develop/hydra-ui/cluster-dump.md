# Cluster Dump — UI Integration

The Cluster Dump feature allows the CLI (`hydra ui`) to export rendered manifests, Helm chart archives, and value files alongside the `hydra.yaml` dependency model. The UI uses these files to display Manifest and Template tabs on entity detail pages.

## Key Concepts

- **ClusterLoader** — Central data loading class (one instance per cluster) with URL fallback logic for `hydra.yaml`
- **Manifest tab** — Displays rendered Kubernetes YAML manifests via CodeMirror editor
- **Template tab** — Extracts and displays Go template source from `.tgz` chart archives
- **TGZ extraction** — Browser-side gzip decompression (pako) and tar parsing for chart archives
- **Cross-cluster loading** — Additional `ClusterLoader` instances for accessing data from other clusters (e.g. in-cluster ArgoCD Application CRs)
- **Export structure** — Multi-cluster directory layout with in-cluster consolidating root app entities from all clusters

## Source Files

| File                   | Description                                                                |
| ---------------------- | -------------------------------------------------------------------------- |
| `src/clusterLoader.ts` | `ClusterLoader` class with `loadHydraYaml`, `loadManifest`, `loadTemplate` |
| `src/tgzExtract.ts`    | TGZ decompression and tar file extraction                                  |
| `src/parseHydra.ts`    | YAML parsing including `manifestPath`                                      |
| `src/App.tsx`          | ClusterLoader instantiation and wiring                                     |

→ **Full details:** [details/cluster-dump.md](details/cluster-dump.md)
