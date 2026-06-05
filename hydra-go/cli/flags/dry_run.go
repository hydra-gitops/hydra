package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithDryRunFlag interface {
	WithDryRunFlag() *DryRunFlag
}

type DryRunFlag struct {
	DryRun types.DryRun
}

var _ Flags = (*DryRunFlag)(nil)
var _ WithDryRunFlag = (*DryRunFlag)(nil)

func (f *DryRunFlag) Flags() Flags {
	return f
}

func (f *DryRunFlag) WithDryRunFlag() *DryRunFlag {
	return f
}
