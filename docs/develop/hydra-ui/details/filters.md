# Predefined Filters & Filter Slots

## Overview

The filter system has two layers:

1. **Predefined Filters** (formerly "Filter Groups"): Reusable filter expression definitions created in the Settings page. Stored in `HydraUiState.filterGroups`.
2. **Active Filter Slots**: Runtime filter instances displayed in the Entity List panel. Each slot holds a local copy of an expression tree. Slots are OR-combined for entity filtering. Stored in `HydraUiState.activeFilterSlots`.

**Source files:**

- `src/model.ts` — `FilterExprNode`, `FilterGroupDefinition`, `ActiveFilterSlot` types
- `src/filterExprHelpers.ts` — Pure tree manipulation helpers (path-based get/set/remove/insert, flatten, etc.)
- `src/filterGroupLogic.ts` — Expression evaluation, matching, slot-based entity filtering, legacy migration
- `src/state.ts` — State persistence, parsing, migration from old `rows` format
- `src/components/SettingsPage.tsx` — Predefined filter editor with recursive DnD expression tree
- `src/components/FilterSlotsPanel.tsx` — Slot-based filter UI with view/edit modes, selection filter display
- `src/components/EntityListPanel.tsx` — Entity list with debounced search and column visibility

## Data Model

### FilterExprNode (Recursive Expression Tree)

```typescript
type FilterExprLeaf = {
  type: "filter";
  field: string; // e.g. "namespace", "kind", "tags"
  value: string; // e.g. "default", "Pod"
  negated?: boolean; // true = exclude (NOT), false/undefined = include
};

type FilterExprRef = {
  type: "ref";
  groupName: string; // reference to another predefined filter by name
  negated?: boolean; // true = invert all referenced filters
};

type FilterExprGroup = {
  type: "group";
  op: "and" | "or"; // logical operator combining children
  negated?: boolean; // true = NOT(children)
  children: FilterExprNode[];
};

type FilterExprNode = FilterExprLeaf | FilterExprRef | FilterExprGroup;
```text

### FilterGroupDefinition (Predefined Filter)

```typescript
type FilterGroupDefinition = {
  name: string; // unique display name
  description: string; // Markdown description
  root: FilterExprNode; // expression tree root
};
```

### ActiveFilterSlot

```typescript
type ActiveFilterSlot = {
  root: FilterExprNode; // local copy of expression tree
  editing: boolean; // false = view mode (chips), true = editor mode
  chipStates: Record<string, boolean>; // persisted slot-local map (legacy/backward-compat)
};
```text

- `root`: A local copy — modifying it does NOT affect the predefined filter definition.
- `chipStates`: Persisted on each slot, but runtime toggling is driven by a global chip state map in `App.tsx` (shared across selection + all slots).
- Predefined matching: Computed at render time via `matchPredefinedName(slot.root, filterGroups)` — compares tree structure only; `chipStates` and `editing` are ignored.

### negated Semantics

- **Editor** (SettingsPage + inline slot editor): Shows as include (normal) vs. exclude (negated)
- **View** (EntityListPanel view mode): Chips toggle between active and inactive via per-slot `chipStates`; the original negated/non-negated state is preserved

### Example: `A & !(B | C)`

```yaml
root:
  type: group
  op: and
  children:
    - type: filter
      field: namespace
      value: default
    - type: group
      op: or
      negated: true
      children:
        - type: filter
          field: kind
          value: Pod
        - type: filter
          field: kind
          value: Service
```

### State

```text
HydraUiState
├── ... (existing fields)
├── filterGroups           FilterGroupDefinition[]   Predefined filter definitions
└── activeFilterSlots      ActiveFilterSlot[]         Runtime filter slots (OR-combined)
```text

## Architecture

### Tree Manipulation Helpers (`filterExprHelpers.ts`)

All functions are pure/immutable — they return new objects, never mutate.

- `getNodeAtPath(root, path)` — Retrieve node at `path` (e.g. `[1, 0]` = root.children[1].children[0])
- `setNodeAtPath(root, path, newNode)` — Return new tree with node at path replaced
- `removeNodeAtPath(root, path)` — Remove node, auto-flatten single-child groups
- `removeNodeRawAtPath(root, path)` — Remove without flattening (used in DnD)
- `insertChildAtPath(root, parentPath, index, newNode)` — Insert child in group at position
- `flattenSingleChildGroups(node)` — Recursively replace groups with 1 child by that child
- `wrapInGroup(a, b, op)` — Create new group containing two nodes (drag-to-nest)
- `toggleOp(root, path)` — Switch AND↔OR at path
- `toggleNegated(root, path)` — Toggle negated flag at path
- `exprChipKey(node)` — Derive stable key for leaf node (used in chipStates maps)
- `collectChipKeys(node)` — Collect all leaf chip keys
- `adjustPathAfterRemoval(target, removed)` — Adjust path indices after sibling removal
- `adjustGapAfterRemoval(parentPath, insertAt, removed)` — Adjust gap insertion after removal
- `exprToString(node, labelFn?)` — Human-readable string representation

### Entity Filtering (`filterGroupLogic.ts`)

#### Slot-Based Entity Evaluation

```typescript
function entityMatchesSlots(
  slots: ActiveFilterSlot[],
  matchLeaf: (field: string, value: string) => boolean,
  allGroups: FilterGroupDefinition[],
  chipStatesOverride?: Record<string, boolean>,
): boolean;
```

- If no slots exist → all entities pass
- Slots are OR-combined: entity passes if ANY slot matches
- Within a slot, `evalExpr` recursively evaluates the expression tree
- `chipStatesOverride`: optional global chip-state map. Inactive chips (value `false`) are skipped — AND treats them as vacuous true, OR as vacuous false.
- `matchLeaf` callback encapsulates entity field resolution (tags special handling, grouping key maps, etc.)

#### Predefined Name Matching

```typescript
function matchPredefinedName(
  slotRoot: FilterExprNode,
  predefined: FilterGroupDefinition[],
): string | null;
```text

Deep structural comparison of expression trees. Returns the predefined filter's name if a match is found.

#### Legacy Functions (still available)

- `expandExprFlat` — Flat expansion of expression tree (for SettingsPage preview)
- `matchFilterGroups` — Match SearchFilter[] against predefined filters
- `addFilterGroupFilters` / `removeFilterGroupFilters` — Add/remove group filters from SearchFilter[]
- `migrateRowsToExpr` — Convert old `rows` format to expression tree

### URL Hash Navigation (`useHashNavigation.ts`)

Filter parameters (`f.`, `x.`, `n.`) have been removed from the URL hash. Filters are now managed exclusively via `activeFilterSlots` in localStorage. The hash only contains cluster name, view mode, and page navigation.

## UI Behavior

### Filter Slots Panel (Sidebar)

The `FilterSlotsPanel` component renders all filter slots in the sidebar's Filter accordion. It manages two categories of filters:

```text
┌─────────────────────────────────────────────────────────────────┐
│ 📍 Selection                                    [📌 pin] [✕]  │
│ [namespace:demo] AND [group:ungrouped]                          │
├─────────────────────────────────────────────────────────────────┤
│ [ K8s Core ]                                   [✏ edit] [✕]   │
│ [ns:default] AND NOT ([kind:Pod] OR [kind:Service])            │
│                         OR                                      │
│ Custom                              [✓ done] [↩ cancel] [✕]   │
│     AND ▼                                                       │
│ [kind:Job]                                                      │
│ [+ filter] [+ group ref…] [+ ( )]                              │
│                                                                  │
│ [+ Filter] [+ Predefined Filter ▼]                              │
└─────────────────────────────────────────────────────────────────┘
```

#### Selection Filter

A temporary filter created by selection actions (sidebar Files/Groups or graph node/group click). Displayed with a green background and `pi-map-marker` icon.

- **Pin** (`pi-thumbtack`): Converts the selection into a permanent global filter slot
- **Clear** (`pi-times`): Removes the selection filter
- When a selection filter is active, the Entity List shows **only** entities matching the selection (global filters are ignored)
- Clicking a group sets ALL parent groups as AND-combined filters (e.g., clicking `demo/ungrouped` → `namespace=demo AND group=ungrouped`)

#### Global Filter Slots

#### Global Chip Toggle Behavior

Chip activation is global across:

- Selection filter chips
- All active filter slots

Toggling a chip key (e.g. `namespace:default`) in one place updates its state everywhere. The global map is stored in `App.tsx` (`globalChipStates`) and passed to `FilterSlotsPanel`.

**Predefined Slot (view mode):**

- Name shown in header when `matchPredefinedName` finds a match
- Chips rendered with expression tree structure (AND/OR labels, parentheses, NOT)
- Chips toggle active/inactive via the global chip-state map
- Edit icon switches to edit mode (saves snapshot for Cancel)

**Predefined Slot (edit mode):**

- Full inline expression editor (add/remove filters, toggle AND/OR, NOT, sub-groups)
- **Done** (`pi-check`): Exits edit mode, keeps changes
- **Cancel** (`pi-undo`): Reverts to pre-edit state
- Name disappears if tree is modified and no longer matches a predefined filter

**Custom Slot (edit mode):**

- **Done** (`pi-check`): Exits edit mode
- **Cancel** (`pi-undo`): Reverts to pre-edit state
- **Save As** (`pi-save`): Opens name input → saves as new predefined filter group, exits edit mode
- Name appears if content matches a predefined filter

**Add Predefined Filter:** Dropdown of available predefined filters → clones root into new slot (view mode).

**Add Custom Filter:** Creates empty slot in edit mode.

**OR Hint:** Shown when more than one slot exists.

### Settings Page (Predefined Filter Editor)

#### Recursive Expression Editor

Each predefined filter card contains a recursive expression tree editor:

```text
┌─ K8s Core ────────────────────────────────────────────────┐
│ Name: [Kubernetes Core              ]  [trash]            │
│ Description: [Edit] [Preview]                              │
│                                                            │
│ [namespace:default]                                        │
│         AND ▼                                              │
│ NOT ┃ [kind:Pod]                                          │
│     ┃      OR ▼                                            │
│     ┃ [kind:Service]                                      │
│     ┃ [+ filter] [+ group ref…] [+ ( )]                  │
│     ┃ )  [trash]                                          │
│         AND ▼                                              │
│ [+ filter] [+ group ref…] [+ ( )]                        │
│                                                            │
│ Expanded: namespace=default AND NOT (kind=Pod OR kind=Svc) │
└────────────────────────────────────────────────────────────┘
```text

**Features:**

- **AND/OR Toggle**: Click on "AND ▼" / "OR ▼" between siblings to switch the parent group's `op`
- **Negation Toggle**: Click "NOT" label next to group brackets to toggle `negated`
- **Chip Mode**: Click chip to toggle include (enabled) / exclude (disabled/negated)
- **Add Filter**: Two-step flow (select field → select value) appends leaf to group
- **Add Group Ref**: Dropdown with earlier-defined groups (name + markdown description)
- **Add Sub-Group**: Creates empty nested group with opposite op
- **Drag & Drop**:
  - Drag chip to gap between siblings → reorder
  - Drag chip onto another chip → create new nested group wrapping both
  - Auto-flatten: single-child groups are automatically collapsed after operations
- **Remove**: Individual chips and groups can be deleted
- **Nested groups**: Visually indented with a vertical bracket line

### Description Editor

Each predefined filter has a Markdown description with Edit/Preview tabs:

- Edit tab: monospace textarea
- Preview tab: rendered Markdown via `marked`

## State Persistence

### Serialization (YAML)

```yaml
filterGroups:
  - name: Kubernetes Core
    description: Core namespace resources
    root:
      type: group
      op: and
      children:
        - type: filter
          field: namespace
          value: default
        - type: group
          op: or
          negated: true
          children:
            - type: filter
              field: kind
              value: Pod
            - type: filter
              field: kind
              value: Service

activeFilterSlots:
  - root:
      type: group
      op: and
      children:
        - type: filter
          field: namespace
          value: default
    editing: false
    chipStates:
      "kind:Pod": false
  - root:
      type: group
      op: and
      children:
        - type: filter
          field: kind
          value: Job
    editing: true
    chipStates: {}
```

### Legacy Format Auto-Migration

YAML files with the old `rows` format are automatically converted to expression trees during deserialization:

```yaml
# Old format (auto-migrated)
filterGroups:
  - name: Legacy
    rows:
      - - type: filter
          field: namespace
          value: default

# Becomes:
filterGroups:
  - name: Legacy
    root:
      type: filter
      field: namespace
      value: default
```text

## Data Flow

```text
HydraUiState (localStorage)
├── filterGroups: FilterGroupDefinition[]
│     ├── Settings Page (Predefined Filters tab)
│     │     └── Recursive tree editor (DnD, AND/OR toggle, NOT, add/remove)
│     │         Uses: filterExprHelpers.ts for tree mutations
│     └── FilterSlotsPanel "Save As" action → adds new definition
│
├── activeFilterSlots: ActiveFilterSlot[]
│     └── FilterSlotsPanel (Sidebar Filter accordion)
│           │
│           ├── Slot rendering (view mode / edit mode per slot)
│           │     ├── View: renderExprView (chips with global chipStates toggle)
│           │     └── Edit: inline expression editor with Done/Cancel/Save As
│           │
│           ├── Predefined matching: matchPredefinedName(slot.root, filterGroups)
│           │     └── Shows/hides predefined name in slot header
│           │
│           └── Add Predefined / Add Custom buttons
│
└── globalChipStates: Record<string, boolean>
      └── Shared chip activation map used by selection + all slots

App.tsx (runtime state, not persisted)
├── selectionSlot: ActiveFilterSlot | null
│     └── Set by sidebar selection actions (Files/Groups)
│         Pin → moves to activeFilterSlots; Clear → null
│
├── filteredEntities (useMemo)
│     └── allEntities filtered by activeFilterSlots (global filters)
│         Uses: entityMatchesSlots(slots, matchLeaf, allGroups, globalChipStates)
│
└── listFilteredEntities (useMemo)
      ├── If selectionSlot: allEntities filtered by selectionSlot ONLY (ignores global slots)
      └── If no selectionSlot: uses filteredEntities (global slots apply)
```

## Tests

### Expression Helper Tests (`src/__tests__/filterExprHelpers.test.ts`)

- `getNodeAtPath` — root, child, deep nested, invalid path
- `setNodeAtPath` — replace root, child, deep nested
- `removeNodeAtPath` — remove root, child, auto-flatten
- `removeNodeRawAtPath` — remove without flatten
- `insertChildAtPath` — insert at beginning, end, nested
- `flattenSingleChildGroups` — single child, multiple children, recursive, preserve negation
- `wrapInGroup` — creates two-child group
- `toggleOp` — AND→OR, nested, leaf no-op
- `toggleNegated` — set and unset
- `collectChipKeys` — all leaves, empty group
- `adjustPathAfterRemoval` — sibling before, after, nested, different subtree
- `adjustGapAfterRemoval` — adjust insert index
- `exprToString` — filter, negated, ref, AND group, nested, negated group, custom label
- `parseGroupKey` — single pair, multiple pairs (nested path), three levels, empty, colons-only, trailing odd segment
- `buildSelectionSlot` — empty (null), single filter (leaf), two filters (AND group), three filters (AND group with 3 children)

### Filter Group Logic Tests (`src/__tests__/filterGroupLogic.test.ts`)

- `expandExprFlat` — simple AND, deduplication, negated leaf, negated group, refs, forward refs, empty
- `matchFilterGroups` — AND, OR, negated leaf, negated group, NOT(A AND B), multiple groups, empty, inactive, deeply nested
- `addFilterGroupFilters` — add with modes, no duplicates, mode update, preserve others
- `removeFilterGroupFilters` — remove unshared, remove all, preserve unrelated, unknown group
- `entityMatchesSlot` — match, no match, chipStates (skip AND/OR), negated, refs, empty slot
- `entityMatchesSlots` — no slots, OR across slots, no match
- `matchPredefinedName` — match, no match, negated difference
- `migrateRowsToExpr` — single/multi rows, OR rows, negated, inactive drop, refs, empty

### Hash Navigation Tests (`src/__tests__/useHashNavigation.test.ts`)

- `parseHash` — empty, cluster, views, pages, tabs, old filter params ignored, invalid pages
- `buildHash` — list/graph, pages, tabs, roundtrips

### State Serialization Tests (`src/__tests__/state.test.ts`)

- filterGroups serialized with new root format
- Legacy rows format auto-migrated during deserialization
- activeFilterSlots serialized/deserialized with root, editing, chipStates
- Round-trip serialize/deserialize
