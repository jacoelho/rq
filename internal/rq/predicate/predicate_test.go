package predicate

import (
	"testing"
)

func TestParseOperator(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "supported", input: "equals"},
		{name: "supported_type_is", input: "type_is"},
		{name: "unsupported", input: "bad", wantErr: true},
		{name: "empty", input: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseOperator(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseOperator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateExpr(t *testing.T) {
	tests := []struct {
		name    string
		expr    Expr
		wantErr bool
	}{
		{
			name: "exists_without_value",
			expr: Expr{
				Op: OpExists,
			},
		},
		{
			name: "exists_with_value",
			expr: Expr{
				Op:       OpExists,
				Value:    true,
				HasValue: true,
			},
			wantErr: true,
		},
		{
			name: "equals_without_value",
			expr: Expr{
				Op: OpEquals,
			},
			wantErr: true,
		},
		{
			name: "equals_with_value",
			expr: Expr{
				Op:       OpEquals,
				Value:    "ok",
				HasValue: true,
			},
		},
		{
			name: "type_is_valid",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "array",
				HasValue: true,
			},
		},
		{
			name: "type_is_invalid_value",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "list",
				HasValue: true,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateExpr(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateExpr() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEvaluateExpr(t *testing.T) {
	tests := []struct {
		name      string
		expr      Expr
		actual    any
		want      bool
		wantError bool
	}{
		{
			name: "equals_numeric_cross_type",
			expr: Expr{
				Op:       OpEquals,
				Value:    float64(42),
				HasValue: true,
			},
			actual: int64(42),
			want:   true,
		},
		{
			name: "contains_string",
			expr: Expr{
				Op:       OpContains,
				Value:    "John",
				HasValue: true,
			},
			actual: "John Doe",
			want:   true,
		},
		{
			name: "contains_non_string_actual",
			expr: Expr{
				Op:       OpContains,
				Value:    "John",
				HasValue: true,
			},
			actual:    123,
			wantError: true,
		},
		{
			name: "regex",
			expr: Expr{
				Op:       OpRegex,
				Value:    "^v\\d+",
				HasValue: true,
			},
			actual: "v10",
			want:   true,
		},
		{
			name: "length",
			expr: Expr{
				Op:       OpLength,
				Value:    int64(3),
				HasValue: true,
			},
			actual: []string{"a", "b", "c"},
			want:   true,
		},
		{
			name: "length_float_expected_is_invalid",
			expr: Expr{
				Op:       OpLength,
				Value:    float64(3),
				HasValue: true,
			},
			actual:    []string{"a", "b", "c"},
			wantError: true,
		},
		{
			name: "in_collection",
			expr: Expr{
				Op:       OpIn,
				Value:    []any{"a", "b", "c"},
				HasValue: true,
			},
			actual: "b",
			want:   true,
		},
		{
			name: "in_non_collection",
			expr: Expr{
				Op:       OpIn,
				Value:    "abc",
				HasValue: true,
			},
			actual:    "b",
			wantError: true,
		},
		{
			name: "exists_true",
			expr: Expr{
				Op: OpExists,
			},
			actual: "non-empty",
			want:   true,
		},
		{
			name: "exists_false_for_empty_string",
			expr: Expr{
				Op: OpExists,
			},
			actual: "",
			want:   false,
		},
		{
			name: "type_is_array",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "array",
				HasValue: true,
			},
			actual: []any{"a", "b"},
			want:   true,
		},
		{
			name: "type_is_object",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "object",
				HasValue: true,
			},
			actual: map[string]any{"id": 1},
			want:   true,
		},
		{
			name: "type_is_number",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "number",
				HasValue: true,
			},
			actual: 42,
			want:   true,
		},
		{
			name: "type_is_boolean",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "boolean",
				HasValue: true,
			},
			actual: true,
			want:   true,
		},
		{
			name: "type_is_null",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    "null",
				HasValue: true,
			},
			actual: nil,
			want:   true,
		},
		{
			name: "type_is_invalid_expected_type",
			expr: Expr{
				Op:       OpTypeIs,
				Value:    10,
				HasValue: true,
			},
			actual:    []any{"a"},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := EvaluateExpr(tt.expr, tt.actual)
			if (err != nil) != tt.wantError {
				t.Fatalf("EvaluateExpr() error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && got != tt.want {
				t.Fatalf("EvaluateExpr() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCachedRegexCompilerCachesByPattern(t *testing.T) {
	t.Parallel()

	compiler := newCachedRegexCompiler()

	first, err := compiler.Compile("^a+$")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	second, err := compiler.Compile("^a+$")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}

	if first != second {
		t.Fatalf("Compile() returned different compiled regex pointers for same pattern")
	}

	if _, err := compiler.Compile("[invalid"); err == nil {
		t.Fatal("Compile() expected invalid regex error")
	}
}
