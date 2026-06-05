package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithContextFlag interface {
	WithContextFlag() *ContextFlag
}

type ContextFlag struct {
	HydraContext types.HydraContext
}

var _ Flags = (*ContextFlag)(nil)
var _ WithContextFlag = (*ContextFlag)(nil)

func (f *ContextFlag) Flags() Flags {
	return f
}

func (f *ContextFlag) WithContextFlag() *ContextFlag {
	return f
}
