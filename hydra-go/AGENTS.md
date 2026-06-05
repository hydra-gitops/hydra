# Agent Notes

## Testing

- Use `hydra/hydra-go/test.sh` to run tests for the Hydra project
- This script handles the proper test environment setup
- **IMPORTANT**: Only run `hydra/hydra-go/test.sh` once - do NOT add extra commands like `| head`, `2>&1`, or repeat the command
- Wait for the test script to complete before evaluating results

## Test Data Guidelines

- **`.expected.yaml` files are auto-generated** - do NOT add comments to these files
- Add comments explaining the test scenario to the **`.given.yaml`** files instead

## Resource model

- GitOps-facing commands should build template and/or live entities once and pass them to [`BuildResourceModel`](core/commands/resource_model.go).
- Local-only flows pass template entities; cluster-only flows pass cluster entities; GitOps flows pass both. Passing neither side is an error.
- Presets are normal preset apps such as `in-cluster.preset.coredns`; use `AppId.IsPresetApp()` / `IsPresetApp()` to skip preset apps where needed.
- Workload scope ([`LiveEntitiesInHydraWorkloadScope`](core/commands/inventory_workload_scope.go)) is an explicit follow-up step for uninstall planning rather than a separate model layer.
- When app assignment is needed, use [`ResourceModel.AssignedApp`](core/commands/resource_model.go), [`IdsForApp`](core/commands/resource_model.go), and [`AssignmentMetadata`](core/commands/resource_model.go) instead of duplicating ownership maps in callers.

## References Package

### Updating Test Data

After making changes to the ref parsers or the `Refs`/`RefDefinitions` functions, run the update script to regenerate the golden files:

```bash
./hydra/hydra-go/update_testdata.sh
```

### CEL Functions for Ref Parsers

The ref-parser YAML files use CEL expressions with these functions:

- `refBuilder()` - Creates a new RefDefinition builder
- `refBuilder().incoming(endpoint)` - Adds an incoming endpoint to the builder
- `refBuilder().outgoing(endpoint)` - Adds an outgoing endpoint to the builder
- `refBuilder().refType(name)` - Sets a non-default ref type (for example `regarding` for `events.k8s.io/v1` Event → subject edges)
- `id(gvk, namespace, name)` - Creates an ID endpoint
- `idString(gvk, namespace, name)` - Canonical Hydra id string for the same arguments as `id(...)` (use for attribute values that must match `To`)
- `ref(type, value)` - Creates a custom endpoint

Example:

```yaml
- "refBuilder().outgoing(id('v1/Secret', ns, ips.name))"
- "refBuilder().incoming(id('v1/Secret', ns, 'image-pull-secret'))"
- "refBuilder().incoming(id('v1/ServiceAccount', ns, name)).outgoing(id('v1/Secret', ns, secretName))"
- "refBuilder().incoming(ref('crd', 'some-crd')"
```

Built-in workload parsers emit `.tag('optional:ref')` when a Kubernetes field sets `optional: true` (env, envFrom, volume, projected volume). Startup/operator optional dependencies use `optional:startup` (`types.RefTagOptionalStartup`).
