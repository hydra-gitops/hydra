package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithColorFlag interface {
	WithColorFlag() *ColorFlag
}

type ColorFlag struct {
	Color types.Color
}

var _ Flags = (*ColorFlag)(nil)
var _ WithColorFlag = (*ColorFlag)(nil)

func (f *ColorFlag) Flags() Flags {
	return f
}

func (f *ColorFlag) WithColorFlag() *ColorFlag {
	return f
}
