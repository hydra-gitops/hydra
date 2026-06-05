# Navigation Architecture

Hydra UI uses a page-based navigation model with sub-tabs within pages. The navigation state is encoded in the URL hash, enabling shareable links and browser history integration.

## Key Concepts

- **Six pages** — List (default), Graph, Charts, Details, Settings, Search — controlled by `page=` URL parameter
- **Sub-tabs** — Tab views within pages (e.g. Details/Relations/RBAC for entity page, Grouping Keys/Filters/Graph for settings)
- **URL hash format** — `#<cluster>?page=<page>&node=<id>&tab=<tab>&appId=<appId>`
- **Navigation hook** — `useHashNavigation()` provides `setPage`, `setTab`, `setSearchParams`, `setAppId`, `goBack`
- **History behavior** — Page changes push history entries; tab changes use `replaceState` (no clutter)
- **Settings edit-copy pattern** — Changes made to local copy, committed via Save/Apply; unsaved changes guard with dialog
- **Escape key** — Closes detail/settings/search pages, or toggles between list and graph

## Source Files

| File                              | Purpose                                       |
| --------------------------------- | --------------------------------------------- |
| `src/state.ts`                    | `PageType` type definition                    |
| `src/useHashNavigation.ts`        | URL hash parsing/building, navigation hook    |
| `src/App.tsx`                     | Page switching, sidebar, routing              |
| `src/components/EntityPage.tsx`   | Entity detail page with tabs                  |
| `src/components/SettingsPage.tsx` | Settings page with tabs and edit-copy pattern |
| `src/components/SearchPage.tsx`   | Fulltext search page                          |

→ **Full details:** [details/navigation.md](details/navigation.md)
