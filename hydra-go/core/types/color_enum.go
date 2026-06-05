package types

import (
	"fmt"
	"slices"
)

type Color bool

const (
	ColorYes Color = true
	ColorNo  Color = false
)

type ColorEnum string

const (
	ColorEnumAuto   ColorEnum = "auto"
	ColorEnumAlways ColorEnum = "always"
	ColorEnumNever  ColorEnum = "never"
)

var colorValues = []ColorEnum{
	ColorEnumAuto,
	ColorEnumAlways,
	ColorEnumNever,
}

type colorEnumType struct{}

var ColorEnumType EnumType[ColorEnum] = colorEnumType{}

var _ EnumType[ColorEnum] = (*colorEnumType)(nil)

func (colorEnumType) Stringify(ce ColorEnum) (string, error) {
	return string(ce), nil
}

func (colorEnumType) Parse(s string) (ColorEnum, error) {
	switch s {
	case "auto":
		return ColorEnumAuto, nil
	case "always":
		return ColorEnumAlways, nil
	case "never":
		return ColorEnumNever, nil
	default:
		return "", fmt.Errorf("invalid color enum: %s, allowed values are: %v", s, colorValues)
	}

}

func (colorEnumType) Values() []ColorEnum {
	return colorValues
}

func (colorEnumType) Valid(ce ColorEnum) bool {
	return slices.Contains(colorValues, ce)
}
