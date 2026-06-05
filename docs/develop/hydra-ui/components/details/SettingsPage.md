# SettingsPage

**File:** `src/components/SettingsPage.tsx`

## Function

Full-page settings overlay using an edit-copy pattern. All editable tabs work on a local copy of the settings; changes are only committed to live state via explicit actions:

- **Save** — Apply + close
- **Apply** — Apply without closing
- **Revert** — Reset edit state to live state
- **Cancel** — Revert + close

An unsaved-changes guard prompts the user when navigating away with pending changes.

### Tabs

| Tab                | ID             | Editable    | Description                                                         |
| ------------------ | -------------- | ----------- | ------------------------------------------------------------------- |
| Grouping Keys      | `groupingKeys` | Yes         | Define custom grouping keys with field/value mappings               |
| Predefined Filters | `filterGroups` | Yes         | Manage named filter groups with expression tree editors             |
| Graph              | `graph`        | Yes         | Graph grouping levels, layout directions, colour rules, clone rules |
| Search             | `search`       | Yes         | Search tree grouping, leaf display, and search field configuration  |
| Appearance         | `appearance`   | Yes         | Editor font size slider (8–24px)                                    |
| State (YAML)       | `state`        | Own pattern | Direct YAML editor for the full application state                   |
| Defaults           | `defaults`     | No          | Read-only view of the full state including defaults                 |
| Reset              | `reset`        | No          | Button to clear all settings and reload                             |

## Exported Types

- `EditableSettings` — Bundle of all editable settings fields
- `SettingsPageHandle` — Imperative handle with `requestLeave()` for unsaved-changes guard

## Props

| Prop               | Type                                   | Description                                           |
| ------------------ | -------------------------------------- | ----------------------------------------------------- |
| `isDark`           | `boolean`                              | Current dark mode state                               |
| `onClose`          | `() => void`                           | Called when settings page is closed                   |
| `activeTab`        | `string`                               | Active tab ID from URL hash                           |
| `onTabChange`      | `(tab: string) => void`                | Called when the active tab changes                    |
| `settings`         | `EditableSettings`                     | Current live settings (for initialisation and revert) |
| `onApply`          | `(settings: EditableSettings) => void` | Called on Apply/Save with the edit state              |
| `filteredEntities` | `HydraEntity[]`                        | Filtered entities (for field value dropdowns)         |
| `allEntities`      | `HydraEntity[]`                        | All entities (for filter group editor)                |
| `hydraGroups`      | `HydraGroup[]`                         | Group definitions                                     |
| `hydraReferences`  | `HydraReference[]`                     | References (for graph settings)                       |
| `groupingKeyMaps`  | `Map<string, Map<string, string>>`     | Resolved grouping key maps                            |
| `edgeCounts`       | `EdgeCounts`                           | Edge count data (for auto-clone settings)             |
| `stateYaml`        | `string`                               | Current state as YAML (for State tab)                 |
| `fullStateYaml`    | `string`                               | Full state including defaults (for Defaults tab)      |
| `onApplyStateYaml` | `(yaml: string) => boolean`            | Apply YAML from State editor                          |
| `onResetSettings`  | `() => void`                           | Reset all settings                                    |

## Used by

- `src/App.tsx` — rendered when `page === "settings"`
