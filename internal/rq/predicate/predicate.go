package predicate

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/jacoelho/rq/internal/rq/number"
)

var (
	ErrInvalidInput = errors.New("invalid predicate input")
	ErrUnsupported  = errors.New("unsupported predicate operation")
)

type Operator string

const (
	OpEquals             Operator = "equals"
	OpNotEquals          Operator = "not_equals"
	OpContains           Operator = "contains"
	OpRegex              Operator = "regex"
	OpExists             Operator = "exists"
	OpLength             Operator = "length"
	OpGreaterThan        Operator = "greater_than"
	OpLessThan           Operator = "less_than"
	OpGreaterThanOrEqual Operator = "greater_than_or_equal"
	OpLessThanOrEqual    Operator = "less_than_or_equal"
	OpStartsWith         Operator = "starts_with"
	OpEndsWith           Operator = "ends_with"
	OpNotContains        Operator = "not_contains"
	OpIn                 Operator = "in"
	OpTypeIs             Operator = "type_is"
)

type Expr struct {
	Op       Operator
	Value    any
	HasValue bool
}

var supportedOperatorSet = map[Operator]struct{}{
	OpEquals:             {},
	OpNotEquals:          {},
	OpContains:           {},
	OpRegex:              {},
	OpExists:             {},
	OpLength:             {},
	OpGreaterThan:        {},
	OpLessThan:           {},
	OpGreaterThanOrEqual: {},
	OpLessThanOrEqual:    {},
	OpStartsWith:         {},
	OpEndsWith:           {},
	OpNotContains:        {},
	OpIn:                 {},
	OpTypeIs:             {},
}

var supportedTypeValues = []string{
	"array",
	"object",
	"string",
	"number",
	"boolean",
	"null",
}

var supportedTypeValueSet = map[string]struct{}{
	"array":   {},
	"object":  {},
	"string":  {},
	"number":  {},
	"boolean": {},
	"null":    {},
}

type regexCompiler interface {
	Compile(pattern string) (*regexp.Regexp, error)
}

type cachedRegexCompiler struct {
	mu       sync.RWMutex
	patterns map[string]*regexp.Regexp
}

func newCachedRegexCompiler() *cachedRegexCompiler {
	return &cachedRegexCompiler{
		patterns: make(map[string]*regexp.Regexp),
	}
}

func (c *cachedRegexCompiler) Compile(pattern string) (*regexp.Regexp, error) {
	c.mu.RLock()
	if compiled, ok := c.patterns[pattern]; ok {
		c.mu.RUnlock()
		return compiled, nil
	}
	c.mu.RUnlock()

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex %q: %v", ErrInvalidInput, pattern, err)
	}

	c.mu.Lock()
	c.patterns[pattern] = compiled
	c.mu.Unlock()

	return compiled, nil
}

type operationFunc func(actual any, expected any) (bool, error)

type Evaluator struct {
	regexCompiler regexCompiler
	operations    map[Operator]operationFunc
}

func NewEvaluator() *Evaluator {
	return newEvaluator(newCachedRegexCompiler())
}

func newEvaluator(compiler regexCompiler) *Evaluator {
	e := &Evaluator{
		regexCompiler: compiler,
	}

	e.operations = map[Operator]operationFunc{
		OpEquals: func(actual any, expected any) (bool, error) {
			return equalValues(actual, expected), nil
		},
		OpNotEquals: func(actual any, expected any) (bool, error) {
			return !equalValues(actual, expected), nil
		},
		OpContains: evaluateContains,
		OpRegex:    e.evaluateRegex,
		OpExists: func(actual any, _ any) (bool, error) {
			return evaluateExists(actual), nil
		},
		OpLength:             evaluateLength,
		OpGreaterThan:        evaluateGreaterThan,
		OpLessThan:           evaluateLessThan,
		OpGreaterThanOrEqual: evaluateGreaterThanOrEqual,
		OpLessThanOrEqual:    evaluateLessThanOrEqual,
		OpStartsWith:         evaluateStartsWith,
		OpEndsWith:           evaluateEndsWith,
		OpNotContains:        evaluateNotContains,
		OpIn:                 evaluateIn,
		OpTypeIs:             evaluateTypeIs,
	}

	return e
}

func isSupportedOperator(op Operator) bool {
	_, ok := supportedOperatorSet[op]
	return ok
}

func ParseOperator(input string) (Operator, error) {
	op := Operator(input)
	if isSupportedOperator(op) {
		return op, nil
	}
	return "", fmt.Errorf("%w: %q", ErrUnsupported, input)
}

func ValidateExpr(expr Expr) error {
	if !isSupportedOperator(expr.Op) {
		return fmt.Errorf("%w: %q", ErrUnsupported, expr.Op)
	}

	if expr.Op == OpExists {
		if expr.HasValue {
			return fmt.Errorf("%w: operation %q does not accept a value", ErrInvalidInput, expr.Op)
		}
		return nil
	}

	if !expr.HasValue {
		return fmt.Errorf("%w: operation %q requires a value", ErrInvalidInput, expr.Op)
	}

	if expr.Op == OpTypeIs {
		if _, err := parseTypeValue(expr.Value); err != nil {
			return err
		}
	}

	return nil
}

func (e *Evaluator) Evaluate(expr Expr, actual any) (bool, error) {
	if err := ValidateExpr(expr); err != nil {
		return false, err
	}

	opFunc, ok := e.operations[expr.Op]
	if !ok {
		return false, fmt.Errorf("%w: %q", ErrUnsupported, expr.Op)
	}

	return opFunc(actual, expr.Value)
}

func EvaluateExpr(expr Expr, actual any) (bool, error) {
	return NewEvaluator().Evaluate(expr, actual)
}

func equalValues(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	actualNumber, actualIsNumber := number.ToFloat64(actual)
	expectedNumber, expectedIsNumber := number.ToFloat64(expected)
	if actualIsNumber && expectedIsNumber {
		return actualNumber == expectedNumber
	}

	return false
}

func evaluateContains(actual, expected any) (bool, error) {
	return evaluateStringComparison(OpContains, actual, expected, strings.Contains)
}

func (e *Evaluator) evaluateRegex(actual any, expected any) (bool, error) {
	actualString, err := requireStringActual(OpRegex, actual)
	if err != nil {
		return false, err
	}
	pattern, err := requireStringExpected(OpRegex, expected)
	if err != nil {
		return false, err
	}

	regex, err := e.regexCompiler.Compile(pattern)
	if err != nil {
		return false, err
	}

	return regex.MatchString(actualString), nil
}

func evaluateExists(actual any) bool {
	if actual == nil {
		return false
	}

	value := reflect.ValueOf(actual)
	switch value.Kind() {
	case reflect.String:
		return value.Len() > 0
	case reflect.Slice, reflect.Array, reflect.Map:
		return value.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !value.IsNil()
	default:
		return true
	}
}

func evaluateLength(actual, expected any) (bool, error) {
	expectedLength, err := number.ToStrictInt(expected)
	if err != nil {
		return false, fmt.Errorf("%w: %q requires integer expected value: %v", ErrInvalidInput, OpLength, err)
	}

	if actual == nil {
		return false, fmt.Errorf("%w: %q requires string/slice/map/array actual value, got nil", ErrInvalidInput, OpLength)
	}

	actualValue := reflect.ValueOf(actual)
	switch actualValue.Kind() {
	case reflect.String, reflect.Slice, reflect.Array, reflect.Map:
		return actualValue.Len() == expectedLength, nil
	default:
		return false, fmt.Errorf("%w: %q requires string/slice/map/array actual value, got %T", ErrInvalidInput, OpLength, actual)
	}
}

func evaluateGreaterThan(actual, expected any) (bool, error) {
	return evaluateNumericComparison(OpGreaterThan, actual, expected, func(a, b float64) bool { return a > b })
}

func evaluateLessThan(actual, expected any) (bool, error) {
	return evaluateNumericComparison(OpLessThan, actual, expected, func(a, b float64) bool { return a < b })
}

func evaluateGreaterThanOrEqual(actual, expected any) (bool, error) {
	return evaluateNumericComparison(OpGreaterThanOrEqual, actual, expected, func(a, b float64) bool { return a >= b })
}

func evaluateLessThanOrEqual(actual, expected any) (bool, error) {
	return evaluateNumericComparison(OpLessThanOrEqual, actual, expected, func(a, b float64) bool { return a <= b })
}

func evaluateNumericComparison(op Operator, actual, expected any, compare func(float64, float64) bool) (bool, error) {
	actualNumber, actualIsNumber := number.ToFloat64(actual)
	expectedNumber, expectedIsNumber := number.ToFloat64(expected)
	if !actualIsNumber || !expectedIsNumber {
		return false, fmt.Errorf("%w: %q requires numeric values, got %T and %T", ErrInvalidInput, op, actual, expected)
	}

	return compare(actualNumber, expectedNumber), nil
}

func evaluateStartsWith(actual, expected any) (bool, error) {
	return evaluateStringComparison(OpStartsWith, actual, expected, strings.HasPrefix)
}

func evaluateEndsWith(actual, expected any) (bool, error) {
	return evaluateStringComparison(OpEndsWith, actual, expected, strings.HasSuffix)
}

func evaluateNotContains(actual, expected any) (bool, error) {
	return evaluateStringComparison(OpNotContains, actual, expected, func(actualString, expectedString string) bool {
		return !strings.Contains(actualString, expectedString)
	})
}

func evaluateIn(actual, expected any) (bool, error) {
	expectedValue := reflect.ValueOf(expected)
	if expectedValue.Kind() != reflect.Slice && expectedValue.Kind() != reflect.Array {
		return false, fmt.Errorf("%w: %q requires array/slice expected value, got %T", ErrInvalidInput, OpIn, expected)
	}

	for i := 0; i < expectedValue.Len(); i++ {
		if equalValues(actual, expectedValue.Index(i).Interface()) {
			return true, nil
		}
	}

	return false, nil
}

func evaluateTypeIs(actual, expected any) (bool, error) {
	expectedType, err := parseTypeValue(expected)
	if err != nil {
		return false, err
	}

	actualType := detectTypeValue(actual)
	return actualType == expectedType, nil
}

func parseTypeValue(value any) (string, error) {
	typeValue, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("%w: %q requires string expected value, got %T", ErrInvalidInput, OpTypeIs, value)
	}

	normalized := strings.ToLower(strings.TrimSpace(typeValue))
	if _, ok := supportedTypeValueSet[normalized]; ok {
		return normalized, nil
	}

	return "", fmt.Errorf("%w: %q requires one of %v, got %q", ErrInvalidInput, OpTypeIs, supportedTypeValues, typeValue)
}

func detectTypeValue(value any) string {
	if value == nil {
		return "null"
	}

	reflected := reflect.ValueOf(value)
	for reflected.Kind() == reflect.Interface || reflected.Kind() == reflect.Ptr {
		if reflected.IsNil() {
			return "null"
		}
		reflected = reflected.Elem()
	}

	switch reflected.Kind() {
	case reflect.Array, reflect.Slice:
		return "array"
	case reflect.Map, reflect.Struct:
		return "object"
	case reflect.String:
		return "string"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "number"
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "number"
	case reflect.Float32, reflect.Float64:
		return "number"
	default:
		return "object"
	}
}

func evaluateStringComparison(op Operator, actual, expected any, compare func(actual string, expected string) bool) (bool, error) {
	actualString, expectedString, err := requireStringPair(op, actual, expected)
	if err != nil {
		return false, err
	}

	return compare(actualString, expectedString), nil
}

func requireStringPair(op Operator, actual, expected any) (string, string, error) {
	actualString, err := requireStringActual(op, actual)
	if err != nil {
		return "", "", err
	}

	expectedString, err := requireStringExpected(op, expected)
	if err != nil {
		return "", "", err
	}

	return actualString, expectedString, nil
}

func requireStringActual(op Operator, actual any) (string, error) {
	actualString, ok := actual.(string)
	if !ok {
		return "", fmt.Errorf("%w: %q requires string actual value, got %T", ErrInvalidInput, op, actual)
	}

	return actualString, nil
}

func requireStringExpected(op Operator, expected any) (string, error) {
	expectedString, ok := expected.(string)
	if !ok {
		return "", fmt.Errorf("%w: %q requires string expected value, got %T", ErrInvalidInput, op, expected)
	}

	return expectedString, nil
}
