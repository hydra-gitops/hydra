package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithBootstrapFlag interface {
	WithBootstrapFlag() *BootstrapFlag
}

type BootstrapFlag struct {
	Bootstrap types.Bootstrap
}

var _ Flags = (*BootstrapFlag)(nil)
var _ WithBootstrapFlag = (*BootstrapFlag)(nil)

func (f *BootstrapFlag) Flags() Flags {
	return f
}

func (f *BootstrapFlag) WithBootstrapFlag() *BootstrapFlag {
	return f
}
