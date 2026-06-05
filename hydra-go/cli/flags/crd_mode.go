package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithCrdModeFlag interface {
	WithCrdModeFlag() *CrdModeFlag
}

type CrdModeFlag struct {
	CrdMode types.CrdMode
}

var _ Flags = (*CrdModeFlag)(nil)
var _ WithCrdModeFlag = (*CrdModeFlag)(nil)

func (f *CrdModeFlag) Flags() Flags {
	return f
}

func (f *CrdModeFlag) WithCrdModeFlag() *CrdModeFlag {
	return f
}
