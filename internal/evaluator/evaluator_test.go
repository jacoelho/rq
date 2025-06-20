package evaluator

import (
	"encoding/json"
	"testing"

	"github.com/jacoelho/rq/internal/parser"
)

const (
	testJSONSimple = `{
		"name": "John Doe",
		"age": 30,
		"email": "john@example.com",
		"tags": ["developer", "golang", "testing"],
		"active": true,
		"score": 85.5
	}`

	testJSONComplex = `{
		"user": {
			"id": 123,
			"name": "Alice",
			"preferences": {
				"theme": "dark",
				"notifications": true
			}
		},
		"items": [
			{"name": "item1", "price": 10.99},
			{"name": "item2", "price": 25.50}
		],
		"status": "success", 
		"code": 200
	}`
)

// Test helper for basic predicate operations
type predicateTest struct {
	name      string
	predicate *predicate
	input     any
	expected  bool
	wantError bool
}

// Test helper for JSONPath operations
type jsonPathTest struct {
	name      string
	jsonPath  string
	predicate *predicate
	expected  bool
	expectErr bool
}

func TestEvaluatePredicate(t *testing.T) {
	tests := []predicateTest{
		// Equals operation tests
		{
			name:      "equals_string_match",
			predicate: &predicate{Operation: opEquals, Value: "test"},
			input:     "test",
			expected:  true,
		},
		{
			name:      "equals_string_no_match",
			predicate: &predicate{Operation: opEquals, Value: "test"},
			input:     "different",
			expected:  false,
		},
		{
			name:      "equals_int_match",
			predicate: &predicate{Operation: opEquals, Value: 42},
			input:     42,
			expected:  true,
		},
		{
			name:      "equals_int_no_match",
			predicate: &predicate{Operation: opEquals, Value: 42},
			input:     99,
			expected:  false,
		},

		// Type conversion tests for equals
		{
			name:      "equals_uint64_to_int",
			predicate: &predicate{Operation: opEquals, Value: uint64(200)},
			input:     200,
			expected:  true,
		},
		{
			name:      "equals_int_to_uint64",
			predicate: &predicate{Operation: opEquals, Value: 200},
			input:     uint64(200),
			expected:  true,
		},
		{
			name:      "equals_int64_to_int",
			predicate: &predicate{Operation: opEquals, Value: int64(200)},
			input:     200,
			expected:  true,
		},
		{
			name:      "equals_float64_to_int",
			predicate: &predicate{Operation: opEquals, Value: float64(200)},
			input:     200,
			expected:  true,
		},

		// Not equals operation tests
		{
			name:      "not_equals_string_match",
			predicate: &predicate{Operation: opNotEquals, Value: "test"},
			input:     "different",
			expected:  true,
		},
		{
			name:      "not_equals_string_no_match",
			predicate: &predicate{Operation: opNotEquals, Value: "test"},
			input:     "test",
			expected:  false,
		},
		{
			name:      "not_equals_with_type_conversion",
			predicate: &predicate{Operation: opNotEquals, Value: uint64(200)},
			input:     300,
			expected:  true,
		},

		// Regex operation tests
		{
			name:      "regex_match",
			predicate: &predicate{Operation: opRegex, Value: "^test.*"},
			input:     "testing",
			expected:  true,
		},
		{
			name:      "regex_no_match",
			predicate: &predicate{Operation: opRegex, Value: "^test.*"},
			input:     "hello",
			expected:  false,
		},
		{
			name:      "regex_non_string_input",
			predicate: &predicate{Operation: opRegex, Value: "^12"},
			input:     123,
			expected:  true,
		},
		{
			name:      "regex_status_code_match",
			predicate: &predicate{Operation: opRegex, Value: "20[0-9]"},
			input:     200,
			expected:  true,
		},
		{
			name:      "regex_status_code_no_match",
			predicate: &predicate{Operation: opRegex, Value: "20[0-9]"},
			input:     404,
			expected:  false,
		},

		// Regex error cases
		{
			name:      "regex_invalid_pattern",
			predicate: &predicate{Operation: opRegex, Value: "["},
			input:     "test",
			wantError: true,
		},
		{
			name:      "regex_non_string_pattern",
			predicate: &predicate{Operation: opRegex, Value: 123},
			input:     "test",
			wantError: true,
		},

		// Contains operation tests
		{
			name:      "contains_match",
			predicate: &predicate{Operation: opContains, Value: "est"},
			input:     "testing",
			expected:  true,
		},
		{
			name:      "contains_no_match",
			predicate: &predicate{Operation: opContains, Value: "xyz"},
			input:     "testing",
			expected:  false,
		},
		{
			name:      "contains_empty_string",
			predicate: &predicate{Operation: opContains, Value: ""},
			input:     "testing",
			expected:  true,
		},

		// Contains error cases
		{
			name:      "contains_non_string_input",
			predicate: &predicate{Operation: opContains, Value: "test"},
			input:     123,
			wantError: true,
		},
		{
			name:      "contains_non_string_value",
			predicate: &predicate{Operation: opContains, Value: 123},
			input:     "testing",
			wantError: true,
		},

		// Exists operation tests
		{
			name:      "exists_non_nil",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     "value",
			expected:  true,
		},
		{
			name:      "exists_nil",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     nil,
			expected:  false,
		},
		{
			name:      "exists_zero_value",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     0,
			expected:  true,
		},
		{
			name:      "exists_empty_string",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     "",
			expected:  true,
		},

		// Length operation tests
		{
			name:      "length_string_match",
			predicate: &predicate{Operation: opLength, Value: 4},
			input:     "test",
			expected:  true,
		},
		{
			name:      "length_string_no_match",
			predicate: &predicate{Operation: opLength, Value: 5},
			input:     "test",
			expected:  false,
		},
		{
			name:      "length_array_match",
			predicate: &predicate{Operation: opLength, Value: 3},
			input:     []int{1, 2, 3},
			expected:  true,
		},
		{
			name:      "length_map_match",
			predicate: &predicate{Operation: opLength, Value: 2},
			input:     map[string]int{"a": 1, "b": 2},
			expected:  true,
		},
		{
			name:      "length_slice_match",
			predicate: &predicate{Operation: opLength, Value: 3},
			input:     []string{"a", "b", "c"},
			expected:  true,
		},

		// Length error case
		{
			name:      "length_invalid_value_type",
			predicate: &predicate{Operation: opLength, Value: "invalid"},
			input:     "test",
			wantError: true,
		},
	}

	runPredicateTests(t, tests)
}

// Helper function to run predicate tests
func runPredicateTests(t *testing.T, tests []predicateTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluatePredicate(tt.predicate, tt.input)
			if tt.wantError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Expected %v but got %v", tt.expected, result)
			}
		})
	}
}

func TestNewPredicate(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		value     any
		wantError bool
	}{
		// Valid cases
		{
			name:      "valid_equals",
			operation: opEquals,
			value:     "test",
			wantError: false,
		},
		{
			name:      "valid_regex",
			operation: opRegex,
			value:     "^test.*",
			wantError: false,
		},
		{
			name:      "exists_nil_value_ok",
			operation: opExists,
			value:     nil,
			wantError: false,
		},

		// Error cases
		{
			name:      "invalid_regex_pattern",
			operation: opRegex,
			value:     "[",
			wantError: true,
		},
		{
			name:      "invalid_operation",
			operation: "unknown",
			value:     "test",
			wantError: true,
		},
		{
			name:      "equals_nil_value",
			operation: opEquals,
			value:     nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewPredicate(tt.operation, tt.value)
			if tt.wantError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestIsValidOperation(t *testing.T) {
	tests := []struct {
		operation string
		expected  bool
	}{
		{opEquals, true},
		{opNotEquals, true},
		{opRegex, true},
		{opContains, true},
		{opExists, true},
		{opLength, true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.operation, func(t *testing.T) {
			result := IsValidOperation(tt.operation)
			if result != tt.expected {
				t.Errorf("Expected %v but got %v for operation %q", tt.expected, result, tt.operation)
			}
		})
	}
}

func TestCompareValues(t *testing.T) {
	tests := []struct {
		name     string
		a        any
		b        any
		expected bool
	}{
		// Direct equality
		{
			name:     "direct_string_equality",
			a:        "hello",
			b:        "hello",
			expected: true,
		},
		{
			name:     "direct_int_equality",
			a:        42,
			b:        42,
			expected: true,
		},

		// Numeric type conversions (consolidated to avoid duplication with main tests)
		{
			name:     "int_to_uint64",
			a:        200,
			b:        uint64(200),
			expected: true,
		},
		{
			name:     "uint64_to_int",
			a:        uint64(200),
			b:        200,
			expected: true,
		},
		{
			name:     "int_to_int64",
			a:        200,
			b:        int64(200),
			expected: true,
		},
		{
			name:     "int64_to_int",
			a:        int64(200),
			b:        200,
			expected: true,
		},
		{
			name:     "int_to_float64",
			a:        200,
			b:        float64(200),
			expected: true,
		},
		{
			name:     "float64_to_int",
			a:        float64(200),
			b:        200,
			expected: true,
		},

		// No match cases
		{
			name:     "different_strings",
			a:        "hello",
			b:        "world",
			expected: false,
		},
		{
			name:     "different_numbers",
			a:        42,
			b:        24,
			expected: false,
		},
		{
			name:     "string_vs_number",
			a:        "42",
			b:        42,
			expected: false,
		},

		// json.Number conversion tests
		{
			name:     "json_number_int_to_int",
			a:        json.Number("42"),
			b:        42,
			expected: true,
		},
		{
			name:     "int_to_json_number_int",
			a:        42,
			b:        json.Number("42"),
			expected: true,
		},
		{
			name:     "json_number_float_to_float64",
			a:        json.Number("42.5"),
			b:        42.5,
			expected: true,
		},
		{
			name:     "float64_to_json_number_float",
			a:        42.5,
			b:        json.Number("42.5"),
			expected: true,
		},
		{
			name:     "json_number_to_json_number",
			a:        json.Number("123"),
			b:        json.Number("123"),
			expected: true,
		},
		{
			name:     "json_number_int_to_uint64",
			a:        json.Number("200"),
			b:        uint64(200),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := compareValues(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("compareValues(%v, %v) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestEvaluateJSONPathPredicate(t *testing.T) {
	tests := []jsonPathTest{
		// Basic type matching
		{
			name:      "string_equals_match",
			jsonPath:  "$.name",
			predicate: &predicate{Operation: opEquals, Value: "John Doe"},
			expected:  true,
		},
		{
			name:      "string_equals_no_match",
			jsonPath:  "$.name",
			predicate: &predicate{Operation: opEquals, Value: "Jane Doe"},
			expected:  false,
		},
		{
			name:      "int_equals_match",
			jsonPath:  "$.age",
			predicate: &predicate{Operation: opEquals, Value: 30},
			expected:  true,
		},
		{
			name:      "float_equals_match",
			jsonPath:  "$.score",
			predicate: &predicate{Operation: opEquals, Value: 85.5},
			expected:  true,
		},
		{
			name:      "bool_equals_match",
			jsonPath:  "$.active",
			predicate: &predicate{Operation: opEquals, Value: true},
			expected:  true,
		},

		// Pattern matching
		{
			name:      "email_regex_match",
			jsonPath:  "$.email",
			predicate: &predicate{Operation: opRegex, Value: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`},
			expected:  true,
		},
		{
			name:      "email_contains_match",
			jsonPath:  "$.email",
			predicate: &predicate{Operation: opContains, Value: "example.com"},
			expected:  true,
		},

		// Array operations
		{
			name:      "array_length_match",
			jsonPath:  "$.tags",
			predicate: &predicate{Operation: opLength, Value: 3},
			expected:  true,
		},
		{
			name:      "array_element_equals",
			jsonPath:  "$.tags[0]",
			predicate: &predicate{Operation: opEquals, Value: "developer"},
			expected:  true,
		},
		{
			name:      "array_element_contains",
			jsonPath:  "$.tags[*]",
			predicate: &predicate{Operation: opContains, Value: "lang"},
			expected:  true,
		},

		// Existence checks
		{
			name:      "exists_check_true",
			jsonPath:  "$.name",
			predicate: &predicate{Operation: opExists, Value: nil},
			expected:  true,
		},
		{
			name:      "exists_check_false",
			jsonPath:  "$.nonexistent",
			predicate: &predicate{Operation: opExists, Value: nil},
			expected:  false,
		},

		// Error case
		{
			name:      "invalid_jsonpath",
			jsonPath:  "$.invalid[",
			predicate: &predicate{Operation: opEquals, Value: "test"},
			expectErr: true,
		},
	}

	runJSONPathTests(t, []byte(testJSONSimple), tests)
}

// Helper function to run JSONPath tests
func runJSONPathTests(t *testing.T, testJSON []byte, tests []jsonPathTest) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluateJSONPathPredicate(testJSON, tt.jsonPath, tt.predicate)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestJSONPathAssertion(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		operation string
		value     any
		expected  bool
		expectErr bool
	}{
		// Basic operations on complex JSON
		{
			name:      "user_id_equals",
			path:      "$.user.id",
			operation: opEquals,
			value:     123,
			expected:  true,
		},
		{
			name:      "user_name_contains",
			path:      "$.user.name",
			operation: opContains,
			value:     "lic",
			expected:  true,
		},
		{
			name:      "theme_regex",
			path:      "$.user.preferences.theme",
			operation: opRegex,
			value:     "^(light|dark)$",
			expected:  true,
		},
		{
			name:      "items_length",
			path:      "$.items",
			operation: opLength,
			value:     2,
			expected:  true,
		},
		{
			name:      "item_price_exists",
			path:      "$.items[0].price",
			operation: opExists,
			value:     nil,
			expected:  true,
		},

		// Error cases
		{
			name:      "invalid_predicate",
			path:      "$.user.name",
			operation: "invalid_op",
			value:     "test",
			expectErr: true,
		},
		{
			name:      "invalid_jsonpath",
			path:      "$.invalid[",
			operation: opEquals,
			value:     "test",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertion, err := NewJSONPathAssertion(tt.path, tt.operation, tt.value)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error during assertion creation but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error during assertion creation: %v", err)
				return
			}

			result, err := assertion.Evaluate([]byte(testJSONComplex))
			if err != nil {
				t.Errorf("unexpected error during evaluation: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestEvaluateJSONPathParserPredicate(t *testing.T) {
	tests := []struct {
		name      string
		jsonPath  string
		predicate *parser.Predicate
		expected  bool
		expectErr bool
	}{
		{
			name:     "status_equals",
			jsonPath: "$.status",
			predicate: &parser.Predicate{
				Operation: opEquals,
				Value:     "success",
			},
			expected: true,
		},
		{
			name:     "code_equals",
			jsonPath: "$.code",
			predicate: &parser.Predicate{
				Operation: opEquals,
				Value:     200,
			},
			expected: true,
		},
		{
			name:     "status_not_equals",
			jsonPath: "$.status",
			predicate: &parser.Predicate{
				Operation: opNotEquals,
				Value:     "error",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluateJSONPathParserPredicate([]byte(testJSONComplex), tt.jsonPath, tt.predicate)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestValidateJSONPathPredicate(t *testing.T) {
	tests := []struct {
		name      string
		jsonPath  string
		predicate *predicate
		expectErr bool
	}{
		{
			name:      "valid_path_and_predicate",
			jsonPath:  "$.user.name",
			predicate: &predicate{Operation: opEquals, Value: "test"},
			expectErr: false,
		},
		{
			name:      "invalid_jsonpath",
			jsonPath:  "$.invalid[",
			predicate: &predicate{Operation: opEquals, Value: "test"},
			expectErr: true,
		},
		{
			name:      "invalid_predicate",
			jsonPath:  "$.user.name",
			predicate: &predicate{Operation: "invalid", Value: "test"},
			expectErr: true,
		},
		{
			name:      "invalid_regex_predicate",
			jsonPath:  "$.user.name",
			predicate: &predicate{Operation: opRegex, Value: "["},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateJSONPathPredicate(tt.jsonPath, tt.predicate)

			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestConvertJSONNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected any
	}{
		{
			name:     "json_number_int",
			input:    json.Number("42"),
			expected: 42,
		},
		{
			name:     "json_number_negative_int",
			input:    json.Number("-42"),
			expected: -42,
		},
		{
			name:     "json_number_float",
			input:    json.Number("42.5"),
			expected: 42.5,
		},
		{
			name:     "json_number_large_int",
			input:    json.Number("9223372036854775807"), // max int64
			expected: 9223372036854775807,                // This will be int on 64-bit systems
		},
		{
			name:     "json_number_large_uint",
			input:    json.Number("18446744073709551615"), // max uint64
			expected: uint64(18446744073709551615),
		},
		{
			name:     "regular_int",
			input:    42,
			expected: 42,
		},
		{
			name:     "regular_string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "nil_value",
			input:    nil,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertJSONNumber(tt.input)
			if result != tt.expected {
				t.Errorf("expected %v (%T), got %v (%T)", tt.expected, tt.expected, result, result)
			}
		})
	}
}

func TestHeaderAssertions(t *testing.T) {
	t.Parallel()

	tests := []predicateTest{
		// Content-Type header tests (from assert_json.yaml)
		{
			name:      "content_type_contains_application_json",
			predicate: &predicate{Operation: opContains, Value: "application/json"},
			input:     "application/json; charset=utf-8",
			expected:  true,
		},
		{
			name:      "content_type_contains_json_partial",
			predicate: &predicate{Operation: opContains, Value: "json"},
			input:     "application/json",
			expected:  true,
		},
		{
			name:      "content_type_does_not_contain_xml",
			predicate: &predicate{Operation: opContains, Value: "xml"},
			input:     "application/json; charset=utf-8",
			expected:  false,
		},

		// Authorization header tests
		{
			name:      "authorization_bearer_token",
			predicate: &predicate{Operation: opContains, Value: "Bearer"},
			input:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected:  true,
		},
		{
			name:      "authorization_basic_auth",
			predicate: &predicate{Operation: opContains, Value: "Basic"},
			input:     "Basic dXNlcjpwYXNzd29yZA==",
			expected:  true,
		},

		// Cache headers
		{
			name:      "cache_control_no_cache",
			predicate: &predicate{Operation: opContains, Value: "no-cache"},
			input:     "no-cache, no-store, must-revalidate",
			expected:  true,
		},
		{
			name:      "cache_control_max_age",
			predicate: &predicate{Operation: opRegex, Value: "max-age=\\d+"},
			input:     "public, max-age=3600",
			expected:  true,
		},

		// Custom headers existence
		{
			name:      "custom_header_exists",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     "custom-value",
			expected:  true,
		},
		{
			name:      "missing_header_not_exists",
			predicate: &predicate{Operation: opExists, Value: nil},
			input:     nil,
			expected:  false,
		},

		// Server headers
		{
			name:      "server_nginx",
			predicate: &predicate{Operation: opContains, Value: "nginx"},
			input:     "nginx/1.18.0 (Ubuntu)",
			expected:  true,
		},
		{
			name:      "server_version_regex",
			predicate: &predicate{Operation: opRegex, Value: "nginx/\\d+\\.\\d+\\.\\d+"},
			input:     "nginx/1.18.0 (Ubuntu)",
			expected:  true,
		},

		// CORS headers
		{
			name:      "cors_allow_origin_wildcard",
			predicate: &predicate{Operation: opEquals, Value: "*"},
			input:     "*",
			expected:  true,
		},
		{
			name:      "cors_allow_methods_contains_post",
			predicate: &predicate{Operation: opContains, Value: "POST"},
			input:     "GET, POST, PUT, DELETE",
			expected:  true,
		},

		// Content-Length header
		{
			name:      "content_length_numeric",
			predicate: &predicate{Operation: opRegex, Value: "^\\d+$"},
			input:     "1234",
			expected:  true,
		},
		{
			name:      "content_length_specific_value",
			predicate: &predicate{Operation: opEquals, Value: "0"},
			input:     "0",
			expected:  true,
		},
	}

	runPredicateTests(t, tests)
}

func TestHTTPStatusCodeAssertions(t *testing.T) {
	t.Parallel()

	tests := []predicateTest{
		// Success status codes
		{
			name:      "status_200_ok",
			predicate: &predicate{Operation: opEquals, Value: 200},
			input:     200,
			expected:  true,
		},
		{
			name:      "status_201_created",
			predicate: &predicate{Operation: opEquals, Value: 201},
			input:     201,
			expected:  true,
		},
		{
			name:      "status_2xx_success_range",
			predicate: &predicate{Operation: opRegex, Value: "^2\\d{2}$"},
			input:     204,
			expected:  true,
		},

		// Client error status codes
		{
			name:      "status_404_not_found",
			predicate: &predicate{Operation: opEquals, Value: 404},
			input:     404,
			expected:  true,
		},
		{
			name:      "status_4xx_client_error_range",
			predicate: &predicate{Operation: opRegex, Value: "^4\\d{2}$"},
			input:     400,
			expected:  true,
		},

		// Server error status codes
		{
			name:      "status_500_internal_error",
			predicate: &predicate{Operation: opEquals, Value: 500},
			input:     500,
			expected:  true,
		},
		{
			name:      "status_5xx_server_error_range",
			predicate: &predicate{Operation: opRegex, Value: "^5\\d{2}$"},
			input:     503,
			expected:  true,
		},

		// Status code as string (some HTTP libraries return as string)
		{
			name:      "status_200_as_string",
			predicate: &predicate{Operation: opEquals, Value: "200"},
			input:     "200",
			expected:  true,
		},
		{
			name:      "status_regex_on_string",
			predicate: &predicate{Operation: opRegex, Value: "^2\\d{2}$"},
			input:     "201",
			expected:  true,
		},
	}

	runPredicateTests(t, tests)
}

func TestJSONPathResponseAssertions(t *testing.T) {
	t.Parallel()

	// Test JSON data similar to what httpbin.org returns
	testJSON := []byte(`{
		"uuid": "550e8400-e29b-41d4-a716-446655440000",
		"origin": "192.168.1.100",
		"url": "https://httpbin.org/get",
		"headers": {
			"Host": "httpbin.org",
			"User-Agent": "rq/1.0",
			"Accept": "application/json",
			"Content-Type": "application/json; charset=utf-8"
		},
		"args": {},
		"data": "",
		"json": null,
		"files": {}
	}`)

	tests := []jsonPathTest{
		// UUID validation (from assert_json.yaml)
		{
			name:      "uuid_format_validation",
			jsonPath:  "$.uuid",
			predicate: &predicate{Operation: opRegex, Value: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"},
			expected:  true,
		},

		// IP address validation (from assert_json.yaml)
		{
			name:      "ip_address_format",
			jsonPath:  "$.origin",
			predicate: &predicate{Operation: opRegex, Value: "^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}"},
			expected:  true,
		},

		// Host header validation (from assert_json.yaml)
		{
			name:      "host_header_equals",
			jsonPath:  "$.headers.Host",
			predicate: &predicate{Operation: opEquals, Value: "httpbin.org"},
			expected:  true,
		},

		// Content-Type header contains check
		{
			name:      "content_type_header_contains",
			jsonPath:  "$.headers.Content-Type",
			predicate: &predicate{Operation: opContains, Value: "application/json"},
			expected:  true,
		},

		// URL validation
		{
			name:      "url_contains_httpbin",
			jsonPath:  "$.url",
			predicate: &predicate{Operation: opContains, Value: "httpbin.org"},
			expected:  true,
		},

		// User-Agent validation
		{
			name:      "user_agent_contains_rq",
			jsonPath:  "$.headers.User-Agent",
			predicate: &predicate{Operation: opContains, Value: "rq"},
			expected:  true,
		},

		// Null field existence
		{
			name:      "json_field_exists_but_null",
			jsonPath:  "$.json",
			predicate: &predicate{Operation: opExists, Value: nil},
			expected:  false, // null values should return false for exists
		},

		// Empty object/array checks
		{
			name:      "args_object_length_zero",
			jsonPath:  "$.args",
			predicate: &predicate{Operation: opLength, Value: 0},
			expected:  true,
		},
		{
			name:      "files_object_length_zero",
			jsonPath:  "$.files",
			predicate: &predicate{Operation: opLength, Value: 0},
			expected:  true,
		},
	}

	runJSONPathTests(t, testJSON, tests)
}

func TestComplexJSONPathScenarios(t *testing.T) {
	t.Parallel()

	// More complex JSON structure for advanced testing
	complexJSON := []byte(`{
		"services": [
			{"name": "auth", "status": "healthy", "port": 8080},
			{"name": "api", "status": "healthy", "port": 8081},
			{"name": "db", "status": "degraded", "port": 5432}
		],
		"metadata": {
			"version": "1.2.3",
			"environment": "production",
			"features": ["auth", "api", "monitoring"]
		},
		"health": {
			"overall": "healthy",
			"checks": {
				"database": {"status": "ok", "response_time": "5ms"},
				"cache": {"status": "ok", "response_time": "1ms"},
				"external_api": {"status": "timeout", "response_time": "30s"}
			}
		}
	}`)

	tests := []jsonPathTest{
		// Array length validation
		{
			name:      "services_array_length",
			jsonPath:  "$.services",
			predicate: &predicate{Operation: opLength, Value: 3},
			expected:  true,
		},

		// Nested object access
		{
			name:      "version_format",
			jsonPath:  "$.metadata.version",
			predicate: &predicate{Operation: opRegex, Value: "^\\d+\\.\\d+\\.\\d+$"},
			expected:  true,
		},

		// Array element contains
		{
			name:      "features_contains_auth",
			jsonPath:  "$.metadata.features[*]",
			predicate: &predicate{Operation: opEquals, Value: "auth"},
			expected:  true,
		},

		// Service status checks
		{
			name:      "all_services_have_ports",
			jsonPath:  "$.services[*].port",
			predicate: &predicate{Operation: opExists, Value: nil},
			expected:  true,
		},

		// Deep nested access
		{
			name:      "database_check_ok",
			jsonPath:  "$.health.checks.database.status",
			predicate: &predicate{Operation: opEquals, Value: "ok"},
			expected:  true,
		},

		// Response time validation
		{
			name:      "cache_response_time_format",
			jsonPath:  "$.health.checks.cache.response_time",
			predicate: &predicate{Operation: opRegex, Value: "\\d+ms"},
			expected:  true,
		},

		// Environment validation
		{
			name:      "production_environment",
			jsonPath:  "$.metadata.environment",
			predicate: &predicate{Operation: opEquals, Value: "production"},
			expected:  true,
		},

		// Service name contains
		{
			name:      "service_name_contains_api",
			jsonPath:  "$.services[*].name",
			predicate: &predicate{Operation: opEquals, Value: "api"},
			expected:  true,
		},
	}

	runJSONPathTests(t, complexJSON, tests)
}

func TestRealWorldHTTPScenarios(t *testing.T) {
	t.Parallel()

	tests := []predicateTest{
		// JWT token validation
		{
			name:      "jwt_token_format",
			predicate: &predicate{Operation: opRegex, Value: "^[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+\\.[A-Za-z0-9-_]+$"},
			input:     "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			expected:  true,
		},

		// API key validation
		{
			name:      "api_key_length",
			predicate: &predicate{Operation: opLength, Value: 32},
			input:     "abcdef1234567890abcdef1234567890",
			expected:  true,
		},

		// Session ID format
		{
			name:      "session_id_hex_format",
			predicate: &predicate{Operation: opRegex, Value: "^[a-f0-9]{40}$"},
			input:     "a1b2c3d4e5f6789012345678901234567890abcd",
			expected:  true,
		},

		// MIME type validation
		{
			name:      "mime_type_image",
			predicate: &predicate{Operation: opRegex, Value: "^image/(jpeg|png|gif|webp)$"},
			input:     "image/jpeg",
			expected:  true,
		},

		// HTTP method validation
		{
			name:      "http_method_uppercase",
			predicate: &predicate{Operation: opRegex, Value: "^[A-Z]+$"},
			input:     "GET",
			expected:  true,
		},

		// Timestamp format validation
		{
			name:      "iso8601_timestamp",
			predicate: &predicate{Operation: opRegex, Value: "^\\d{4}-\\d{2}-\\d{2}T\\d{2}:\\d{2}:\\d{2}"},
			input:     "2023-12-07T10:30:45Z",
			expected:  true,
		},

		// Error code format
		{
			name:      "error_code_format",
			predicate: &predicate{Operation: opRegex, Value: "^E\\d{4}$"},
			input:     "E1001",
			expected:  true,
		},

		// Pagination parameters
		{
			name:      "page_number_positive",
			predicate: &predicate{Operation: opRegex, Value: "^[1-9]\\d*$"},
			input:     "1",
			expected:  true,
		},

		// Rate limit headers
		{
			name:      "rate_limit_remaining",
			predicate: &predicate{Operation: opRegex, Value: "^\\d+$"},
			input:     "99",
			expected:  true,
		},

		// Request ID tracing
		{
			name:      "request_id_uuid_format",
			predicate: &predicate{Operation: opRegex, Value: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"},
			input:     "123e4567-e89b-12d3-a456-426614174000",
			expected:  true,
		},
	}

	runPredicateTests(t, tests)
}

func TestAssertJSONYamlScenarios(t *testing.T) {
	t.Parallel()

	// Test the exact scenarios from assert_json.yaml
	tests := []struct {
		name        string
		description string
		predicate   *predicate
		input       any
		expected    bool
	}{
		{
			name:        "content_type_header_from_assert_json_yaml",
			description: "Test the exact Content-Type contains application/json from assert_json.yaml",
			predicate:   &predicate{Operation: opContains, Value: "application/json"},
			input:       "application/json; charset=utf-8",
			expected:    true,
		},
		{
			name:        "status_code_200_from_assert_json_yaml",
			description: "Test status code 200 equals check from assert_json.yaml",
			predicate:   &predicate{Operation: opEquals, Value: 200},
			input:       200,
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluatePredicate(tt.predicate, tt.input)
			if err != nil {
				t.Fatalf("EvaluatePredicate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("EvaluatePredicate(%+v, %v) = %v, want %v", tt.predicate, tt.input, result, tt.expected)
			}
		})
	}
}

func TestAssertJSONYamlJSONPathScenarios(t *testing.T) {
	t.Parallel()

	// Test JSON similar to httpbin.org responses used in assert_json.yaml
	httpbinJSON := []byte(`{
		"uuid": "550e8400-e29b-41d4-a716-446655440000",
		"origin": "203.0.113.42",
		"headers": {
			"Host": "httpbin.org",
			"User-Agent": "rq/1.0",
			"Accept": "*/*"
		}
	}`)

	tests := []struct {
		name        string
		description string
		jsonPath    string
		predicate   *predicate
		expected    bool
	}{
		{
			name:        "uuid_regex_from_assert_json_yaml",
			description: "Test UUID regex pattern from assert_json.yaml",
			jsonPath:    "$.uuid",
			predicate:   &predicate{Operation: opRegex, Value: "^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$"},
			expected:    true,
		},
		{
			name:        "ip_address_regex_from_assert_json_yaml",
			description: "Test IP address regex pattern from assert_json.yaml",
			jsonPath:    "$.origin",
			predicate:   &predicate{Operation: opRegex, Value: "^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}"},
			expected:    true,
		},
		{
			name:        "host_header_equals_from_assert_json_yaml",
			description: "Test Host header equals httpbin.org from assert_json.yaml",
			jsonPath:    "$.headers.Host",
			predicate:   &predicate{Operation: opEquals, Value: "httpbin.org"},
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := EvaluateJSONPathPredicate(httpbinJSON, tt.jsonPath, tt.predicate)
			if err != nil {
				t.Fatalf("EvaluateJSONPathPredicate failed: %v", err)
			}
			if result != tt.expected {
				t.Errorf("EvaluateJSONPathPredicate(jsonPath=%s, predicate=%+v) = %v, want %v",
					tt.jsonPath, tt.predicate, result, tt.expected)
			}
		})
	}
}
