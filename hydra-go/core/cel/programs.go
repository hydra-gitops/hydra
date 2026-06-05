package cel

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type programs struct {
	env      *Env
	programs []program
}

var _ Predicate = programs{}

func (ps programs) Predicate() []types.CelPredicate {
	predicates := []types.CelPredicate{}

	for _, p := range ps.programs {
		predicates = append(predicates, types.CelPredicate(p.code))
	}

	return predicates
}

func (ps programs) Select(e entity.Entities) (entity.Entities, entity.Entities, error) {
	return e.Select(func(e entity.Entity) (bool, error) {
		return ps.EvalBool(e, types.MissingKeysAccept)
	})
}

func (ps programs) EvalBool(e entity.Entity, missingKeys types.MissingKeys) (bool, error) {
	vars := newEntityActivation(ps.env, e)
	return ps.EvalBoolFromInput(vars, missingKeys)
}

func (ps programs) EvalBoolFromInput(input any, missingKeys types.MissingKeys) (bool, error) {
	for _, p := range ps.programs {
		result, err := p.evalBool(input, missingKeys)
		if err != nil {
			return false, err
		}
		if !result {
			return false, nil
		}
	}
	return true, nil
}

func (ps programs) EvalBoolFromMap(m map[string]any, missingKeys types.MissingKeys) (bool, error) {
	return ps.EvalBoolFromInput(m, missingKeys)
}
