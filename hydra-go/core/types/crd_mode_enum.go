package types

import (
	"fmt"
	"slices"
)

type CrdMode int

const (
	CrdModeError CrdMode = iota
	CrdModeIgnore
	CrdModeSilent         // like ignore but without warning logs
	CrdModeIgnoreOptional // like silent but errors on required GVKs
	CrdModeKeepUnknown    // keep unknown GVKs unchanged (internal only)
)

var crdModeValues = []CrdMode{
	CrdModeError,
	CrdModeIgnore,
}

type crdModeEnumType struct{}

var CrdModeEnumType EnumType[CrdMode] = crdModeEnumType{}

var _ EnumType[CrdMode] = (*crdModeEnumType)(nil)

func (crdModeEnumType) Stringify(cm CrdMode) (string, error) {
	return cm.String(), nil
}

func (crdModeEnumType) Parse(cm string) (CrdMode, error) {
	switch cm {
	case "error":
		return CrdModeError, nil
	case "ignore":
		return CrdModeIgnore, nil
	default:
		return -1, fmt.Errorf("invalid crd mode enum: %s, allowed values are: %v", cm, crdModeValues)
	}
}

func (crdModeEnumType) Values() []CrdMode {
	return crdModeValues
}

func (crdModeEnumType) Valid(cm CrdMode) bool {
	return slices.Contains(crdModeValues, cm)
}

// String returns the string representation of CrdMode
func (cm CrdMode) String() string {
	switch cm {
	case CrdModeError:
		return "error"
	case CrdModeIgnore:
		return "ignore"
	case CrdModeKeepUnknown:
		return "keep-unknown"
	default:
		return fmt.Sprintf("CrdModeUnknown(%d)", cm)
	}
}

// NewCrdMode creates a CrdMode from a string
func NewCrdMode(cm string) (CrdMode, error) {
	return CrdModeEnumType.Parse(cm)
}
