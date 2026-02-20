package template

import "testing"

func TestNormalize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple_variable",
			input: "Bearer {{token}}",
			want:  "Bearer {{.token}}",
		},
		{
			name:  "nested_variable",
			input: "{{ user.id }}",
			want:  "{{.user.id}}",
		},
		{
			name:  "already_normalized",
			input: "{{.session_token}}",
			want:  "{{.session_token}}",
		},
		{
			name:  "expression_unchanged",
			input: "{{ value | upper }}",
			want:  "{{value | upper}}",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := Normalize(tt.input)
			if got != tt.want {
				t.Fatalf("Normalize() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeDetailed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		want      string
		wantDiag  bool
		wantInner string
	}{
		{
			name:      "maps_timestamp_dynamic_variable",
			input:     "id={{$timestamp}}",
			want:      "id={{timestamp}}",
			wantDiag:  false,
			wantInner: "",
		},
		{
			name:      "maps_guid_dynamic_variable",
			input:     "trace={{$guid}}",
			want:      "trace={{uuidv4}}",
			wantDiag:  false,
			wantInner: "",
		},
		{
			name:      "unsupported_hyphenated_placeholder",
			input:     "url={{base-url}}",
			want:      "url={{base-url}}",
			wantDiag:  true,
			wantInner: "base-url",
		},
		{
			name:      "unsupported_dynamic_placeholder",
			input:     "id={{$randomFoo}}",
			want:      "id={{$randomFoo}}",
			wantDiag:  true,
			wantInner: "$randomFoo",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, diags := NormalizeDetailed(tt.input)
			if got != tt.want {
				t.Fatalf("NormalizeDetailed() value = %q, want %q", got, tt.want)
			}

			if (len(diags) > 0) != tt.wantDiag {
				t.Fatalf("NormalizeDetailed() diagnostics = %+v, wantDiag %v", diags, tt.wantDiag)
			}

			if tt.wantDiag && diags[0].Inner != tt.wantInner {
				t.Fatalf("NormalizeDetailed() first inner = %q, want %q", diags[0].Inner, tt.wantInner)
			}
		})
	}
}
