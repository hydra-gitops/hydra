package types

import (
	"fmt"
	"slices"
)

// HelmNetworkMode represents different network modes for loading charts
type HelmNetworkMode int

const (
	HelmNetworkModeOnline HelmNetworkMode = iota
	HelmNetworkModeOffline
	HelmNetworkModeLocal
	HelmNetworkModeError
)

var helmNetworkModeValues = []HelmNetworkMode{
	HelmNetworkModeOnline,
	HelmNetworkModeOffline,
	HelmNetworkModeLocal,
	HelmNetworkModeError,
}

type helmNetworkModeEnumType struct{}

var HelmNetworkModeEnumType EnumType[HelmNetworkMode] = helmNetworkModeEnumType{}

var _ EnumType[HelmNetworkMode] = (*helmNetworkModeEnumType)(nil)

func (helmNetworkModeEnumType) Stringify(nm HelmNetworkMode) (string, error) {
	return nm.String(), nil
}

func (helmNetworkModeEnumType) Parse(nm string) (HelmNetworkMode, error) {
	switch nm {
	case "online":
		return HelmNetworkModeOnline, nil
	case "offline":
		return HelmNetworkModeOffline, nil
	case "local":
		return HelmNetworkModeLocal, nil
	case "error":
		return HelmNetworkModeError, nil
	default:
		return -1, fmt.Errorf("invalid helm network mode enum: %s, allowed values are: %v", nm, helmNetworkModeValues)
	}
}

func (helmNetworkModeEnumType) Values() []HelmNetworkMode {
	return helmNetworkModeValues
}

func (helmNetworkModeEnumType) Valid(nm HelmNetworkMode) bool {
	return slices.Contains(helmNetworkModeValues, nm)
}

// String returns the string representation of HelmNetworkMode
func (nm HelmNetworkMode) String() string {
	switch nm {
	case HelmNetworkModeOnline:
		return "online"
	case HelmNetworkModeOffline:
		return "offline"
	case HelmNetworkModeLocal:
		return "local"
	case HelmNetworkModeError:
		return "error"
	default:
		return fmt.Sprintf("HelmNetworkModeUnknown(%d)", nm)
	}
}

// NewHelmNetworkMode creates a HelmNetworkMode from a string
func NewHelmNetworkMode(nm string) (HelmNetworkMode, error) {
	return HelmNetworkModeEnumType.Parse(nm)
}
