# Graph Layout Architecture

Hydra UI layout details are split into focused subpages. This index remains the stable entry point for the topic and preserves the most important legacy headings from the former monolithic document.

## Detailed Pages

| Page                                                            | Description                                                                                              |
| --------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| [data-model-and-grouping.md](layout/data-model-and-grouping.md) | Graph data model, grouping configuration, references, reachability, grouping pipeline, and NSCS logic    |
| [cloning-and-colors.md](layout/cloning-and-colors.md)           | Color-rule model, clone rules, clone page behavior, and threshold-based auto-cloning                     |
| [algorithm-and-rendering.md](layout/algorithm-and-rendering.md) | Node geometry, recursive layout algorithm, Cytoscape rendering, graph interaction, and overall data flow |
| [tests.md](layout/tests.md)                                     | Vitest and golden-file coverage across grouping, cloning, graph models, colors, and layout               |
| [api-reference.md](layout/api-reference.md)                     | Exported types, functions, and constants grouped by source module                                        |

## Overview

See [Data Model and Grouping](layout/data-model-and-grouping.md#overview) for the architecture overview and processing order.

## Example Model (YAML)

See [Data Model and Grouping](layout/data-model-and-grouping.md#example-model-yaml) for the full example input model and resulting tree.

## Data Model

See [Data Model and Grouping](layout/data-model-and-grouping.md#data-model) for the full graph data model.

### Filters

See [Data Model and Grouping](layout/data-model-and-grouping.md#filters).

### Grouping and Tree

See [Data Model and Grouping](layout/data-model-and-grouping.md#grouping-and-tree).

#### Grouping Keys

See [Data Model and Grouping](layout/data-model-and-grouping.md#grouping-keys) and [grouping-keys.md](grouping-keys.md).

#### Color Configuration

See [Entity Cloning and Colors](layout/cloning-and-colors.md#color-configuration).

#### Grouping Pipeline

See [Data Model and Grouping](layout/data-model-and-grouping.md#grouping-pipeline).

##### Step 1: Entity ID Parsing (`parseEntityId`)

See [Data Model and Grouping](layout/data-model-and-grouping.md#step-1-entity-id-parsing-parseentityid).

##### Step 2: Entity Group Map (`buildEntityGroupMap`)

See [Data Model and Grouping](layout/data-model-and-grouping.md#step-2-entity-group-map-buildentitygroupmap).

##### Step 3: NSCS Map (`buildNscsMap`) — optional

See [Data Model and Grouping](layout/data-model-and-grouping.md#step-3-nscs-map-buildnscsmap--optional).

##### Step 4: Field Value Resolution (`getFieldValue`, `getFieldLabel`)

See [Data Model and Grouping](layout/data-model-and-grouping.md#step-4-field-value-resolution-getfieldvalue-getfieldlabel).

##### Step 5: Tree Building (`buildTree`)

See [Data Model and Grouping](layout/data-model-and-grouping.md#step-5-tree-building-buildtree).

#### Entity Cloning

See [Entity Cloning and Colors](layout/cloning-and-colors.md#entity-cloning).

##### per=entityGroup — clones in REFERENCING entity groups

See [Entity Cloning and Colors](layout/cloning-and-colors.md#perentitygroup--clones-in-referencing-entity-groups).

##### per=nscs — clones UNGROUPED at the namespace level

See [Entity Cloning and Colors](layout/cloning-and-colors.md#pernscs--clones-ungrouped-at-the-namespace-level).

##### Clone Page

See [Entity Cloning and Colors](layout/cloning-and-colors.md#clone-page).

##### Context Menu (cxtmenu)

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#context-menu-cxtmenu).

##### Entity Page (Tabbed)

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#entity-page-tabbed) and [rbac-display.md](rbac-display.md).

##### Auto-Clone (Threshold)

See [Entity Cloning and Colors](layout/cloning-and-colors.md#auto-clone-threshold).

#### NSCS Grouping Logic

See [Data Model and Grouping](layout/data-model-and-grouping.md#nscs-grouping-logic).

##### Strategy 1 – Group Peers (Single Namespace)

See [Data Model and Grouping](layout/data-model-and-grouping.md#strategy-1--group-peers-single-namespace).

##### Strategy 2 – Direct References (Multi-Namespace Group)

See [Data Model and Grouping](layout/data-model-and-grouping.md#strategy-2--direct-references-multi-namespace-group).

##### Strategy 3 – Group References (Cluster-Only Group)

See [Data Model and Grouping](layout/data-model-and-grouping.md#strategy-3--group-references-cluster-only-group).

##### Transitive Resolution (Iterative Convergence)

See [Data Model and Grouping](layout/data-model-and-grouping.md#transitive-resolution-iterative-convergence).

##### No Resolution

See [Data Model and Grouping](layout/data-model-and-grouping.md#no-resolution).

### Groups

See [Data Model and Grouping](layout/data-model-and-grouping.md#groups).

### Entities

See [Data Model and Grouping](layout/data-model-and-grouping.md#entities).

### References

See [Data Model and Grouping](layout/data-model-and-grouping.md#references) for reference direction semantics and the parse-time swap behavior.

### Reachability

See [Data Model and Grouping](layout/data-model-and-grouping.md#reachability).

### Expand/Collapse State

See [Data Model and Grouping](layout/data-model-and-grouping.md#expandcollapse-state).

### Node Geometry

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#node-geometry).

## Node Types

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#node-types).

## Layout Algorithm

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#layout-algorithm).

### Step 1: Measure label sizes

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-1-measure-label-sizes).

### Step 2: Compute positions (bottom-up) — `computeRecursiveLayout()` in `src/layoutLogic.ts`

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-2-compute-positions-bottom-up--computerecursivelayout-in-srclayoutlogicts).

#### Step 2.1: Layout function — `computeNodeLayout()`

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-21-layout-function--computenodelayout).

#### Step 2.2: Recursive bottom-up layout — `computeRecursiveLayout()`

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-22-recursive-bottom-up-layout--computerecursivelayout).

### Step 3: Render nodes to the DOM

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-3-render-nodes-to-the-dom).

### Graph Interaction

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#graph-interaction).

#### Selection Highlighting (Reachability)

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#selection-highlighting-reachability).

### Step 4: Fit-all zoom and hide loading screen

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#step-4-fit-all-zoom-and-hide-loading-screen).

## Data Flow

See [Algorithm and Rendering](layout/algorithm-and-rendering.md#data-flow).

## Tests

See [Layout Tests](layout/tests.md).

### Clone Tests (`src/__tests__/cloneLogic.test.ts`)

See [Layout Tests](layout/tests.md#clone-tests-srctestsclonelogictestts).

#### Core Clone Tests

See [Layout Tests](layout/tests.md#core-clone-tests).

#### Auto-Clone Tests

See [Layout Tests](layout/tests.md#auto-clone-tests).

### Filter Tests (`src/__tests__/filterLogic.test.ts`)

See [Layout Tests](layout/tests.md#filter-tests-srctestsfilterlogictestts).

### Tree Building Tests (`src/__tests__/treeLogic.test.ts`)

See [Layout Tests](layout/tests.md#tree-building-tests-srcteststreelogictestts).

### Expand/Collapse Tests (`src/__tests__/graphModel.test.ts`)

See [Layout Tests](layout/tests.md#expandcollapse-tests-srctestsgraphmodeltestts).

### Reference / Edge Tests (`src/__tests__/graphModel.test.ts`)

See [Layout Tests](layout/tests.md#reference--edge-tests-srctestsgraphmodeltestts).

### Layout Tests (`src/__tests__/layoutLogic.test.ts`)

See [Layout Tests](layout/tests.md#layout-tests-srctestslayoutlogictestts).

### Graph Node Computation Tests (`src/__tests__/graphModel.test.ts`)

See [Layout Tests](layout/tests.md#graph-node-computation-tests-srctestsgraphmodeltestts).

### End-to-End Data Flow Test (`src/__tests__/graphModel.test.ts`)

See [Layout Tests](layout/tests.md#end-to-end-data-flow-test-srctestsgraphmodeltestts).

### Grouping Key Tests (`src/__tests__/groupingKeyLogic.test.ts`)

See [Layout Tests](layout/tests.md#grouping-key-tests-srctestsgroupingkeylogictestts).

### Color Logic Tests (`src/__tests__/colorLogic.test.ts`)

See [Layout Tests](layout/tests.md#color-logic-tests-srctestscolorlogictestts).

#### `autoColor`

See [Layout Tests](layout/tests.md#autocolor).

#### `resolveColorFromRules`

See [Layout Tests](layout/tests.md#resolvecolorfromrules).

#### `resolveColorFromRules` with `gk:` field

See [Layout Tests](layout/tests.md#resolvecolorfromrules-with-gk-field).

#### Color Palette Structure

See [Layout Tests](layout/tests.md#color-palette-structure).

## API Reference

See [Layout API Reference](layout/api-reference.md).

### model.ts — Core Types

See [Layout API Reference](layout/api-reference.md#modelts--core-types).

### parseHydra.ts — YAML Parsing

See [Layout API Reference](layout/api-reference.md#parsehydrats--yaml-parsing).

### treeLogic.ts — Grouping & Tree Building

See [Layout API Reference](layout/api-reference.md#treelogicts--grouping--tree-building).

### filterLogic.ts — Filter Application

See [Layout API Reference](layout/api-reference.md#filterlogicts--filter-application).

### colorLogic.ts — Color Resolution

See [Layout API Reference](layout/api-reference.md#colorlogicts--color-resolution).

### cloneLogic.ts — Entity Cloning

See [Layout API Reference](layout/api-reference.md#clonelogicts--entity-cloning).

### groupingKeyLogic.ts — Custom Grouping Keys

See [Layout API Reference](layout/api-reference.md#groupingkeylogicts--custom-grouping-keys).

### graphModel.ts — Graph Node/Edge Computation

See [Layout API Reference](layout/api-reference.md#graphmodelts--graph-nodeedge-computation).

### layoutLogic.ts — Layout Computation

See [Layout API Reference](layout/api-reference.md#layoutlogicts--layout-computation).

### filesTreeLogic.ts — Files Tree

See [Layout API Reference](layout/api-reference.md#filestreelogicts--files-tree).
