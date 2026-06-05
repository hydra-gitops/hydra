package flags

import "hydra-gitops.org/hydra/hydra-go/core/types"

type WithPickFlag interface {
	WithPickFlag() *PickFlag
}

type PickFlag struct {
	Pick types.CelExpression
}

var _ WithPickFlag = (*PickFlag)(nil)

func (f *PickFlag) WithPickFlag() *PickFlag {
	return f
}
