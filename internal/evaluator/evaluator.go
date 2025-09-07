package evaluator

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacoelho/rq/internal/parser"
	"github.com/theory/jsonpath"
)

// Common errors for evaluation operations.
var (
	ErrEvaluation   = errors.New("evaluation error")
	ErrInvalidInput = errors.New("invalid input")
	ErrUnsupported  = errors.New("unsupported operation")
)

// Operation represents a type-safe operation for evaluation
type Operation string

// Supported operations
const (
	OpEquals    Operation = "equals"
	OpNotEquals Operation = "not_equals"
	OpContains  Operation = "contains"
	OpRegex     Operation = "regex"
	OpExists    Operation = "exists"
	OpLength    Operation = "length"
)

// ParseOperation converts a string to an Operation with validation
func ParseOperation(s string) (Operation, error) {
	op := Operation(s)
	switch op {
	case OpEquals, OpNotEquals, OpContains, OpRegex, OpExists, OpLength:
		return op, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupported, s)
	}
}

// String returns the string representation of the operation
func (op Operation) String() string {
	return string(op)
}

// Evaluate performs the specified operation on actual and expected values
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
	default:
		return false, fmt.Errorf("%w: %s", ErrUnsupported, op)
	}
}

// evaluateEquals checks if two values are equal with smart numeric comparison.
func evaluateEquals(actual, expected any) bool {
	// First try exact deep equality
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	// Handle numeric comparisons across different types
	return numericEqual(actual, expected)
}

// numericEqual checks if two values are numerically equal across different types.
func numericEqual(actual, expected any) bool {
	actualNum, actualOk := toNumeric(actual)
	expectedNum, expectedOk := toNumeric(expected)

	if actualOk && expectedOk {
		return actualNum == expectedNum
	}

	return false
}

// toNumeric converts numeric types to float64 for comparison.
// Handles both normalized types (from YAML parsing) and runtime types (from HTTP responses).
// Normalized types: int64, float64
// Runtime types: int, float32 (for compatibility with standard library)
func toNumeric(value any) (float64, bool) {
	switch v := value.(type) {
	// Normalized types (from YAML parsing)
	case int64:
		return float64(v), true
	case float64:
		return v, true
	// Runtime types (from HTTP responses, standard library, etc.)
	case int:
		return float64(v), true
	case float32:
		return float64(v), true
	default:
		return 0, false
	}
}

// evaluateContains checks if actual contains the expected value.
func evaluateContains(actual, expected any) (bool, error) {
	actualStr := fmt.Sprintf("%v", actual)
	expectedStr := fmt.Sprintf("%v", expected)

	return strings.Contains(actualStr, expectedStr), nil
}

// evaluateRegex checks if actual matches the expected regex pattern.
func evaluateRegex(actual, expected any) (bool, error) {
	pattern, ok := expected.(string)
	if !ok {
		return false, fmt.Errorf("%w: regex pattern must be string, got %T", ErrInvalidInput, expected)
	}

	// Compile the regex pattern each time (fine for CLI usage)
	regex, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("%w: invalid regex pattern %s: %v", ErrInvalidInput, pattern, err)
	}

	actualStr := fmt.Sprintf("%v", actual)
	return regex.MatchString(actualStr), nil
}

// evaluateExists checks if the actual value exists (not nil and not empty).
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

// evaluateLength checks if the actual value has the expected length.
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

// getLength returns the length of the given value.
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

// convertToInt converts normalized types to int.
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

// EvaluateJSONPathParserPredicate evaluates a JSONPath predicate against JSON data.
// This function bridges the parser package types with the evaluator.
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

// GetSupportedOperations returns a list of supported predicate operations.
func GetSupportedOperations() []string {
	return []string{
		string(OpEquals),
		string(OpNotEquals),
		string(OpContains),
		string(OpRegex),
		string(OpExists),
		string(OpLength),
	}
}

// IsSupportedOperation checks if the given operation is supported.
func IsSupportedOperation(operation string) bool {
	_, err := ParseOperation(operation)
	return err == nil
}
