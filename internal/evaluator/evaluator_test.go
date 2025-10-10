package evaluator

import (
	"slices"
	"testing"

	"github.com/jacoelho/rq/internal/parser"
)

func TestParseOperation(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Operation
		wantErr  bool
	}{
		{"equals", "equals", OpEquals, false},
		{"not_equals", "not_equals", OpNotEquals, false},
		{"contains", "contains", OpContains, false},
		{"regex", "regex", OpRegex, false},
		{"exists", "exists", OpExists, false},
		{"length", "length", OpLength, false},
		{"greater_than", "greater_than", OpGreaterThan, false},
		{"less_than", "less_than", OpLessThan, false},
		{"greater_than_or_equal", "greater_than_or_equal", OpGreaterThanOrEqual, false},
		{"less_than_or_equal", "less_than_or_equal", OpLessThanOrEqual, false},
		{"starts_with", "starts_with", OpStartsWith, false},
		{"ends_with", "ends_with", OpEndsWith, false},
		{"not_contains", "not_contains", OpNotContains, false},
		{"in", "in", OpIn, false},
		{"unsupported", "unsupported", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseOperation(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseOperation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("ParseOperation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluate(t *testing.T) {
	tests := []struct {
		name     string
		op       Operation
		actual   any
		expected any
		want     bool
		wantErr  bool
	}{
		// Equals tests
		{"equals string", OpEquals, "test", "test", true, false},
		{"equals different strings", OpEquals, "test", "other", false, false},
		{"equals number", OpEquals, int64(42), int64(42), true, false},
		{"equals different numbers", OpEquals, int64(42), int64(43), false, false},
		{"equals numeric types", OpEquals, int64(42), float64(42), true, false},

		// Not equals tests
		{"not_equals string", OpNotEquals, "test", "other", true, false},
		{"not_equals same string", OpNotEquals, "test", "test", false, false},

		// Contains tests
		{"contains string", OpContains, "this is a test string", "test", true, false},
		{"contains not found", OpContains, "this is a test string", "xyz", false, false},

		// Regex tests
		{"regex match", OpRegex, "test string", "^test.*", true, false},
		{"regex no match", OpRegex, "other string", "^test.*", false, false},
		{"regex invalid pattern", OpRegex, "test", "[invalid", false, true},
		{"regex non-string pattern", OpRegex, "test", 123, false, true},

		// Exists tests
		{"exists non-nil", OpExists, "test", nil, true, false},
		{"exists nil", OpExists, nil, nil, false, false},
		{"exists empty string", OpExists, "", nil, false, false},
		{"exists non-empty string", OpExists, "test", nil, true, false},
		{"exists empty slice", OpExists, []string{}, nil, false, false},
		{"exists non-empty slice", OpExists, []string{"a"}, nil, true, false},

		// Length tests - simplified to normalized types only
		{"length string", OpLength, "test", int64(4), true, false},
		{"length different string", OpLength, "test", int64(5), false, false},
		{"length slice", OpLength, []string{"a", "b", "c"}, int64(3), true, false},

		// Test normalized types (what the parser actually produces after normalization)
		{"length with int64", OpLength, "test", int64(4), true, false},
		{"length with float64", OpLength, "test", float64(4), true, false},

		// Edge cases with normalized types
		{"length with large int64", OpLength, []int{1, 2, 3, 4, 5}, int64(5), true, false},
		{"length zero int64", OpLength, "", int64(0), true, false},

		// Error cases
		{"length invalid expected", OpLength, "test", "not_int", false, true},
		{"length invalid actual", OpLength, int64(42), int64(2), false, true},

		// Numeric comparison tests
		{"greater_than int64", OpGreaterThan, int64(5), int64(3), true, false},
		{"greater_than float64", OpGreaterThan, float64(5.5), float64(3.2), true, false},
		{"greater_than mixed types", OpGreaterThan, int64(5), float64(3.5), true, false},
		{"greater_than false", OpGreaterThan, int64(3), int64(5), false, false},
		{"greater_than equal", OpGreaterThan, int64(5), int64(5), false, false},
		{"greater_than non-numeric", OpGreaterThan, "test", int64(5), false, true},

		{"less_than int64", OpLessThan, int64(3), int64(5), true, false},
		{"less_than float64", OpLessThan, float64(3.2), float64(5.5), true, false},
		{"less_than mixed types", OpLessThan, int64(3), float64(5.5), true, false},
		{"less_than false", OpLessThan, int64(5), int64(3), false, false},
		{"less_than equal", OpLessThan, int64(5), int64(5), false, false},
		{"less_than non-numeric", OpLessThan, "test", int64(5), false, true},

		{"greater_than_or_equal int64", OpGreaterThanOrEqual, int64(5), int64(3), true, false},
		{"greater_than_or_equal equal", OpGreaterThanOrEqual, int64(5), int64(5), true, false},
		{"greater_than_or_equal false", OpGreaterThanOrEqual, int64(3), int64(5), false, false},
		{"greater_than_or_equal non-numeric", OpGreaterThanOrEqual, "test", int64(5), false, true},

		{"less_than_or_equal int64", OpLessThanOrEqual, int64(3), int64(5), true, false},
		{"less_than_or_equal equal", OpLessThanOrEqual, int64(5), int64(5), true, false},
		{"less_than_or_equal false", OpLessThanOrEqual, int64(5), int64(3), false, false},
		{"less_than_or_equal non-numeric", OpLessThanOrEqual, "test", int64(5), false, true},

		// String operation tests
		{"starts_with match", OpStartsWith, "hello world", "hello", true, false},
		{"starts_with no match", OpStartsWith, "hello world", "world", false, false},
		{"starts_with empty prefix", OpStartsWith, "hello world", "", true, false},
		{"starts_with empty string", OpStartsWith, "", "hello", false, false},

		{"ends_with match", OpEndsWith, "hello world", "world", true, false},
		{"ends_with no match", OpEndsWith, "hello world", "hello", false, false},
		{"ends_with empty suffix", OpEndsWith, "hello world", "", true, false},
		{"ends_with empty string", OpEndsWith, "", "world", false, false},

		{"not_contains match", OpNotContains, "hello world", "xyz", true, false},
		{"not_contains no match", OpNotContains, "hello world", "hello", false, false},
		{"not_contains empty string", OpNotContains, "hello world", "", false, false},
		{"not_contains empty actual", OpNotContains, "", "hello", true, false},

		// Collection operation tests
		{"in string slice", OpIn, "apple", []string{"apple", "banana", "orange"}, true, false},
		{"in string slice not found", OpIn, "grape", []string{"apple", "banana", "orange"}, false, false},
		{"in int64 slice", OpIn, int64(2), []int64{1, 2, 3}, true, false},
		{"in int64 slice not found", OpIn, int64(4), []int64{1, 2, 3}, false, false},
		{"in float64 slice", OpIn, float64(2.5), []float64{1.5, 2.5, 3.5}, true, false},
		{"in any slice", OpIn, "test", []any{"hello", "test", "world"}, true, false},
		{"in empty slice", OpIn, "test", []string{}, false, false},
		{"in non-collection", OpIn, "test", "not_a_slice", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(tt.op, tt.actual, tt.expected)
			if (err != nil) != tt.wantErr {
				t.Errorf("Evaluate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.want {
				t.Errorf("Evaluate() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestEvaluateUnsupportedOperation(t *testing.T) {
	result, err := Evaluate("invalid", "test", "test")
	if err == nil {
		t.Error("Expected error for unsupported operation")
	}
	if result {
		t.Error("Expected false result for error case")
	}
}

func TestEvaluateJSONPathParserPredicate(t *testing.T) {
	testJSON := `{
		"user": {
			"name": "John Doe",
			"age": 30
		},
		"items": ["apple", "banana", "orange"],
		"active": true
	}`

	tests := []struct {
		name      string
		path      string
		predicate *parser.Predicate
		want      bool
		wantErr   bool
	}{
		{
			name: "string equals",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "equals",
				Value:     "John Doe",
			},
			want: true,
		},
		{
			name: "string not equals",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "equals",
				Value:     "Jane Doe",
			},
			want: false,
		},
		{
			name: "number equals",
			path: "$.user.age",
			predicate: &parser.Predicate{
				Operation: "equals",
				Value:     float64(30), // JSON numbers are float64
			},
			want: true,
		},
		{
			name: "contains",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "contains",
				Value:     "John",
			},
			want: true,
		},
		{
			name: "regex match",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "regex",
				Value:     "^John.*",
			},
			want: true,
		},
		{
			name: "exists true",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "exists",
				Value:     nil,
			},
			want: true,
		},
		{
			name: "exists false",
			path: "$.user.nonexistent",
			predicate: &parser.Predicate{
				Operation: "exists",
				Value:     nil,
			},
			want: false,
		},
		{
			name: "length",
			path: "$.items",
			predicate: &parser.Predicate{
				Operation: "length",
				Value:     int64(3),
			},
			want: true,
		},
		{
			name:    "invalid json path",
			path:    "$.invalid.[",
			wantErr: true,
		},
		{
			name: "no results non-exists",
			path: "$.nonexistent",
			predicate: &parser.Predicate{
				Operation: "equals",
				Value:     "test",
			},
			wantErr: true,
		},
		{
			name: "invalid operation",
			path: "$.user.name",
			predicate: &parser.Predicate{
				Operation: "unsupported",
				Value:     "test",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluateJSONPathParserPredicate([]byte(testJSON), tt.path, tt.predicate)
			if (err != nil) != tt.wantErr {
				t.Errorf("EvaluateJSONPathParserPredicate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != tt.want {
				t.Errorf("EvaluateJSONPathParserPredicate() = %v, want %v", result, tt.want)
			}
		})
	}
}

func TestGetSupportedOperations(t *testing.T) {
	operations := GetSupportedOperations()
	expected := []string{
		"equals", "not_equals", "contains", "regex", "exists", "length",
		"greater_than", "less_than", "greater_than_or_equal", "less_than_or_equal",
		"starts_with", "ends_with", "not_contains", "in",
	}

	if len(operations) != len(expected) {
		t.Errorf("GetSupportedOperations() length = %d, want %d", len(operations), len(expected))
	}

	for _, op := range expected {
		found := slices.Contains(operations, op)
		if !found {
			t.Errorf("GetSupportedOperations() missing operation: %s", op)
		}
	}
}

func TestIsSupportedOperation(t *testing.T) {
	tests := []struct {
		operation string
		want      bool
	}{
		{"equals", true},
		{"not_equals", true},
		{"contains", true},
		{"regex", true},
		{"exists", true},
		{"length", true},
		{"greater_than", true},
		{"less_than", true},
		{"greater_than_or_equal", true},
		{"less_than_or_equal", true},
		{"starts_with", true},
		{"ends_with", true},
		{"not_contains", true},
		{"in", true},
		{"unsupported", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			result := IsSupportedOperation(tt.operation)
			if result != tt.want {
				t.Errorf("IsSupportedOperation(%s) = %v, want %v", tt.operation, result, tt.want)
			}
		})
	}
}

func TestOperationString(t *testing.T) {
	tests := []struct {
		op   Operation
		want string
	}{
		{OpEquals, "equals"},
		{OpNotEquals, "not_equals"},
		{OpContains, "contains"},
		{OpRegex, "regex"},
		{OpExists, "exists"},
		{OpLength, "length"},
		{OpGreaterThan, "greater_than"},
		{OpLessThan, "less_than"},
		{OpGreaterThanOrEqual, "greater_than_or_equal"},
		{OpLessThanOrEqual, "less_than_or_equal"},
		{OpStartsWith, "starts_with"},
		{OpEndsWith, "ends_with"},
		{OpNotContains, "not_contains"},
		{OpIn, "in"},
	}

	for _, tt := range tests {
		t.Run(string(tt.op), func(t *testing.T) {
			if tt.op.String() != tt.want {
				t.Errorf("Operation.String() = %v, want %v", tt.op.String(), tt.want)
			}
		})
	}
}

// Test helper functions

func TestEvaluateEquals(t *testing.T) {
	tests := []struct {
		name     string
		actual   any
		expected any
		want     bool
	}{
		{"same strings", "test", "test", true},
		{"different strings", "test", "other", false},
		{"same numbers", int64(42), int64(42), true},
		{"different numbers", int64(42), int64(43), false},
		{"numeric types", int64(42), float64(42), true},
		{"nil values", nil, nil, true},
		{"nil vs non-nil", nil, "test", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateEquals(tt.actual, tt.expected)
			if result != tt.want {
				t.Errorf("evaluateEquals(%v, %v) = %v, want %v", tt.actual, tt.expected, result, tt.want)
			}
		})
	}
}

func TestEvaluateExists(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"nil value", nil, false},
		{"empty string", "", false},
		{"non-empty string", "test", true},
		{"empty slice", []string{}, false},
		{"non-empty slice", []string{"a"}, true},
		{"empty map", map[string]string{}, false},
		{"non-empty map", map[string]string{"a": "b"}, true},
		{"zero int", 0, true},
		{"non-zero int", 42, true},
		{"false bool", false, true},
		{"true bool", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateExists(tt.value)
			if result != tt.want {
				t.Errorf("evaluateExists(%v) = %v, want %v", tt.value, result, tt.want)
			}
		})
	}
}

func TestConvertToInt(t *testing.T) {
	tests := []struct {
		name    string
		value   any
		want    int
		wantErr bool
	}{
		// Normalized types (what the parser produces after normalization)
		{"int64", int64(42), 42, false},
		{"float64", float64(42.0), 42, false},
		{"int64 zero", int64(0), 0, false},
		{"int64 negative", int64(-10), -10, false},
		{"int64 large", int64(999999), 999999, false},
		{"float64 zero", float64(0.0), 0, false},

		// String conversions
		{"string number", "42", 42, false},
		{"string zero", "0", 0, false},
		{"string negative", "-10", -10, false},
		{"string non-number", "abc", 0, true},

		// Error cases - types that shouldn't appear after normalization
		{"unsupported type", []string{"a"}, 0, true},
		{"raw int not normalized", 42, 0, true},            // Should be int64 after normalization
		{"raw uint64 not normalized", uint64(42), 0, true}, // Should be int64 after normalization
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToInt(tt.value)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertToInt() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && result != tt.want {
				t.Errorf("convertToInt() = %v, want %v", result, tt.want)
			}
		})
	}
}
