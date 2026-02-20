package expr

import (
	"math"
	"reflect"

	"github.com/jacoelho/rq/internal/rq/number"
)

func evaluate(root node, variables map[string]any) (any, error) {
	switch current := root.(type) {
	case literalNode:
		return current.value, nil
	case identifierNode:
		value, ok := variables[current.name]
		if !ok {
			return nil, expressionError("unknown variable %q", current.name)
		}
		return value, nil
	case unaryNode:
		if current.op != tokenNot {
			return nil, expressionError("unsupported unary operator")
		}

		rightValue, err := evaluate(current.right, variables)
		if err != nil {
			return nil, err
		}

		rightBool, err := mustBool(rightValue)
		if err != nil {
			return nil, err
		}
		return !rightBool, nil
	case binaryNode:
		switch current.op {
		case tokenAnd:
			leftValue, err := evaluate(current.left, variables)
			if err != nil {
				return nil, err
			}
			leftBool, err := mustBool(leftValue)
			if err != nil {
				return nil, err
			}
			if !leftBool {
				return false, nil
			}

			rightValue, err := evaluate(current.right, variables)
			if err != nil {
				return nil, err
			}
			rightBool, err := mustBool(rightValue)
			if err != nil {
				return nil, err
			}
			return rightBool, nil
		case tokenOr:
			leftValue, err := evaluate(current.left, variables)
			if err != nil {
				return nil, err
			}
			leftBool, err := mustBool(leftValue)
			if err != nil {
				return nil, err
			}
			if leftBool {
				return true, nil
			}

			rightValue, err := evaluate(current.right, variables)
			if err != nil {
				return nil, err
			}
			rightBool, err := mustBool(rightValue)
			if err != nil {
				return nil, err
			}
			return rightBool, nil
		case tokenEqual, tokenNotEqual:
			leftValue, err := evaluate(current.left, variables)
			if err != nil {
				return nil, err
			}
			rightValue, err := evaluate(current.right, variables)
			if err != nil {
				return nil, err
			}

			equal, err := compareValues(leftValue, rightValue)
			if err != nil {
				return nil, err
			}

			if current.op == tokenEqual {
				return equal, nil
			}
			return !equal, nil
		default:
			return nil, expressionError("unsupported binary operator")
		}
	default:
		return nil, expressionError("unsupported expression node")
	}
}

func mustBool(value any) (bool, error) {
	boolean, ok := value.(bool)
	if !ok {
		return false, expressionError("expected boolean value, got %T", value)
	}
	return boolean, nil
}

func compareValues(left any, right any) (bool, error) {
	if left == nil || right == nil {
		return left == nil && right == nil, nil
	}

	leftNumber, leftIsNumber := number.ToFloat64(left)
	rightNumber, rightIsNumber := number.ToFloat64(right)
	if leftIsNumber || rightIsNumber {
		if !leftIsNumber || !rightIsNumber {
			return false, expressionError("cannot compare %T and %T", left, right)
		}
		return nearlyEqual(leftNumber, rightNumber), nil
	}

	leftBool, leftIsBool := left.(bool)
	rightBool, rightIsBool := right.(bool)
	if leftIsBool || rightIsBool {
		if !leftIsBool || !rightIsBool {
			return false, expressionError("cannot compare %T and %T", left, right)
		}
		return leftBool == rightBool, nil
	}

	leftString, leftIsString := left.(string)
	rightString, rightIsString := right.(string)
	if leftIsString || rightIsString {
		if !leftIsString || !rightIsString {
			return false, expressionError("cannot compare %T and %T", left, right)
		}
		return leftString == rightString, nil
	}

	if reflect.TypeOf(left) == reflect.TypeOf(right) {
		return reflect.DeepEqual(left, right), nil
	}

	return false, expressionError("cannot compare %T and %T", left, right)
}

func nearlyEqual(left float64, right float64) bool {
	const epsilon = 1e-12
	return math.Abs(left-right) <= epsilon
}

func Eval(input string, variables map[string]any) (bool, error) {
	root, err := parse(input)
	if err != nil {
		return false, err
	}

	value, err := evaluate(root, variables)
	if err != nil {
		return false, err
	}

	result, ok := value.(bool)
	if !ok {
		return false, expressionError("expression must evaluate to boolean, got %T", value)
	}

	return result, nil
}

func Validate(input string) error {
	_, err := parse(input)
	return err
}

func ValidateBoolean(input string) error {
	root, err := parse(input)
	if err != nil {
		return err
	}

	return validateBooleanExpression(root)
}

func validateBooleanExpression(root node) error {
	switch current := root.(type) {
	case literalNode:
		if _, ok := current.value.(bool); ok {
			return nil
		}
		return expressionError("expression must evaluate to boolean, got %T", current.value)
	case identifierNode:
		return nil
	case unaryNode:
		if current.op != tokenNot {
			return expressionError("unsupported unary operator")
		}
		return validateBooleanExpression(current.right)
	case binaryNode:
		switch current.op {
		case tokenAnd, tokenOr:
			if err := validateBooleanExpression(current.left); err != nil {
				return err
			}
			if err := validateBooleanExpression(current.right); err != nil {
				return err
			}
			return nil
		case tokenEqual, tokenNotEqual:
			return nil
		default:
			return expressionError("unsupported binary operator")
		}
	default:
		return expressionError("unsupported expression node")
	}
}
