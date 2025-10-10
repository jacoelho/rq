package evaluator

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/jacoelho/rq/internal/parser"
	"github.com/theory/jsonpath"
)

// Common errors for evaluation operations.
var (
	ErrEvaluation   = errors.New("evaluation error")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnsupported  = errors.New("unsupported operation")
)

var regexCache = struct {
	sync.RWMutex
	patterns map[string]*regexp.Regexp
}{
	patterns: make(map[string]*regexp.Regexp),
}

type Operation string

const (
	OpEquals             Operation = "equals"
	OpNotEquals          Operation = "not_equals"
	OpContains           Operation = "contains"
	OpRegex              Operation = "regex"
	OpExists             Operation = "exists"
	OpLength             Operation = "length"
	OpGreaterThan        Operation = "greater_than"
	OpLessThan           Operation = "less_than"
	OpGreaterThanOrEqual Operation = "greater_than_or_equal"
	OpLessThanOrEqual    Operation = "less_than_or_equal"
	OpStartsWith         Operation = "starts_with"
	OpEndsWith           Operation = "ends_with"
	OpNotContains        Operation = "not_contains"
	OpIn                 Operation = "in"
)

func ParseOperation(s string) (Operation, error) {
	op := Operation(s)
	switch op {
	case OpEquals, OpNotEquals, OpContains, OpRegex, OpExists, OpLength,
		OpGreaterThan, OpLessThan, OpGreaterThanOrEqual, OpLessThanOrEqual,
		OpStartsWith, OpEndsWith, OpNotContains, OpIn:
		return op, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupported, s)
	}
}

func (op Operation) String() string {
	return string(op)
}

func compilePattern(pattern string) (*regexp.Regexp, error) {
	regexCache.RLock()
	if compiled, exists := regexCache.patterns[pattern]; exists {
		regexCache.RUnlock()
		return compiled, nil
	}
	regexCache.RUnlock()

	compiled, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}

	regexCache.Lock()
	regexCache.patterns[pattern] = compiled
	regexCache.Unlock()

	return compiled, nil
}

func Evaluate(op Operation, actual, expected any) (bool, error) {
	switch op {
	case OpEquals:
		return evaluateEquals(actual, expected), nil
	case OpNotEquals:
		return !evaluateEquals(actual, expected), nil
	case OpContains:
		return evaluateContains(actual, expected)
	case OpRegex:
		return evaluateRegex(actual, expected)
	case OpExists:
		return evaluateExists(actual), nil
	case OpLength:
		return evaluateLength(actual, expected)
	case OpGreaterThan:
		return evaluateGreaterThan(actual, expected)
	case OpLessThan:
		return evaluateLessThan(actual, expected)
	case OpGreaterThanOrEqual:
		return evaluateGreaterThanOrEqual(actual, expected)
	case OpLessThanOrEqual:
		return evaluateLessThanOrEqual(actual, expected)
	case OpStartsWith:
		return evaluateStartsWith(actual, expected)
	case OpEndsWith:
		return evaluateEndsWith(actual, expected)
	case OpNotContains:
		return evaluateNotContains(actual, expected)
	case OpIn:
		return evaluateIn(actual, expected)
	default:
		return false, fmt.Errorf("%w: %s", ErrUnsupported, op)
	}
}

// evaluateEquals checks equality with smart numeric comparison.
func evaluateEquals(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	return numericEqual(actual, expected)
}

func numericEqual(actual, expected any) bool {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if actualOk && expectedOk {
		return actualNum == expectedNum
	}

	return false
}

// toNumeric handles both YAML normalized types and HTTP response types.
func toNumeric(value any) (float64, bool) {
	switch v := value.(type) {
	case int64:
		return float64(v), true
	case float64:
		return v, true
	case int:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

func evaluateContains(actual, expected any) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return strings.Contains(actualStr, expectedStr), nil
}

func evaluateRegex(actual, expected any) (bool, error) {
	pattern, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("%w: regex pattern must be string, got %T", ErrInvalidInput, expected)
	}

	regex, err := compilePattern(pattern)
	if err != nil {
		return false, fmt.Errorf("%w: invalid regex pattern %s: %v", ErrInvalidInput, pattern, err)
	}

	actualStr := fmt.Sprintf("%v", actual)
	return regex.MatchString(actualStr), nil
}

func evaluateExists(actual any) bool {
	if actual == nil {
		return false
	}

	v := reflect.ValueOf(actual)
	switch v.Kind() {
	case reflect.String:
		return v.String() != ""
	case reflect.Slice, reflect.Map, reflect.Array:
		return v.Len() > 0
	case reflect.Ptr, reflect.Interface:
		return !v.IsNil()
	default:
		return true
	}
}

func evaluateLength(actual, expected any) (bool, error) {
	expectedLen, err := convertToInt(expected)
	if err != nil {
		return false, fmt.Errorf("%w: expected length must be integer: %v", ErrInvalidInput, err)
	}

	actualLen, err := getLength(actual)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrEvaluation, err)
	}

	return actualLen == expectedLen, nil
}

func getLength(value any) (int, error) {
	if value == nil {
		return 0, nil
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Slice, reflect.Map, reflect.Array:
		return v.Len(), nil
	default:
		return 0, fmt.Errorf("cannot get length of %T", value)
	}
}

// Since the parser now normalizes all numeric types, we only need to handle:
// - int64 (all integers are normalized to this)
// - float64 (all floats are normalized to this)
// - string (for string-to-int conversion)
func convertToInt(value any) (int, error) {
	switch v := value.(type) {
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case string:
		return strconv.Atoi(v)
	default:
		return 0, fmt.Errorf("cannot convert %T to int", value)
	}
}

// EvaluateJSONPathParserPredicate bridges parser package types with evaluator.
func EvaluateJSONPathParserPredicate(jsonData []byte, path string, predicate *parser.Predicate) (bool, error) {
	if predicate == nil {
		return false, fmt.Errorf("%w: predicate is nil", ErrInvalidInput)
	}

	// Parse JSONPath expression
	jsonpathExpr, err := jsonpath.Parse(path)
	if err != nil {
		return false, fmt.Errorf("%w: invalid JSONPath %s: %v", ErrInvalidInput, path, err)
	}

	// Parse JSON data
	var data any
	if err := json.Unmarshal(jsonData, &data); err != nil {
		return false, fmt.Errorf("%w: failed to parse JSON: %v", ErrInvalidInput, err)
	}

	// Execute JSONPath query
	results := jsonpathExpr.Select(data)

	// If no results, handle based on operation
	if len(results) == 0 {
		if predicate.Operation == "exists" {
			return false, nil
		}
		return false, fmt.Errorf("%w: JSONPath %s returned no results", ErrEvaluation, path)
	}

	// Parse operation and evaluate
	op, err := ParseOperation(predicate.Operation)
	if err != nil {
		return false, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	// Evaluate against the first result
	return Evaluate(op, results[0], predicate.Value)
}

func GetSupportedOperations() []string {
	return []string{
		string(OpEquals),
		string(OpNotEquals),
		string(OpContains),
		string(OpRegex),
		string(OpExists),
		string(OpLength),
		string(OpGreaterThan),
		string(OpLessThan),
		string(OpGreaterThanOrEqual),
		string(OpLessThanOrEqual),
		string(OpStartsWith),
		string(OpEndsWith),
		string(OpNotContains),
		string(OpIn),
	}
}

func IsSupportedOperation(operation string) bool {
	_, err := ParseOperation(operation)
	return err == nil
}

// evaluateGreaterThan checks if actual > expected (numeric comparison).
func evaluateGreaterThan(actual, expected any) (bool, error) {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if !actualOk || !expectedOk {
		return false, fmt.Errorf("%w: greater_than requires numeric values, got %T and %T", ErrInvalidInput, actual, expected)
	}

	return actualNum > expectedNum, nil
}

// evaluateLessThan checks if actual < expected (numeric comparison).
func evaluateLessThan(actual, expected any) (bool, error) {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if !actualOk || !expectedOk {
		return false, fmt.Errorf("%w: less_than requires numeric values, got %T and %T", ErrInvalidInput, actual, expected)
	}

	return actualNum < expectedNum, nil
}

// evaluateGreaterThanOrEqual checks if actual >= expected (numeric comparison).
func evaluateGreaterThanOrEqual(actual, expected any) (bool, error) {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if !actualOk || !expectedOk {
		return false, fmt.Errorf("%w: greater_than_or_equal requires numeric values, got %T and %T", ErrInvalidInput, actual, expected)
	}

	return actualNum >= expectedNum, nil
}

// evaluateLessThanOrEqual checks if actual <= expected (numeric comparison).
func evaluateLessThanOrEqual(actual, expected any) (bool, error) {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if !actualOk || !expectedOk {
		return false, fmt.Errorf("%w: less_than_or_equal requires numeric values, got %T and %T", ErrInvalidInput, actual, expected)
	}

	return actualNum <= expectedNum, nil
}

// evaluateStartsWith checks if actual string starts with expected string.
func evaluateStartsWith(actual, expected any) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return strings.HasPrefix(actualStr, expectedStr), nil
}

// evaluateEndsWith checks if actual string ends with expected string.
func evaluateEndsWith(actual, expected any) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return strings.HasSuffix(actualStr, expectedStr), nil
}

// evaluateNotContains checks if actual string does not contain expected string.
func evaluateNotContains(actual, expected any) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return !strings.Contains(actualStr, expectedStr), nil
}

// evaluateIn checks if actual value exists in expected collection (slice/array).
func evaluateIn(actual, expected any) (bool, error) {
	var collection []any
	switch v := expected.(type) {
	case []any:
		collection = v
	case []string:
		collection = make([]any, len(v))
		for i, item := range v {
			collection[i] = item
		}
	case []int64:
		collection = make([]any, len(v))
		for i, item := range v {
			collection[i] = item
		}
	case []float64:
		collection = make([]any, len(v))
		for i, item := range v {
			collection[i] = item
		}
	default:
		return false, fmt.Errorf("%w: in operation requires a collection (slice/array), got %T", ErrInvalidInput, expected)
	}

	for _, item := range collection {
		if evaluateEquals(actual, item) {
			return true, nil
		}
	}

	return false, nil
}
