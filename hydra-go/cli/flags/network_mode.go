package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithHelmNetworkModeFlag interface {
	WithHelmNetworkModeFlag() *HelmNetworkModeFlag
}

type HelmNetworkModeFlag struct {
	HelmNetworkMode types.HelmNetworkMode
}

var _ Flags = (*HelmNetworkModeFlag)(nil)
var _ WithHelmNetworkModeFlag = (*HelmNetworkModeFlag)(nil)

func (f *HelmNetworkModeFlag) Flags() Flags {
	return f
}

func (f *HelmNetworkModeFlag) WithHelmNetworkModeFlag() *HelmNetworkModeFlag {
	return f
}
