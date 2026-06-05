# Chart Page

The Chart Page is a dedicated view for displaying Helm chart metadata (versions, dependencies, values, manifests, templates, and ArgoCD Application CRs) for the currently filtered entities. It is accessible via the Charts button in the top navigation or by clicking a chart in the sidebar.

## Key Concepts

- **Five tabs** — Details (versions, dependencies, Chart.yaml), Values (tree + merged preview), Manifests, Templates, App (ArgoCD CR)
- **Chart matching** — Charts are matched to filtered entities via `appId`; dropdown shown when multiple charts match
- **Values provenance** — Merged values annotated with source-file origin, unnecessary-override detection, and global clone detection
- **Sub-chart export** — hydra-go recursively exports sub-chart dependencies for transitive dependency tree display
- **Cross-cluster loading** — ArgoCD Application CRs loaded from in-cluster export when viewing other clusters
- **Filter interaction** — Chart page respects active filter pipeline; Chart.yaml content is never affected by filters

## Source Files

| File                                 | Description                                               |
| ------------------------------------ | --------------------------------------------------------- |
| `src/chartPageLogic.ts`              | Matching, grouping, parent-finding, value file tree logic |
| `src/argocdAppLookup.ts`             | Matches chart appIds to ArgoCD Application entities       |
| `src/components/ChartPage.tsx`       | Chart page component with five tabs                       |
| `src/components/ArgocdAppView.tsx`   | ArgoCD Application CR display                             |
| `src/components/YamlHighlighter.tsx` | Read-only YAML syntax highlighting                        |
| `src/valuesProvenance.ts`            | Provenance analysis functions                             |

→ **Full details:** [details/chart-page.md](details/chart-page.md)
