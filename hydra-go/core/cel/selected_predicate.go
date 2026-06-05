package cel

import (
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type selectedPredicate struct {
	selector types.RefSelector
	delegate Predicate
}

var _ Predicate = selectedPredicate{}

func (s selectedPredicate) Predicate() []types.CelPredicate {
	if s.delegate == nil {
		return nil
	}
	return s.delegate.Predicate()
}

func (s selectedPredicate) Select(e entity.Entities) (entity.Entities, entity.Entities, error) {
	return e.Select(func(item entity.Entity) (bool, error) {
		return s.EvalBool(item, types.MissingKeysAccept)
	})
}

func (s selectedPredicate) EvalBool(e entity.Entity, missingKeys types.MissingKeys) (bool, error) {
	if !selectorMatchesEntity(s.selector, e) {
		return false, nil
	}
	if s.delegate == nil {
		return true, nil
	}
	return s.delegate.EvalBool(e, missingKeys)
}

func (s selectedPredicate) EvalBoolFromInput(input any, missingKeys types.MissingKeys) (bool, error) {
	if !selectorMatchesInput(s.selector, input) {
		return false, nil
	}
	if s.delegate == nil {
		return true, nil
	}
	return s.delegate.EvalBoolFromInput(input, missingKeys)
}

func (s selectedPredicate) EvalBoolFromMap(m map[string]any, missingKeys types.MissingKeys) (bool, error) {
	return s.EvalBoolFromInput(m, missingKeys)
}
