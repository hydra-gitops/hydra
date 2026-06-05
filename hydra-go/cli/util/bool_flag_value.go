package util

import "fmt"

type boolFlagValue[T ~bool] struct {
	value bool
}

var _ FlagValue[bool] = (*boolFlagValue[bool])(nil)

func NewBoolFlagValue[T ~bool](value T) FlagValue[T] {
	return &boolFlagValue[T]{value: bool(value)}
}

func (s *boolFlagValue[T]) Type() FlagType {
	return FlagTypeBool
}

func (s *boolFlagValue[T]) Value() T {
	return T(s.value)
}

func (s *boolFlagValue[T]) SetValue(v T) error {
	s.value = bool(v)
	return nil
}

func (s *boolFlagValue[T]) String() string {
	return fmt.Sprint(s.value)
}

func (s *boolFlagValue[T]) SetString(v string) error {
	s.value = v == "true"
	return nil
}

func (s *boolFlagValue[T]) Values() []T {
	return nil
}

func (s *boolFlagValue[T]) StringValues() []string {
	return nil
}
