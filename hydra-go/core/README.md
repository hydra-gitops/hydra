# hydra-go/core

Hydra business logic without CLI framework dependencies.

## Packages

### cel

CEL (Common Expression Language) evaluation for predicates and expressions.

- `entity_support.go` - CEL support for entity operations
- `env.go` - CEL environment setup
- `expression.go` - Expression compilation and evaluation
- `list_support.go` - CEL list operations
- `predicate.go` - Predicate evaluation
- `program.go` - CEL program execution
- `programs.go` - Program caching
- `ref_type.go` - Reference type support
- `service_support.go` - Service-related CEL functions
- `util_support.go` - Utility CEL functions
- `value_type.go` - Value type support

### commands

Business commands for Hydra operations.

- `backup.go` - Per-app backup create/restore using SopsSecret CRDs
- `cluster.go` - Cluster operations
- `delete.go` - Resource deletion
- `list_cluster.go` - List cluster resources
- `mark_as_selected.go` - Entity selection
- `namespace.go` - Namespace handling
- `order.go` - Entity ordering
- `render.go` - Manifest rendering
- `scope_info.go` - Scope information
- `visit.go` - Entity tree traversal

### entity

Entity representation for Kubernetes resources.

- `entities.go` - Entity collection operations
- `entities_compare.go` - Entity comparison
- `entities_group.go` - Entity grouping
- `entities_select.go` - Entity selection
- `entities_sort.go` - Entity sorting
- `entity.go` - Core entity type
- `entity_map.go` - Entity map operations
- `order.go` - Entity ordering
- `tools.go` - Entity utilities

### helm

Helm chart processing.

- `chart_cache.go` - Chart caching
- `chartdirectory.go` - Chart directory handling
- `clone.go` - Chart cloning
- `downloader.go` - Chart downloading
- `hydra_fallback_values.go` - Fallback values handling
- `manifest.go` - Manifest processing
- `render.go` - Template rendering
- `values.go` - Values merging

### hydra

Hydra core logic.

- `caches.go` - Internal caches
- `child_app.go` - Child application handling
- `cluster.go` - Cluster configuration
- `context.go` - Hydra context management
- `hydra.go` - Main Hydra interface
- `hydra_app.go` - Application interface
- `hydra_values.go` - Values processing
- `path_resolver.go` - Path resolution
- `root_app.go` - Root application handling
- `validation.go` - Configuration validation

### k8s

Kubernetes integration.

- `context.go` - Kubernetes context handling
- `crds.go` - Custom Resource Definitions
- `diff.go` - Resource comparison
- `validate.go` - Schema validation

### references

Reference handling between entities.

- `refs.go` - Reference collection and resolution
- `ref-parsers/` - YAML-based reference parser definitions

### sops

SOPS encryption for secrets.

```go
plaintext, err := sops.DecryptSopsFile(path)
err := sops.EncryptSopsFile(data, path)
```

### types

Hydra-specific types.

- `app_id.go` - Application identifier (`cluster.rootApp.childApp`)
- `bool.go` - Boolean type aliases (Direction, DryRun, etc.)
- `color_enum.go` - Color enum type
- `config.go` - Configuration interface
- `constants.go` - Directory constants
- `crd_mode_enum.go` - CRD mode enum
- `enum_type.go` - Enum type interface
- `hydra.go` - Hydra configuration types
- `k8s.go` - Kubernetes-related types
- `keys.go` - Entity key types
- `kubernetes.go` - GVK, GVR, ApiVersion types
- `labels.go` - Label types
- `network_mode_enum.go` - Network mode enum
- `refs.go` - Reference types
- `scope_info.go` - Scope information types
- `types.go` - Core types (HydraContext, ContextPath, etc.)
- `value.go` - Value interface and implementations

### values

Values processing.

- `values.go` - Values loading and merging

### view

Dependency view and analysis.

- `dependencies.go` - Dependency graph building

### yaml

YAML utilities.

- `printer.go` - Kubernetes object printing
- `yaml.go` - YAML serialization and lookup

### yq

YQ integration for YAML patching.

```go
patched, err := yq.Yq(yaml, ".metadata.name = \"new-name\"")
colored, err := yq.YamlStringColored(types.ColorYes, yaml)
```

## Design Principles

- **No Cobra dependencies** - Framework-agnostic
- **Testable without CLI** - All functions can be unit tested
- **Well-defined interfaces** - Interfaces for dependency injection
- **Depends only on base** - No dependency on cli
