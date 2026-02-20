package expr

import "testing"

func TestEval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		expr      string
		variables map[string]any
		want      bool
		wantErr   bool
	}{
		{
			name: "numeric_equals",
			expr: "status_code == 200",
			variables: map[string]any{
				"status_code": int64(200),
			},
			want: true,
		},
		{
			name: "boolean_identifier",
			expr: "status_code == 200 && is_ready",
			variables: map[string]any{
				"status_code": 200,
				"is_ready":    true,
			},
			want: true,
		},
		{
			name: "precedence_and_before_or",
			expr: "a || b && c",
			variables: map[string]any{
				"a": false,
				"b": true,
				"c": false,
			},
			want: false,
		},
		{
			name: "parentheses",
			expr: "(a || b) && c",
			variables: map[string]any{
				"a": false,
				"b": true,
				"c": true,
			},
			want: true,
		},
		{
			name: "not_operator",
			expr: "!is_ready",
			variables: map[string]any{
				"is_ready": false,
			},
			want: true,
		},
		{
			name: "null_comparison",
			expr: "token == null",
			variables: map[string]any{
				"token": nil,
			},
			want: true,
		},
		{
			name: "unknown_variable",
			expr: "missing == true",
			variables: map[string]any{
				"known": true,
			},
			wantErr: true,
		},
		{
			name: "type_mismatch",
			expr: "status_code == '200'",
			variables: map[string]any{
				"status_code": 200,
			},
			wantErr: true,
		},
		{
			name: "non_boolean_root",
			expr: "status_code",
			variables: map[string]any{
				"status_code": 200,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Eval(tt.expr, tt.variables)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Eval() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got != tt.want {
				t.Fatalf("Eval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{name: "valid", expr: "status_code == 200 && is_ready", wantErr: false},
		{name: "empty", expr: "   ", wantErr: true},
		{name: "missing_right_operand", expr: "status_code ==", wantErr: true},
		{name: "missing_closing_paren", expr: "(status_code == 200", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateBoolean(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		expr    string
		wantErr bool
	}{
		{name: "identifier", expr: "is_ready", wantErr: false},
		{name: "comparison", expr: "status_code == 200", wantErr: false},
		{name: "boolean_literal", expr: "true", wantErr: false},
		{name: "not_expression", expr: "!is_ready", wantErr: false},
		{name: "number_literal", expr: "1", wantErr: true},
		{name: "string_literal", expr: "'ok'", wantErr: true},
		{name: "null_literal", expr: "null", wantErr: true},
		{name: "invalid_boolean_operand", expr: "is_ready && 1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateBoolean(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateBoolean() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
