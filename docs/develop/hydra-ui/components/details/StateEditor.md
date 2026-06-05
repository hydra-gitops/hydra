# StateEditor

**File:** `src/components/StateEditor.tsx`

## Function

Full-panel YAML editor for directly editing the application state. Built on CodeMirror 6 with YAML syntax highlighting, line wrapping, and keyboard shortcuts (Ctrl+S to save). Provides:

- **Save** — Apply changes and close
- **Apply** — Apply changes (stay open)
- **Reset** — Revert to current live state
- **Cancel** — Discard changes and close

Shows a parse error message when the YAML is invalid.

Also exports `ReadOnlyYamlViewer` — a simplified read-only variant used in the Defaults tab.

## Exports

- `StateEditor` — The editable YAML editor
- `ReadOnlyYamlViewer` — Read-only YAML display

## Props (StateEditor)

| Prop        | Type                        | Description                                     |
| ----------- | --------------------------- | ----------------------------------------------- |
| `stateYaml` | `string`                    | Current live state as YAML                      |
| `onApply`   | `(yaml: string) => boolean` | Try to apply YAML; returns false on parse error |
| `onClose`   | `() => void`                | Called on Cancel or Save                        |

## Props (ReadOnlyYamlViewer)

| Prop   | Type     | Description             |
| ------ | -------- | ----------------------- |
| `yaml` | `string` | YAML content to display |

## Used by

- `src/components/SettingsPage.tsx` — `StateEditor` in the "State (YAML)" tab; `ReadOnlyYamlViewer` in the "Defaults" tab
