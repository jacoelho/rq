package sanitizer

import (
	"strings"
	"testing"
)

const testSalt = "testsalt-2025-07-05"

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		secrets map[string]any
		want    string
	}{
		{
			name:    "no secrets provided",
			data:    "hello world",
			secrets: map[string]any{},
			want:    "hello world",
		},
		{
			name:    "nil secrets map",
			data:    "hello world",
			secrets: nil,
			want:    "hello world",
		},
		{
			name:    "single secret replacement",
			data:    "hello secret123 world",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "hello [S256:b693407b4d117bbe] world",
		},
		{
			name:    "multiple different secrets",
			data:    "hello secret123 world token456",
			secrets: map[string]any{"api_key": "secret123", "access_token": "token456"},
			want:    "hello [S256:b693407b4d117bbe] world [S256:038b06ee733b06f2]",
		},
		{
			name:    "empty secret value ignored",
			data:    "hello world",
			secrets: map[string]any{"api_key": ""},
			want:    "hello world",
		},
		{
			name:    "non-string secret ignored",
			data:    "hello 123 world",
			secrets: map[string]any{"number": 123},
			want:    "hello 123 world",
		},
		{
			name:    "multiple occurrences of same secret",
			data:    "secret123 and secret123 again",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "[S256:b693407b4d117bbe] and [S256:b693407b4d117bbe] again",
		},
		{
			name:    "secret with special characters",
			data:    "password: my@secret#123!",
			secrets: map[string]any{"password": "my@secret#123!"},
			want:    "password: [S256:e117bf423bb0b569]",
		},
		{
			name:    "secret with unicode characters",
			data:    "token: 🔑secret🔐",
			secrets: map[string]any{"token": "🔑secret🔐"},
			want:    "token: [S256:ce2ac9c97e2851c6]",
		},
		{
			name:    "secret with newlines",
			data:    "key:\nmulti\nline\nsecret",
			secrets: map[string]any{"key": "multi\nline\nsecret"},
			want:    "key:\n[S256:1390f75743035739]",
		},
		{
			name:    "partial secret match is replaced",
			data:    "hello secret123extra world",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "hello [S256:b693407b4d117bbe]extra world",
		},
		{
			name:    "case sensitive replacement",
			data:    "hello Secret123 and secret123",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "hello Secret123 and [S256:b693407b4d117bbe]",
		},
		{
			name:    "empty data",
			data:    "",
			secrets: map[string]any{"api_key": "secret"},
			want:    "",
		},
		{
			name:    "large data with multiple secrets",
			data:    strings.Repeat("data ", 1000) + "secret123 " + strings.Repeat("more data ", 1000) + "token456",
			secrets: map[string]any{"api_key": "secret123", "access_token": "token456"},
			want:    strings.Repeat("data ", 1000) + "[S256:b693407b4d117bbe] " + strings.Repeat("more data ", 1000) + "[S256:038b06ee733b06f2]",
		},
		{
			name:    "mixed secret types",
			data:    "hello secret123 world 456",
			secrets: map[string]any{"api_key": "secret123", "number": 456, "float": 3.14},
			want:    "hello [S256:b693407b4d117bbe] world 456",
		},
		{
			name:    "secret at start of data",
			data:    "secret123 hello world",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "[S256:b693407b4d117bbe] hello world",
		},
		{
			name:    "secret at end of data",
			data:    "hello world secret123",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "hello world [S256:b693407b4d117bbe]",
		},
		{
			name:    "secret is entire data",
			data:    "secret123",
			secrets: map[string]any{"api_key": "secret123"},
			want:    "[S256:b693407b4d117bbe]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactSecrets([]byte(tt.data), tt.secrets, testSalt)
			if string(got) != tt.want {
				t.Errorf("redactSecrets() = %s, want %s", string(got), tt.want)
			}
		})
	}
}
