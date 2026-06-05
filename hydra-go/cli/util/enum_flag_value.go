package util

import (
	"fmt"

	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type enumFlagValue[T comparable, E types.EnumType[T]] struct {
	value        T
	stringValue  string
	stringValues []string
	enumType     E
}

type e bool
type ets struct{}

func (ets) Stringify(v e) (string, error) {
	return "", nil
}

func (ets) Parse(s string) (e, error) {
	return false, nil
}

func (ets) Values() []e {
	return nil
}

func (ets) Valid(v e) bool {
	return true
}

var _ FlagValue[e] = (*enumFlagValue[e, ets])(nil)

func (s *enumFlagValue[T, E]) Type() FlagType {
	return FlagTypeEnum
}

func NewEnumFlagValue[T comparable, E types.EnumType[T]](
	enumType E,
) (*enumFlagValue[T, E], error) {
	values := enumType.Values()
	if len(values) == 0 {
		panic("values cannot be empty")
	}
	strings := make([]string, len(values))
	for i, opt := range values {
		str, err := enumType.Stringify(opt)
		if err != nil {
			return nil, fmt.Errorf("failed to stringify value: %v", err)
		}
		strings[i] = str
	}
	return &enumFlagValue[T, E]{
		value:        values[0],
		stringValue:  strings[0],
		stringValues: strings,
		enumType:     enumType,
	}, nil
}

func (e *enumFlagValue[T, E]) Value() T {
	return e.value
}

func (e *enumFlagValue[T, E]) SetValue(value T) error {
	stringValue, err := e.enumType.Stringify(value)
	if err != nil {
		return err
	}
	e.stringValue = stringValue
	e.value = value
	return nil
}

func (e *enumFlagValue[T, E]) String() string {
	return e.stringValue
}

func (e *enumFlagValue[T, E]) SetString(stringValue string) error {
	value, err := e.enumType.Parse(stringValue)
	if err != nil {
		return err
	}
	e.stringValue = stringValue
	e.value = value
	return nil
}

func (e *enumFlagValue[T, E]) Values() []T {
	return e.enumType.Values()
}

func (e *enumFlagValue[T, E]) StringValues() []string {
	return e.stringValues
}
