package evaluator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/jacoelho/rq/internal/jsonpath"
	"github.com/jacoelho/rq/internal/parser"
)

// Operation constants for predicate evaluation.
const (
	opEquals    = "equals"
	opNotEquals = "not_equals"
	opRegex     = "regex"
	opContains  = "contains"
	opExists    = "exists"
	opLength    = "length"
)

// predicate represents a predicate operation with validation.
type predicate struct {
	Operation string
	Value     any
}

// NewPredicate creates a new predicate with validation.
func NewPredicate(operation string, value any) (*predicate, error) {
	p := &predicate{
		Operation: operation,
		Value:     value,
	}

	if err := p.validate(); err != nil {
		return nil, err
	}

	return p, nil
}

// validate performs operation-specific validation for predicates.
func (p *predicate) validate() error {
	switch p.Operation {
	case opRegex:
		if p.Value == nil {
			return fmt.Errorf("regex predicate requires a pattern value")
		}
		pattern, ok := p.Value.(string)
		if !ok {
			return fmt.Errorf("regex predicate pattern must be a string, got %T", p.Value)
		}
		if _, err := regexp.Compile(pattern); err != nil {
			return fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
		}
	case opContains:
		if p.Value == nil {
			return fmt.Errorf("contains predicate requires a value")
		}
		if _, ok := p.Value.(string); !ok {
			return fmt.Errorf("contains predicate value must be a string, got %T", p.Value)
		}
	case opLength:
		if p.Value == nil {
			return fmt.Errorf("length predicate requires a value")
		}
		// Allow various numeric types that can be converted to int
		switch p.Value.(type) {
		case int, int64, uint64, float64, json.Number:
			// Valid numeric types
		default:
			return fmt.Errorf("length predicate value must be an integer, got %T", p.Value)
		}
	case opEquals, opNotEquals:
		if p.Value == nil {
			return fmt.Errorf("%s predicate requires a value", p.Operation)
		}
	case opExists:
		// opExists doesn't require a value
	default:
		return fmt.Errorf("unknown predicate operation: %q", p.Operation)
	}
	return nil
}

// IsValidOperation checks if an operation string is valid.
func IsValidOperation(operation string) bool {
	switch operation {
	case opEquals, opNotEquals, opRegex, opContains, opExists, opLength:
		return true
	default:
		return false
	}
}

// EvaluatePredicate evaluates the given predicate against the provided input value.
// It returns true if the predicate matches, false otherwise, or an error if evaluation fails.
//
// Supported predicate operations:
//   - equals: exact equality comparison
//   - not_equals: inequality comparison
//   - regex: regular expression pattern matching
//   - contains: substring containment check
//   - exists: existence/nil check
//   - length: length comparison for arrays, slices, maps, and strings
func EvaluatePredicate(pred *predicate, input any) (bool, error) {
	switch pred.Operation {
	case opEquals:
		return compareValues(input, pred.Value), nil
	case opNotEquals:
		return !compareValues(input, pred.Value), nil
	case opRegex:
		return evaluateRegex(pred.Value, input)
	case opContains:
		return evaluateContains(pred.Value, input)
	case opExists:
		return input != nil, nil
	case opLength:
		return evaluateLength(pred.Value, input)
	default:
		return false, fmt.Errorf("unsupported predicate operation: %q", pred.Operation)
	}
}

func evaluateRegex(patternValue, input any) (bool, error) {
	pattern, ok := patternValue.(string)
	if !ok {
		return false, fmt.Errorf("regex predicate expects string pattern, got %T", patternValue)
	}

	// Convert input to string for regex matching
	inputStr := convertToString(input)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false, fmt.Errorf("invalid regex pattern %q: %w", pattern, err)
	}

	return re.MatchString(inputStr), nil
}

// convertToString converts various types to their string representation
func convertToString(input any) string {
	switch v := input.(type) {
	case string:
		return v
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case uint64:
		return strconv.FormatUint(v, 10)
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(v)
	case json.Number:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func evaluateContains(valueValue, input any) (bool, error) {
	inputStr, ok := input.(string)
	if !ok {
		return false, fmt.Errorf("contains predicate expects string input, got %T", input)
	}

	valueStr, ok := valueValue.(string)
	if !ok {
		return false, fmt.Errorf("contains predicate expects string value, got %T", valueValue)
	}

	return strings.Contains(inputStr, valueStr), nil
}

func evaluateLength(expectedValue, input any) (bool, error) {
	// Convert expectedValue to int with type coercion for numeric types
	var expectedLen int
	switch v := expectedValue.(type) {
	case int:
		expectedLen = v
	case int64:
		expectedLen = int(v)
	case uint64:
		expectedLen = int(v)
	case float64:
		expectedLen = int(v)
	case json.Number:
		if intVal, err := v.Int64(); err == nil {
			expectedLen = int(intVal)
		} else {
			return false, fmt.Errorf("length predicate expects integer value, got json.Number that cannot be converted to int: %v", v)
		}
	default:
		return false, fmt.Errorf("length predicate expects integer value, got %T", expectedValue)
	}

	inputLen, ok := getLength(input)
	if !ok {
		return false, fmt.Errorf("length predicate expects array, slice, map, or string input, got %T", input)
	}

	return inputLen == expectedLen, nil
}

// EvaluateParserPredicate evaluates a parser.Predicate directly.
// This is the main function to use when working with predicates parsed from YAML.
func EvaluateParserPredicate(pred *parser.Predicate, input any) (bool, error) {
	evalPred := &predicate{
		Operation: pred.Operation,
		Value:     pred.Value,
	}

	return EvaluatePredicate(evalPred, input)
}

// EvaluateJSONPathPredicate evaluates a predicate against JSON data using a JSONPath expression.
// This is a convenience function that combines JSONPath evaluation with predicate checking.
func EvaluateJSONPathPredicate(jsonData []byte, jsonPathExpr string, pred *predicate) (bool, error) {
	if err := jsonpath.Validate(jsonPathExpr); err != nil {
		return false, fmt.Errorf("invalid JSONPath expression %q: %w", jsonPathExpr, err)
	}

	ctx := context.Background()
	results, err := jsonpath.Stream(ctx, bytes.NewReader(jsonData), jsonPathExpr)
	if err != nil {
		return false, fmt.Errorf("JSONPath execution failed for %q: %w", jsonPathExpr, err)
	}

	for result, err := range results {
		if err != nil {
			return false, fmt.Errorf("JSONPath result error for %q: %w", jsonPathExpr, err)
		}

		match, evalErr := EvaluatePredicate(pred, result.Value)
		if evalErr != nil {
			continue // Skip values that can't be evaluated
		}
		if match {
			return true, nil
		}
	}

	return false, nil
}

// EvaluateJSONPathParserPredicate evaluates a parser.Predicate against JSON data using JSONPath.
// This combines JSONPath execution with parser predicate evaluation.
func EvaluateJSONPathParserPredicate(jsonData []byte, jsonPathExpr string, pred *parser.Predicate) (bool, error) {
	evalPred := &predicate{
		Operation: pred.Operation,
		Value:     pred.Value,
	}

	return EvaluateJSONPathPredicate(jsonData, jsonPathExpr, evalPred)
}

// ValidateJSONPathPredicate validates both a JSONPath expression and a predicate.
// This is useful for upfront validation before execution.
func ValidateJSONPathPredicate(jsonPathExpr string, pred *predicate) error {
	if err := jsonpath.Validate(jsonPathExpr); err != nil {
		return fmt.Errorf("invalid JSONPath expression %q: %w", jsonPathExpr, err)
	}

	if err := pred.validate(); err != nil {
		return fmt.Errorf("invalid predicate: %w", err)
	}

	return nil
}

// jsonPathAssertion combines a JSONPath expression with a predicate for assertion testing.
type jsonPathAssertion struct {
	Path      string
	Predicate *predicate
}

// NewJSONPathAssertion creates a new JSONPath assertion with validation.
func NewJSONPathAssertion(path string, operation string, value any) (*jsonPathAssertion, error) {
	pred, err := NewPredicate(operation, value)
	if err != nil {
		return nil, fmt.Errorf("invalid predicate: %w", err)
	}

	if err := jsonpath.Validate(path); err != nil {
		return nil, fmt.Errorf("invalid JSONPath expression %q: %w", path, err)
	}

	return &jsonPathAssertion{
		Path:      path,
		Predicate: pred,
	}, nil
}

// Evaluate executes the JSONPath assertion against the provided JSON data.
func (ja *jsonPathAssertion) Evaluate(jsonData []byte) (bool, error) {
	return EvaluateJSONPathPredicate(jsonData, ja.Path, ja.Predicate)
}

// compareValues compares two values with type coercion for numeric types.
// This handles cases where YAML might parse integers as uint64 but we're comparing with int,
// and where JSONPath returns json.Number but we're comparing with numeric types.
func compareValues(a, b any) bool {
	if a == b {
		return true
	}

	a = convertJSONNumber(a)
	b = convertJSONNumber(b)

	if a == b {
		return true
	}

	// Handle numeric type conversions
	switch valA := a.(type) {
	case int:
		switch valB := b.(type) {
		case uint64:
			return uint64(valA) == valB
		case int64:
			return int64(valA) == valB
		case float64:
			return float64(valA) == valB
		}
	case uint64:
		switch valB := b.(type) {
		case int:
			return valA == uint64(valB)
		case int64:
			return valA == uint64(valB)
		case float64:
			return float64(valA) == valB
		}
	case int64:
		switch valB := b.(type) {
		case int:
			return valA == int64(valB)
		case uint64:
			return uint64(valA) == valB
		case float64:
			return float64(valA) == valB
		}
	case float64:
		switch valB := b.(type) {
		case int:
			return valA == float64(valB)
		case uint64:
			return float64(valA) == float64(valB)
		case int64:
			return valA == float64(valB)
		}
	}

	return false
}

// convertJSONNumber converts json.Number to appropriate Go type.
func convertJSONNumber(val any) any {
	num, ok := val.(json.Number)
	if !ok {
		return val
	}

	// Try int64 first for integers
	if intVal, err := num.Int64(); err == nil {
		// Convert to int if it fits in int range
		if intVal >= int64(int(^uint(0)>>1)*-1) && intVal <= int64(int(^uint(0)>>1)) {
			return int(intVal)
		}
		return intVal
	}

	// If int64 failed, it might be a large uint64
	numStr := string(num)
	if uintVal, err := strconv.ParseUint(numStr, 10, 64); err == nil {
		return uintVal
	}

	// Try float64 for decimal numbers
	if floatVal, err := num.Float64(); err == nil {
		return floatVal
	}

	return val
}

// getLength returns the length of input if it is a string, array, slice, or map.
// Returns the length and true if successful, otherwise 0 and false.
func getLength(input any) (int, bool) {
	if input == nil {
		return 0, false
	}

	if v, ok := input.(string); ok {
		return len(v), true
	}

	// Use reflection for slices, arrays, and maps
	rv := reflect.ValueOf(input)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len(), true
	default:
		return 0, false
	}
}
