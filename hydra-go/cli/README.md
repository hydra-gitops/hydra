# hydra-go/cli

CLI implementation with Cobra framework.

## Packages

### action

CLI action handlers that call core functions.

- `cluster_cert_manager.go` - Cert-manager backup/restore actions
- `cluster_dump.go` - Cluster dump action
- `cluster_uninstall.go` - Uninstall actions
- `cluster_validate_current_context.go` - Context validation action
- `cluster_view.go` - Export actions
- `diff.go` - Diff action
- `config.go` (action) - Local config output
- `template.go` - Template action
- `values.go` - Values action

### cmd

Cobra commands.

- `cluster.go` - GitOps command group (`hydra gitops`)
- `cluster_only.go` - Reserved future cluster-only command (`hydra cluster`)
- `cluster_cert_manager.go` - Cert-manager subcommands
- `cluster_dump.go` - Dump command
- `cluster_uninstall.go` - Uninstall commands
- `cluster_validate_current_context.go` - Context validation command
- `export.go` - Export commands
- `define_flags.go` - Flag definition utilities
- `diff.go` - Diff command
- `config.go` - `hydra local config` command
- `root.go` - Root command
- `template.go` - Template command
- `values.go` - Values command

### flags

Flag definitions for CLI commands.

- `app.go` - App ID flag
- `cluster.go` - Cluster flag
- `color.go` - Color flag
- `config.go` - Config creation from flags
- `context.go` - Context flag
- `crd_mode.go` - CRD mode flag
- `dry_run.go` - Dry run flag
- `keep_server_fields.go` - Keep server fields flag
- `kubernetes_connection_allowed.go` - K8s connection flag
- `kubernetes_version.go` - K8s version flag
- `network_mode.go` - Network mode flag
- `predicates.go` - Predicate flags

### util

Cobra-specific utilities.

- `add_prerun.go` - PreRun hook utilities
- `bool_flag_value.go` - Boolean flag value
- `enum_flag_value.go` - Enum flag value
- `flag_builder.go` - Flag builder pattern
- `flag_value.go` - Flag value interface
- `string_flag_value.go` - String flag value

#### FlagBuilder

Builder pattern for type-safe flag definition:

```go
util.NewStringFlagBuilder(cmd, "default").
    Name("context").
    Short("c").
    Usage("Path to hydra context").
    Validate(validatePath).
    Build()
```

#### AddPreRun

Utilities for Cobra PreRun hooks:

```go
util.AddPreRun(cmd, func(cmd *cobra.Command, args []string) {
    // Setup code
})

util.AddPreRunE(cmd, func(cmd *cobra.Command, args []string) error {
    return validate()
})
```

## Usage

```bash
# Create cluster dump
hydra gitops dump production --hydra-context ./gitops

# Show diff
hydra gitops diff production.argocd --hydra-context ./gitops --color always

# Render template
hydra local template production.argocd --hydra-context ./gitops

# Show values
hydra local values production.monitoring.prometheus --hydra-context ./gitops

# Uninstall app
hydra gitops uninstall production.monitoring.prometheus --hydra-context ./gitops

# Export Hydra UI data
hydra local export ./hydra-export --hydra-context ./gitops
```

## Command Surfaces

- `hydra local` reads only the local Hydra context.
- `hydra gitops` uses the local Hydra context plus live cluster access.
- `hydra cluster` is a reserved, not-yet-implemented cluster-only surface.

## Design Principles

- **Cobra encapsulation** - All Cobra dependencies are isolated in this module
- **Thin commands** - Commands delegate to core functions via action handlers
- **Consistent flags** - Common flags are defined centrally in flags package
- **Testable** - Commands can be tested with mock inputs
