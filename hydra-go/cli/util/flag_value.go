package util

type FlagType int

const (
	FlagTypeString FlagType = iota
	FlagTypeBool
	FlagTypeEnum
)

type FlagValue[T any] interface {
	Type() FlagType
	Value() T
	SetValue(T) error
	String() string
	SetString(string) error
	Values() []T
	StringValues() []string
}
