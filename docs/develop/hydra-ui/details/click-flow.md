# Click Flow Architecture

## Overview

This document describes all user click paths that change navigation or selection state.

```mermaid
flowchart LR
    FilesTree --> handleSidebarFilter
    ChartsTree --> handleOpenChart
    GroupsTree --> handleSidebarFilter
    SearchTree --> handleListEntitySelect
    EntityList --> handleListEntitySelect
    GraphPanel --> handleGraphSelect

    handleSidebarFilter --> selectionSlot
    handleGraphSelect --> selectionSlot
    handleListEntitySelect --> setPageDetails["setPage(details)"]
    handleOpenChart --> selectionSlot
    handleOpenChart --> setPageCharts["setPage(charts)"]

    selectionSlot --> selectionMatchIds["selectionMatchIds (derived)"]
    selectionSlot --> listSelection["listFilteredEntities (selection-only)"]
```text

Core handlers in `src/App.tsx`:

- `handleSidebarFilter(filters)` -> set selection slot from sidebar filters.
- `handleGraphSelect(nodeId, addToSelection)` -> set/extend selection from graph clicks.
- `handleListEntitySelect(entityId)` -> open details page for an entity.

Graph-specific tap logic is separated into `src/graphInteractionLogic.ts` which exports `getEntityTapAction()` — determines whether a tap should "select" the node or "open-details" (when the node is already selected).

## Files Tree Clicks

Source: `src/components/FilesTreeView.tsx`

- Clicking a non-entity node applies `node.filterInfo` through `onFilterByFiles` -> `handleSidebarFilter`. Always opens list view (no chart-specific behavior).
- Clicking an entity leaf calls `onSelectEntity` -> `handleListEntitySelect` -> `setPage({ type: "details", nodeId })`.
- File search only filters the visible tree; it does not mutate persisted state.

## Charts Tree Clicks

Source: `src/components/ChartsTreeView.tsx`

- Clicking a chart node calls `onOpenChart(chartName)` -> `handleOpenChart` -> sets a `selectionSlot` with `appId` filters (OR-combined for all appIds of that chart name) + `setChartPageSelectedName(chartName)` + `setPage("charts")`.
- Clicking a navigable dependency (a dependency that exists as a chart) calls `onOpenChart(depName)` -> same flow as above.
- Clicking a non-navigable dependency does nothing.
- Chart search only filters the visible tree; it does not mutate persisted state.
- The selection filter causes `listFilteredEntities` to contain only entities matching the chart's appIds, so `ChartPage` shows only that chart (dropdown hidden).

## Nodes Tree (SearchTree) Clicks

Source: `SearchTree` in `src/components/TreePanel.tsx`

- Clicking an entity leaf opens details through `onSelectEntity` -> `handleListEntitySelect`.
- Group nodes only control tree expand/collapse in PrimeReact; they do not apply filters.

## Groups Tree Clicks

Source: `TreePanel` in `src/components/TreePanel.tsx`

- Clicking a group label parses the group key (`parseGroupKey`) and forwards filters to `handleSidebarFilter`.
- Clicking the checkbox toggles graph expand/collapse state only (`graphExpandedGroups`), no selection change.
- Group labels are bold when any descendant entity is currently selected (`selectionMatchIds` overlap).

## Graph Clicks

Source: `src/components/GraphPanel.tsx` + `handleGraphSelect` in `src/App.tsx` + `getEntityTapAction` in `src/graphInteractionLogic.ts`

- Background click -> clear selection (`selectionSlot = null`).
- Group node click -> parse `group:` key into field/value filters.
- Entity node click -> `getEntityTapAction()` decides:
  - First tap (node not yet selected) -> "select": build filters from entity identity:
    - namespaced: `namespace + kind + name`
    - cluster-scoped: `kind + name`
  - Second tap (node already selected) -> "open-details": navigate to entity details page.
- Ctrl/Cmd click -> append as OR branch to existing selection tree.
- Normal click -> replace selection with a new single branch.

## Entity List Clicks

Source: `src/components/EntityListPanel.tsx`

- Row click -> `onEntitySelect(entityId)` -> `handleListEntitySelect` -> details page.

## Filter Panel Selection Actions

Source: `src/components/FilterSlotsPanel.tsx` with handlers in `src/App.tsx`

- Pin selection -> move `selectionSlot` into persistent `activeFilterSlots`.
- Clear selection -> `selectionSlot = null`.
- Chip toggles update global chip state (`globalChipStates`), affecting slot evaluation.

## Selection and List Behavior

- `selectionSlot` is transient state and the single source for click-driven selection.
- `selectionMatchIds` is derived from `selectionSlot` and used for graph highlighting + groups tree bolding.
- The entity list has two modes:
  - no selection: shows entities filtered by global slots (`filteredEntities`)
  - selection active: shows entities matched by selection slot only (`listFilteredEntities`)
- The chart page uses `listFilteredEntities` to determine which charts to show, so it also responds to the selection slot. When a chart is clicked in the sidebar, the resulting selection filter narrows the chart dropdown to just that chart.
