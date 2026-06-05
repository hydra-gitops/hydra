package cel

import (
	"fmt"
	"reflect"

	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	baseerrors "hydra-gitops.org/hydra/hydra-go/base/errors"
	"hydra-gitops.org/hydra/hydra-go/base/log"
	"hydra-gitops.org/hydra/hydra-go/core/types"
)

// ExpressionResultTypeError reports that a compiled CEL expression evaluated successfully but
// returned a value of an unexpected type for the current consumer.
type ExpressionResultTypeError struct {
	Expression types.CelExpression
	Expected   string
	GotType    string
}

func (e *ExpressionResultTypeError) Error() string {
	return fmt.Sprintf("CEL expression returned %s, expected %s", e.GotType, e.Expected)
}

func (e *ExpressionResultTypeError) ErrorId() baseerrors.ErrorId {
	return baseerrors.ErrEvaluationFailed
}

func NewExpressionResultTypeError(expression types.CelExpression, expected string, got any) error {
	return log.CreateError(baseerrors.ErrEvaluationFailed,
		"CEL expression '{expression}' returned unexpected type {got}; expected {expected}",
		log.String("expression", string(expression)),
		log.String("got", expressionResultTypeName(got)),
		log.String("expected", expected))
}

func expressionResultTypeName(got any) string {
	switch value := got.(type) {
	case nil:
		return "nil"
	case ref.Val:
		if value == nil {
			return "nil"
		}
		if value == celtypes.NullValue {
			return "null"
		}
		if raw := value.Value(); raw != nil {
			return fmt.Sprintf("%T", raw)
		}
		if t := value.Type(); t != nil {
			return string(t.TypeName())
		}
		return fmt.Sprintf("%T", value)
	default:
		return fmt.Sprintf("%T", got)
	}
}

func RefValToNative(v ref.Val, expression types.CelExpression, target reflect.Type, expected string) (any, error) {
	if v == nil || v == celtypes.NullValue {
		return nil, NewExpressionResultTypeError(expression, expected, v)
	}
	native, err := v.ConvertToNative(target)
	if err != nil || native == nil {
		return nil, NewExpressionResultTypeError(expression, expected, v)
	}
	return native, nil
}
