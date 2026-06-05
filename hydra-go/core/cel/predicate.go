package cel

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type Predicate interface {
	Predicate() []types.CelPredicate
	Select(e entity.Entities) (entity.Entities, entity.Entities, error)
	EvalBool(e entity.Entity, missingKeys types.MissingKeys) (bool, error)
	EvalBoolFromInput(input any, missingKeys types.MissingKeys) (bool, error)
	EvalBoolFromMap(m map[string]any, missingKeys types.MissingKeys) (bool, error)
}
