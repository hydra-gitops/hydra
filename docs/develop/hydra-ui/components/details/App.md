# App

**File:** `src/App.tsx`

## Function

Root application component that orchestrates the entire Hydra UI. Manages all persistent state (unified YAML in localStorage), URL-based navigation, entity filtering pipeline, graph layout, clone logic, and renders the sidebar (with filter slots, charts tree, files tree, nodes/search tree, groups tree) alongside the main content area (graph, list, charts, details, settings pages).

## Props

None — this is the root component rendered by `ThemeWrapper`.

## Used by

- `src/ThemeWrapper.tsx` — wraps `App` with the theme context provider
- `src/main.tsx` — entry point, renders `ThemeWrapper` which contains `App`

## Key Responsibilities

- **State management:** All UI state fields are individual `useState` hooks, combined into `HydraUiState` and persisted to localStorage as YAML on every change.
- **Navigation:** Uses `useHashNavigation` hook for URL hash-based page/tab/node navigation with browser history support.
- **Entity filtering:** Multi-slot filter pipeline with selection slot, debounced filtering, and entity ↔ appId mapping.
- **Graph:** Computes tree structure, layout geometry, visible entities, collapsed groups, graph nodes/edges, and selection highlighting.
- **Cloning:** Applies clone rules to duplicate entities across grouping branches.
- **Chart integration:** `handleOpenChart` creates selection filters (including fallback to parent chart appIds for dependency charts) and navigates to the charts page.
