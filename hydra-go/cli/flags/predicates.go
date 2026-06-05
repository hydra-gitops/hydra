package flags

import (
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type WithPredicatesFlag interface {
	WithPredicatesFlag() *PredicatesFlag
}

type PredicatesFlag struct {
	Predicates []types.CelPredicate
}

var _ WithPredicatesFlag = (*PredicatesFlag)(nil)

func (c *PredicatesFlag) WithPredicatesFlag() *PredicatesFlag {
	return c
}
