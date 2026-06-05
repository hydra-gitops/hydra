package util

type stringFlagValue[T ~string] struct {
	value string
}

var _ FlagValue[string] = (*stringFlagValue[string])(nil)

func NewStringFlagValue[T ~string](defaultValue T) FlagValue[T] {
	return &stringFlagValue[T]{value: string(defaultValue)}
}

func (s *stringFlagValue[T]) Type() FlagType {
	return FlagTypeString
}

func (s *stringFlagValue[T]) Value() T {
	return T(s.value)
}

func (s *stringFlagValue[T]) SetValue(v T) error {
	s.value = string(v)
	return nil
}

func (s *stringFlagValue[T]) String() string {
	return s.value
}

func (s *stringFlagValue[T]) SetString(v string) error {
	s.value = v
	return nil
}

func (s *stringFlagValue[T]) Values() []T {
	return nil
}

func (s *stringFlagValue[T]) StringValues() []string {
	return nil
}
