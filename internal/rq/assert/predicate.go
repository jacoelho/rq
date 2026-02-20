package assert

import (
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/predicate"
)

// BuildExpr converts parser predicate input into a validated predicate expression.
func BuildExpr(input model.Predicate) (predicate.Expr, error) {
	op, err := predicate.ParseOperator(input.Operation)
	if err != nil {
		return predicate.Expr{}, err
	}

	hasValue := input.HasValue || input.Value != nil

	return predicate.Expr{
		Op:       op,
		Value:    input.Value,
		HasValue: hasValue,
	}, nil
}

func Validate(input model.Predicate) error {
	expr, err := BuildExpr(input)
	if err != nil {
		return err
	}

	return predicate.ValidateExpr(expr)
}

func Evaluate(actual any, input model.Predicate) (bool, error) {
	expr, err := BuildExpr(input)
	if err != nil {
		return false, err
	}

	return predicate.EvaluateExpr(expr, actual)
}
