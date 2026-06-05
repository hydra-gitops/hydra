package types

import (
	"fmt"
	"slices"
)

type DiffMode int

const (
	DiffModeServer DiffMode = iota
	DiffModeRaw
)

var diffModeValues = []DiffMode{
	DiffModeServer,
	DiffModeRaw,
}

type diffModeEnumType struct{}

var DiffModeEnumType EnumType[DiffMode] = diffModeEnumType{}

var _ EnumType[DiffMode] = (*diffModeEnumType)(nil)

func (diffModeEnumType) Stringify(dm DiffMode) (string, error) {
	return dm.String(), nil
}

func (diffModeEnumType) Parse(dm string) (DiffMode, error) {
	switch dm {
	case "server":
		return DiffModeServer, nil
	case "raw":
		return DiffModeRaw, nil
	default:
		return -1, fmt.Errorf("invalid diff mode: %s, allowed values are: %v", dm, diffModeValues)
	}
}

func (diffModeEnumType) Values() []DiffMode {
	return diffModeValues
}

func (diffModeEnumType) Valid(dm DiffMode) bool {
	return slices.Contains(diffModeValues, dm)
}

func (dm DiffMode) String() string {
	switch dm {
	case DiffModeServer:
		return "server"
	case DiffModeRaw:
		return "raw"
	default:
		return fmt.Sprintf("DiffModeUnknown(%d)", dm)
	}
}
