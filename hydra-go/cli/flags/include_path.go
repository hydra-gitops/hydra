package flags

// WithIncludePathFlag marks flag structs that support --include-path.
type WithIncludePathFlag interface {
	WithIncludePathFlag() *IncludePathFlag
}

// IncludePathFlag holds chart-relative path prefixes for filtering template files.
type IncludePathFlag struct {
	IncludePathPrefixes []string
}
