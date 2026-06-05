package types

type EnumType[T comparable] interface {
	Stringify(T) (string, error)
	Parse(string) (T, error)
	Values() []T
	Valid(T) bool
}
