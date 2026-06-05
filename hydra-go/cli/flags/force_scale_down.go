package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithForceScaleDownFlag interface {
	WithForceScaleDownFlag() *ForceScaleDownFlag
}

type ForceScaleDownFlag struct {
	ForceScaleDown types.ForceScaleDown
}

var _ Flags = (*ForceScaleDownFlag)(nil)
var _ WithForceScaleDownFlag = (*ForceScaleDownFlag)(nil)

func (f *ForceScaleDownFlag) Flags() Flags {
	return f
}

func (f *ForceScaleDownFlag) WithForceScaleDownFlag() *ForceScaleDownFlag {
	return f
}
