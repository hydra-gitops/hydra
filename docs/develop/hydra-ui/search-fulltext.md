# Fulltext Search

The fulltext search page provides global, client-side text search across five resource types in the current cluster: manifests, templates, local values, merged values, and Helm chart values.

## Key Concepts

- **Five searchable sources** — Manifests, Templates, Local Values, Merged Values, Helm Chart Values (all enabled by default, individually togglable)
- **Case-insensitive substring matching** — Results grouped by file with 2 lines of context above and below
- **On-demand content loading** — Files loaded when search page opens, cached for the session
- **URL-persisted search state** — Query and sources stored in URL hash as `q=` and `sources=` parameters
- **Debounced input** — 300ms debounce on search input to avoid excessive re-computation
- **Pure search logic** — `searchInContent()` in `fullSearchLogic.ts` handles matching with context merging

## Source Files

| File                            | Purpose                                                |
| ------------------------------- | ------------------------------------------------------ |
| `src/fullSearchLogic.ts`        | `searchInContent()` pure search function               |
| `src/components/SearchPage.tsx` | Search page React component                            |
| `src/clusterLoader.ts`          | Bulk-load methods for all search sources               |
| `src/tgzExtract.ts`             | `extractAllTextFilesFromTgz()` for template extraction |
| `src/state.ts`                  | `SearchSourceType`, `FulltextSearchState` types        |
| `src/useHashNavigation.ts`      | Search page URL handling                               |

→ **Full details:** [details/search-fulltext.md](details/search-fulltext.md)
