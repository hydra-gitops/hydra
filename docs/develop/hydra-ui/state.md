# State Architecture

All persistent UI state in Hydra UI is managed through a single unified state object (`HydraUiState`), serialized to YAML and stored in one localStorage key (`hydra-state`). Default values are stripped from the YAML to keep it minimal.

## Key Concepts

- **Single localStorage key** — `hydra-state` stores all persistent state as YAML; no other keys or cookies used
- **Settings defaults cascade** — Code defaults → YAML defaults file (`hydra-ui-defaults.yaml`) → effective defaults; only user changes persisted
- **Sub-states** — Theme, filters, grouping keys, graph config, color/clone rules, sidebar, entity list, values view, editor font size
- **URL-based navigation** — Page, view, tab, entity ID stored in URL hash (not in localStorage)
- **Serialization** — `serializeState()` strips effective defaults; `deserializeState()` fills missing fields
- **State editor** — Raw YAML editor in Settings for direct state inspection/modification
- **Adding new settings** — Define default, add to serialization/deserialization, wire in App.tsx

## Source Files

| File                             | Purpose                                                                     |
| -------------------------------- | --------------------------------------------------------------------------- |
| `src/state.ts`                   | `HydraUiState` type, serialization, defaults cascade, persistence functions |
| `src/App.tsx`                    | State initialization, auto-save, external defaults loading                  |
| `src/components/StateEditor.tsx` | YAML state editor component                                                 |
| `src/useHashNavigation.ts`       | URL-based navigation state                                                  |

→ **Full details:** [details/state.md](details/state.md)
