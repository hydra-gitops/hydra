package hydra

// ClusterDefaultsPresetEffectiveTestFixture builds a minimal ClusterDefaultsPresetEffective for unit tests
// outside this package (CLI/action tests). Builtin predicate YAML content is not loaded; only
// id/default flags and merged predicates matter for CEL evaluation and report output.
func ClusterDefaultsPresetEffectiveTestFixture(
	id string,
	builtinDefaultEnabled bool,
	effectiveEnabled bool,
	predicates map[string]ClusterDefaultsPredicateEffective,
) ClusterDefaultsPresetEffective {
	return ClusterDefaultsPresetEffective{
		ID:          id,
		Enabled:     effectiveEnabled,
		Predicates:  predicates,
		BuiltinFile: builtinClusterDefaultsPresetFile{ID: id, DefaultEnabled: builtinDefaultEnabled},
	}
}
