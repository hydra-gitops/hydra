package cel

import (
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types/ref"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// can be a Predicate or an Expression
type program struct {
	env     *Env
	code    string
	program cel.Program
}

// Predicate
var _ Predicate = program{}

func (p program) Predicate() []types.CelPredicate {
	return []types.CelPredicate{
		types.CelPredicate(p.code),
	}
}

func (p program) Select(e entity.Entities) (entity.Entities, entity.Entities, error) {
	return e.Select(func(e entity.Entity) (bool, error) {
		return p.EvalBool(e, types.MissingKeysAccept)
	})
}

func (p program) EvalBool(e entity.Entity, missingKeys types.MissingKeys) (bool, error) {
	vars := newEntityActivation(p.env, e)
	return p.evalBool(vars, missingKeys)
}

func (p program) EvalBoolFromInput(input any, missingKeys types.MissingKeys) (bool, error) {
	return p.evalBool(input, missingKeys)
}

func (p program) EvalBoolFromMap(m map[string]any, missingKeys types.MissingKeys) (bool, error) {
	return p.evalBool(m, missingKeys)
}

// Expression
var _ Expression = program{}

func (p program) Expression() types.CelExpression {
	return types.CelExpression(p.code)
}

func (p program) Eval(e entity.Entity) (ref.Val, error) {
	vars := newEntityActivation(p.env, e)
	return p.eval(vars)
}

func (p program) EvalFromInput(input any) (ref.Val, error) {
	return p.eval(input)
}

func (p program) EvalFromMap(m map[string]any) (ref.Val, error) {
	return p.eval(m)
}
