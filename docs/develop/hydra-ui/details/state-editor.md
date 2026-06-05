# State Editor Architecture

## Overview

The State Editor is a YAML editor component embedded in the Settings Page's **State (YAML)** tab. It allows direct inspection and modification of the complete application state (`HydraUiState`) as YAML.

**Source file:** `src/components/StateEditor.tsx`

**Access:** Settings icon (⚙) → Settings Page → **State (YAML)** tab (`#<cluster>?page=settings&tab=state`)

```text
┌──────────────────────┬───┬──────────────────────────────────────────────┐
│ Sidebar              │ R │ Settings Page                                │
│                      │ e │ ┌──────────────────────────────────────────┐ │
│ (unchanged,          │ s │ │ [Filters][GroupKeys][Graph][Search]      │ │
│  still functional)   │ i │ │ [State (YAML)][Defaults][Reset]          │ │
│                      │ z │ ├──────────────────────────────────────────┤ │
│                      │ e │ │  1│ searchGrouping:                     │ │
│                      │   │ │  2│   - namespace                       │ │
│                      │ H │ │  3│   - entityGroup                     │ │
│                      │ a │ │  4│ graphGroupingConfig:                │ │
│                      │ n │ │  5│   topLevelLayout: horizontal        │ │
│                      │ d │ │  6│   levels:                           │ │
│                      │ l │ │  7│     - field: namespace              │ │
│                      │ e │ │  8│       layout: vertical              │ │
│                      │   │ │   │     ...                              │ │
│                      │   │ ├──────────────────────────────────────────┤ │
│                      │   │ │ [Save] [Apply] [Reset] [×]              │ │
│                      │   │ └──────────────────────────────────────────┘ │
└──────────────────────┴───┴──────────────────────────────────────────────┘
```text

## Technology

The editor uses [CodeMirror 6](https://codemirror.net/) with the following extensions:

| Package                      | Purpose                                                 |
| ---------------------------- | ------------------------------------------------------- |
| `codemirror` (`basicSetup`)  | Line numbers, bracket matching, undo/redo, search, etc. |
| `@codemirror/lang-yaml`      | YAML syntax highlighting and language support           |
| `@codemirror/theme-one-dark` | Dark theme (used when app is in dark mode)              |

## Buttons

The toolbar at the top of the editor provides four actions:

| Button    | Shortcut           | Enabled when | Action                                                                |
| --------- | ------------------ | ------------ | --------------------------------------------------------------------- |
| **Save**  | `Ctrl+S` / `Cmd+S` | Dirty        | Applies changes to live state → saves to localStorage → closes editor |
| **Apply** | —                  | Dirty        | Applies changes to live state (editor stays open, dirty flag resets)  |
| **Reset** | —                  | Dirty        | Reverts editor content to the current live state (discards edits)     |
| **Close** | —                  | Always       | Closes the editor without applying changes (edits are lost)           |

## Dirty Tracking

The editor tracks whether the user has made manual edits ("dirty" state):

```text
                    ┌──────────────┐
                    │  Not Dirty   │ ←── initial state
                    │  (clean)     │
                    └──────┬───────┘
                           │ user types in editor
                           ▼
                    ┌──────────────┐
                    │    Dirty     │
                    │  (modified)  │
                    └──────┬───────┘
                           │ Save / Apply / Reset
                           ▼
                    ┌──────────────┐
                    │  Not Dirty   │
                    │  (clean)     │
                    └──────────────┘
```

**Behavior by dirty state:**

| State                 | Save/Apply/Reset buttons | External state changes                             |
| --------------------- | ------------------------ | -------------------------------------------------- |
| **Not dirty** (clean) | Disabled (grayed out)    | Immediately reflected in the editor (live preview) |
| **Dirty** (modified)  | Enabled (active)         | **Not** shown in editor — user edits are preserved |

**Implementation detail:** Programmatic editor updates (live sync, reset) use a `suppressDirtyRef` flag to prevent the CodeMirror `updateListener` from marking the editor as dirty. Only user-initiated keystrokes set the dirty flag.

## Live Sync

When the editor is **not dirty**, it acts as a live preview of the current state:

- Changing a filter in the sidebar → editor shows updated `activeFilterSlots:` section
- Changing the theme → editor shows updated `theme:`
- Any other state change → immediately visible in the editor

This allows the user to observe how UI interactions translate to YAML state changes.

When the editor **is dirty**, external changes are ignored to protect the user's in-progress edits. Clicking **Reset** returns to the live state.

## Theme Support

The editor supports light and dark themes:

- **Light mode:** Default CodeMirror theme (light background)
- **Dark mode:** `oneDark` theme from `@codemirror/theme-one-dark`

Theme switching is handled by destroying and recreating the CodeMirror instance (CodeMirror 6 does not support dynamic theme changes). The editor content and cursor position are preserved across theme changes.

## Validation

When the user clicks **Save** or **Apply**, the YAML is parsed via `deserializeState()`. If parsing fails (invalid YAML syntax or unexpected structure), an error message is displayed in the toolbar and the action is aborted. The editor content is preserved so the user can fix the error.

## Read-Only Defaults Viewer

The **Defaults** tab (`#<cluster>?page=settings&tab=defaults`) shows the full `HydraUiState` serialized as YAML **including all default values**. This is useful for inspecting the complete effective configuration, especially when the State (YAML) tab only shows overrides.

**Component:** `ReadOnlyYamlViewer` (exported from `src/components/StateEditor.tsx`)

| Property            | Description                                                                       |
| ------------------- | --------------------------------------------------------------------------------- |
| Read-only           | The editor is not editable (`EditorState.readOnly`, `EditorView.editable(false)`) |
| Syntax highlighting | Full YAML highlighting via `@codemirror/lang-yaml`                                |
| Theme support       | Light / dark theme matching the main editor                                       |
| Live sync           | Always reflects the current full state (updates whenever the state changes)       |

**Data flow:**

```text
App.tsx: currentState ──serializeFullState()──→ fullStateYaml (prop)
  └─→ SettingsPage.fullStateYaml
        └─→ ReadOnlyYamlViewer.yaml
```text

## State Type Reference

The YAML content represents a `HydraUiState` object. In the **State (YAML)** tab, only non-default values appear in the YAML. An empty editor (or empty string) means all defaults. In the **Defaults** tab, all values including defaults are shown. See [state.md](state.md) for the complete type definition and all sub-states.
