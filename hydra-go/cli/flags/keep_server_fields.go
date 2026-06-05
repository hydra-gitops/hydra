package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithKeepServerFieldsFlag interface {
	WithKeepServerFieldsFlag() *KeepServerFieldsFlag
}

type KeepServerFieldsFlag struct {
	KeepServerFields types.KeepServerFields
}

var _ Flags = (*KeepServerFieldsFlag)(nil)
var _ WithKeepServerFieldsFlag = (*KeepServerFieldsFlag)(nil)

func (f *KeepServerFieldsFlag) Flags() Flags {
	return f
}

func (f *KeepServerFieldsFlag) WithKeepServerFieldsFlag() *KeepServerFieldsFlag {
	return f
}
