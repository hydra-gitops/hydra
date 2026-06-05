package references

import "fmt"

// PickerExprError is returned when a ref-parser pick expression does not evaluate
// to a list of CelRef ([]types.RefDefinition). Error() is intentionally short so
// CLI stderr stays minimal; use Expr, Expected, and GotType for full context.
type PickerExprError struct {
	Expr     string
	Expected string // "list" or "[]RefDefinition"
	GotType  string // fmt.Sprintf("%T", v)
}

func (e *PickerExprError) Error() string {
	switch e.Expected {
	case "list":
		return "picker expression must evaluate to a list"
	case "[]RefDefinition":
		return "picker list element must be []RefDefinition (CelRef)"
	default:
		return "picker expression produced an invalid value"
	}
}

func newPickerExprWantList(expr string, got any) *PickerExprError {
	return &PickerExprError{
		Expr:     expr,
		Expected: "list",
		GotType:  fmt.Sprintf("%T", got),
	}
}

func newPickerExprWantRefDefSlice(expr string, got any) *PickerExprError {
	return &PickerExprError{
		Expr:     expr,
		Expected: "[]RefDefinition",
		GotType:  fmt.Sprintf("%T", got),
	}
}
