# State Editor

The State Editor is a CodeMirror 6 YAML editor embedded in the Settings Page's "State (YAML)" tab. It allows direct inspection and modification of the complete application state (`HydraUiState`) as YAML.

## Key Concepts

- **CodeMirror 6 editor** — Full-featured YAML editor with syntax highlighting, line numbers, search, undo/redo
- **Dirty tracking** — Distinguishes user edits from programmatic updates; buttons enabled only when dirty
- **Live sync** — When not dirty, editor acts as live preview of current state changes
- **Action buttons** — Save (apply + close), Apply (keep open), Reset (revert to live state), Close (discard)
- **Validation** — YAML parsed via `deserializeState()` on Save/Apply; errors shown inline
- **Read-only defaults viewer** — Separate "Defaults" tab shows full state including all default values
- **Theme support** — Light/dark mode via CodeMirror `oneDark` theme

## Source Files

| File                             | Description                                                      |
| -------------------------------- | ---------------------------------------------------------------- |
| `src/components/StateEditor.tsx` | `StateEditor` and `ReadOnlyYamlViewer` components                |
| `src/state.ts`                   | `serializeState()`, `deserializeState()`, `serializeFullState()` |

→ **Full details:** [details/state-editor.md](details/state-editor.md)
