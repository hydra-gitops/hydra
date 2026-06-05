# Click Flow Architecture

This document describes all user click paths that change navigation or selection state in Hydra UI. It maps each interactive element to its handler and the resulting state change.

## Key Concepts

- **Files tree clicks** — Non-entity nodes apply filters via `handleSidebarFilter`; entity leaves open the details page
- **Charts tree clicks** — Chart nodes set a `selectionSlot` with `appId` filters and navigate to the charts page
- **Graph clicks** — First tap selects (builds filters from entity identity); second tap opens details; Ctrl/Cmd appends to selection
- **Groups tree clicks** — Group label clicks parse the group key into field/value filters; checkboxes toggle graph expand/collapse only
- **Entity list clicks** — Row click opens the entity details page
- **Selection slot** — Transient state and single source for click-driven selection; can be pinned to persistent filter slots
- **Filter panel actions** — Pin selection to persistent slots, clear selection, toggle chip states

## Source Files

| File                                  | Purpose                                                                             |
| ------------------------------------- | ----------------------------------------------------------------------------------- |
| `src/App.tsx`                         | Core handlers: `handleSidebarFilter`, `handleGraphSelect`, `handleListEntitySelect` |
| `src/graphInteractionLogic.ts`        | `getEntityTapAction()` — select vs open-details decision                            |
| `src/components/FilesTreeView.tsx`    | Files tree click handling                                                           |
| `src/components/ChartsTreeView.tsx`   | Charts tree click handling                                                          |
| `src/components/TreePanel.tsx`        | Nodes tree and groups tree click handling                                           |
| `src/components/GraphPanel.tsx`       | Graph click handling                                                                |
| `src/components/EntityListPanel.tsx`  | Entity list row clicks                                                              |
| `src/components/FilterSlotsPanel.tsx` | Filter panel selection actions                                                      |

→ **Full details:** [details/click-flow.md](details/click-flow.md)
