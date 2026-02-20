package sanitizer

import (
	"fmt"
	"strings"
	"testing"
)

const testSalt = "testsalt-2025-07-05"

func TestRedactSecrets(t *testing.T) {
	tests := []struct {
		name         string
		data         string
		redactValues []any
		want         string
	}{
		{
			name:         "no secrets provided",
			data:         "hello world",
			redactValues: []any{},
			want:         "hello world",
		},
		{
			name:         "nil secrets slice",
			data:         "hello world",
			redactValues: nil,
			want:         "hello world",
		},
		{
			name:         "single secret replacement",
			data:         "hello secret123 world",
			redactValues: []any{"secret123"},
			want:         "hello [S256:b693407b4d117bbe] world",
		},
		{
			name:         "multiple different secrets",
			data:         "hello secret123 world token456",
			redactValues: []any{"secret123", "token456"},
			want:         "hello [S256:b693407b4d117bbe] world [S256:038b06ee733b06f2]",
		},
		{
			name:         "empty secret value ignored",
			data:         "hello world",
			redactValues: []any{""},
			want:         "hello world",
		},
		{
			name:         "non-string secret ignored",
			data:         "hello 123 world",
			redactValues: []any{123},
			want:         "hello 123 world",
		},
		{
			name:         "multiple occurrences of same secret",
			data:         "secret123 and secret123 again",
			redactValues: []any{"secret123"},
			want:         "[S256:b693407b4d117bbe] and [S256:b693407b4d117bbe] again",
		},
		{
			name:         "secret with special characters",
			data:         "password: my@secret#123!",
			redactValues: []any{"my@secret#123!"},
			want:         "password: [S256:e117bf423bb0b569]",
		},
		{
			name:         "secret with unicode characters",
			data:         "token: 🔑secret🔐",
			redactValues: []any{"🔑secret🔐"},
			want:         "token: [S256:ce2ac9c97e2851c6]",
		},
		{
			name:         "secret with newlines",
			data:         "key:\nmulti\nline\nsecret",
			redactValues: []any{"multi\nline\nsecret"},
			want:         "key:\n[S256:1390f75743035739]",
		},
		{
			name:         "partial secret match is replaced",
			data:         "hello secret123extra world",
			redactValues: []any{"secret123"},
			want:         "hello [S256:b693407b4d117bbe]extra world",
		},
		{
			name:         "case sensitive replacement",
			data:         "hello Secret123 and secret123",
			redactValues: []any{"secret123"},
			want:         "hello Secret123 and [S256:b693407b4d117bbe]",
		},
		{
			name:         "empty data",
			data:         "",
			redactValues: []any{"secret"},
			want:         "",
		},
		{
			name:         "large data with multiple secrets",
			data:         strings.Repeat("data ", 1000) + "secret123 " + strings.Repeat("more data ", 1000) + "token456",
			redactValues: []any{"secret123", "token456"},
			want:         strings.Repeat("data ", 1000) + "[S256:b693407b4d117bbe] " + strings.Repeat("more data ", 1000) + "[S256:038b06ee733b06f2]",
		},
		{
			name:         "mixed secret types",
			data:         "hello secret123 world 456",
			redactValues: []any{"secret123", 456, 3.14},
			want:         "hello [S256:b693407b4d117bbe] world 456",
		},
		{
			name:         "secret at start of data",
			data:         "secret123 hello world",
			redactValues: []any{"secret123"},
			want:         "[S256:b693407b4d117bbe] hello world",
		},
		{
			name:         "secret at end of data",
			data:         "hello world secret123",
			redactValues: []any{"secret123"},
			want:         "hello world [S256:b693407b4d117bbe]",
		},
		{
			name:         "secret is entire data",
			data:         "secret123",
			redactValues: []any{"secret123"},
			want:         "[S256:b693407b4d117bbe]",
		},
		{
			name:         "overlapping secrets longest match first",
			data:         "Bearer abcd",
			redactValues: []any{"abc", "abcd"},
			want:         fmt.Sprintf("Bearer %s", string(hashToken("abcd", testSalt))),
		},
		{
			name:         "overlapping secrets deterministic regardless of order",
			data:         "Bearer abcd",
			redactValues: []any{"abcd", "abc"},
			want:         fmt.Sprintf("Bearer %s", string(hashToken("abcd", testSalt))),
		},
		{
			name:         "single char secret does not mutate hash marker for longer secret",
			data:         "Bearer abcd",
			redactValues: []any{"S", "abcd"},
			want:         fmt.Sprintf("Bearer %s", string(hashToken("abcd", testSalt))),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactOutput([]byte(tt.data), tt.redactValues, testSalt)
			if string(got) != tt.want {
				t.Errorf("redactSecrets() = %s, want %s", string(got), tt.want)
			}
		})
	}
}
