package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithDiffModeFlag interface {
	WithDiffModeFlag() *DiffModeFlag
}

type DiffModeFlag struct {
	DiffMode types.DiffMode
}

var _ Flags = (*DiffModeFlag)(nil)
var _ WithDiffModeFlag = (*DiffModeFlag)(nil)

func (f *DiffModeFlag) Flags() Flags {
	return f
}

func (f *DiffModeFlag) WithDiffModeFlag() *DiffModeFlag {
	return f
}
