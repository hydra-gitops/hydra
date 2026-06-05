package cel

import (
	"github.com/google/cel-go/common/types/ref"
	"hydra-gitops.org/hydra/hydra-go/core/entity"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

type Expression interface {
	Expression() types.CelExpression
	Eval(e entity.Entity) (ref.Val, error)
	EvalFromInput(input any) (ref.Val, error)
	EvalFromMap(m map[string]any) (ref.Val, error)
}
