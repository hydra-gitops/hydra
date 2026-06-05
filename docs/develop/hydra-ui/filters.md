# Predefined Filters & Filter Slots

The filter system provides two layers: reusable predefined filter definitions (expression trees) created in Settings, and runtime filter slots displayed in the sidebar that are OR-combined for entity filtering.

## Key Concepts

- **FilterExprNode** — Recursive expression tree with leaf filters, group references, and AND/OR/NOT logical groups
- **Predefined filters** — Reusable filter definitions stored in `HydraUiState.filterGroups`, editable via recursive DnD expression tree editor in Settings
- **Active filter slots** — Runtime filter instances with local expression tree copies, OR-combined for entity evaluation
- **Selection slot** — Transient filter from sidebar/graph clicks; can be pinned to persistent slots
- **Global chip states** — Shared chip activation map across selection and all filter slots
- **Legacy migration** — Old `rows` format auto-converted to expression trees during deserialization

## Source Files

| File                                  | Purpose                                                             |
| ------------------------------------- | ------------------------------------------------------------------- |
| `src/model.ts`                        | `FilterExprNode`, `FilterGroupDefinition`, `ActiveFilterSlot` types |
| `src/filterExprHelpers.ts`            | Pure tree manipulation helpers                                      |
| `src/filterGroupLogic.ts`             | Expression evaluation, slot-based entity filtering                  |
| `src/state.ts`                        | State persistence, legacy migration                                 |
| `src/components/SettingsPage.tsx`     | Predefined filter editor                                            |
| `src/components/FilterSlotsPanel.tsx` | Slot-based filter UI                                                |

→ **Full details:** [details/filters.md](details/filters.md)
