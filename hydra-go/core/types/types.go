package types

type YamlString string

type ValuesMap = map[string]any

// HydraContext represents a hydra context path provided by the user via flag or environment variable
// this can be either an absolute or relative path
type HydraContext string

func (hc HydraContext) String() string {
	return string(hc)
}

// context path type used internally to represent resolved absolute paths
type ContextPath string

func (c ContextPath) String() string {
	return string(c)
}

type CelPredicate string
type CelExpression string

type RepoPath string
type AbsPath string
