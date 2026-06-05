package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithForceUninstallFlag interface {
	WithForceUninstallFlag() *ForceUninstallFlag
}

type ForceUninstallFlag struct {
	ForceUninstall types.ForceUninstall
}

var _ Flags = (*ForceUninstallFlag)(nil)
var _ WithForceUninstallFlag = (*ForceUninstallFlag)(nil)

func (f *ForceUninstallFlag) Flags() Flags {
	return f
}

func (f *ForceUninstallFlag) WithForceUninstallFlag() *ForceUninstallFlag {
	return f
}
