package types

// EnumType is a generic interface for type-safe enums
type EnumType[T comparable] interface {
	Stringify(T) (string, error)
	Parse(string) (T, error)
	Values() []T
	Valid(T) bool
}
