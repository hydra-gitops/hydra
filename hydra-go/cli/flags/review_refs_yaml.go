package flags

// ReviewRefsYamlFlag selects YAML vs human-readable text for review finding stdout.
type ReviewRefsYamlFlag struct {
	Yaml bool
}

// WithReviewRefsYamlFlag is implemented by flag structs that support --yaml on review refs commands.
type WithReviewRefsYamlFlag interface {
	WithReviewRefsYamlFlag() *ReviewRefsYamlFlag
}
