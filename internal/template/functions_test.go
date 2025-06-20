package template

import (
	"encoding/base64"
	"regexp"
	"strings"
	"testing"
)

func TestRandomFunctions(t *testing.T) {
	t.Parallel()

	t.Run("randomInt", func(t *testing.T) {
		min, max := 10, 20

		for range 100 {
			result := randomInt(min, max)
			if result < min || result > max {
				t.Errorf("randomInt(%d, %d) = %d, should be between %d and %d", min, max, result, min, max)
			}
		}
	})

	t.Run("randomInt_reversed_params", func(t *testing.T) {
		result := randomInt(20, 10)
		if result < 10 || result > 20 {
			t.Errorf("randomInt(20, 10) = %d, should be between 10 and 20", result)
		}
	})

	t.Run("randomString", func(t *testing.T) {
		length := 10
		result := randomString(length)

		if len(result) != length {
			t.Errorf("randomString(%d) returned string of length %d, want %d", length, len(result), length)
		}

		if !regexp.MustCompile(`^[a-zA-Z0-9]+$`).MatchString(result) {
			t.Errorf("randomString(%d) returned non-alphanumeric string: %s", length, result)
		}
	})

	t.Run("randomString_zero_length", func(t *testing.T) {
		result := randomString(0)
		if result != "" {
			t.Errorf("randomString(0) = %q, want empty string", result)
		}
	})

	t.Run("randomString_negative_length", func(t *testing.T) {
		result := randomString(-5)
		if result != "" {
			t.Errorf("randomString(-5) = %q, want empty string", result)
		}
	})

	t.Run("randomString_uniqueness", func(t *testing.T) {
		length := 20
		strings := make(map[string]bool)

		for range 10 {
			result := randomString(length)
			if strings[result] {
				t.Errorf("randomString(%d) generated duplicate string: %s", length, result)
			}
			strings[result] = true
		}
	})
}

func TestFuncMap(t *testing.T) {
	t.Parallel()

	funcMap := FuncMap()

	expectedFunctions := []string{
		"uuidv4", "uuid", "now", "timestamp", "iso8601", "rfc3339",
		"upper", "lower", "title", "trim", "randomInt", "randomString", "base64",
	}

	for _, funcName := range expectedFunctions {
		if _, exists := funcMap[funcName]; !exists {
			t.Errorf("FuncMap() missing expected function: %s", funcName)
		}
	}
}

func TestNewTemplate(t *testing.T) {
	t.Parallel()

	tmpl := NewTemplate("test")
	if tmpl == nil {
		t.Error("NewTemplate() returned nil")
	}

	_, err := tmpl.Parse("{{ uuidv4 }}")
	if err != nil {
		t.Errorf("NewTemplate() template doesn't have uuidv4 function: %v", err)
	}
}

func TestMustParse(t *testing.T) {
	t.Parallel()

	tmpl := MustParse("test", "Hello {{ .name }}")
	if tmpl == nil {
		t.Error("MustParse() returned nil for valid template")
	}

	tmpl = MustParse("test", "ID: {{ uuidv4 }}")
	if tmpl == nil {
		t.Error("MustParse() returned nil for template with custom function")
	}
}

func TestTemplateIntegration(t *testing.T) {
	t.Parallel()

	tmpl := MustParse("integration", `{
  "id": "{{ uuidv4 }}",
  "timestamp": "{{ now }}",
  "name": "{{ upper .name }}",
  "token": "{{ base64 .secret }}",
  "random": "{{ randomString 8 }}"
}`)

	data := map[string]string{
		"name":   "john doe",
		"secret": "mysecret",
	}

	var buf strings.Builder
	err := tmpl.Execute(&buf, data)
	if err != nil {
		t.Fatalf("Template execution failed: %v", err)
	}

	result := buf.String()

	if !strings.Contains(result, "JOHN DOE") {
		t.Error("Template didn't process 'upper' function correctly")
	}

	if !strings.Contains(result, "bXlzZWNyZXQ=") {
		t.Error("Template didn't process 'base64' function correctly")
	}

	uuidRegex := regexp.MustCompile(`"id": "([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})"`)
	if !uuidRegex.MatchString(result) {
		t.Error("Template didn't generate valid UUID")
	}

	timestampRegex := regexp.MustCompile(`"timestamp": "\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[Z+-]`)
	if !timestampRegex.MatchString(result) {
		t.Error("Template didn't generate valid timestamp")
	}

	randomRegex := regexp.MustCompile(`"random": "([a-zA-Z0-9]{8})"`)
	if !randomRegex.MatchString(result) {
		t.Error("Template didn't generate valid random string")
	}
}

func TestTemplateFunctionErrors(t *testing.T) {
	t.Parallel()

	tmpl := NewTemplate("error_test")

	tmpl, err := tmpl.Parse("{{ randomInt }}")
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, nil)
	if err == nil {
		t.Error("Expected error for randomInt without arguments, but got none")
	}
}

func BenchmarkUUIDGeneration(b *testing.B) {
	for b.Loop() {
		_ = generateUUIDv4()
	}
}

func BenchmarkRandomString(b *testing.B) {
	for b.Loop() {
		_ = randomString(16)
	}
}

func BenchmarkTemplateExecution(b *testing.B) {
	tmpl := MustParse("bench", "{{ uuidv4 }}-{{ randomString 8 }}-{{ now }}")

	for b.Loop() {
		var buf strings.Builder
		_ = tmpl.Execute(&buf, nil)
	}
}

func TestApply(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		template string
		data     any
		wantErr  bool
	}{
		{
			name:     "empty_template",
			template: "",
			data:     nil,
			wantErr:  false,
		},
		{
			name:     "simple_variable",
			template: "Hello {{ .name }}",
			data:     map[string]string{"name": "World"},
			wantErr:  false,
		},
		{
			name:     "uuidv4_function",
			template: "ID: {{ uuidv4 }}",
			data:     nil,
			wantErr:  false,
		},
		{
			name:     "complex_template",
			template: `{"id":"{{ uuidv4 }}","user":"{{ upper .username }}","time":"{{ now }}"}`,
			data:     map[string]string{"username": "john"},
			wantErr:  false,
		},
		{
			name:     "invalid_template",
			template: "{{ .missing )",
			data:     nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Apply(tt.template, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Apply() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Apply() unexpected error: %v", err)
				return
			}

			// Basic validation that template was processed
			if tt.template == "" && result != "" {
				t.Errorf("Apply() with empty template should return empty string, got %q", result)
			}

			if tt.template != "" && result == "" {
				t.Errorf("Apply() with non-empty template returned empty result")
			}

			// Verify specific patterns for dynamic content
			switch tt.name {
			case "simple_variable":
				if result != "Hello World" {
					t.Errorf("Apply() = %q, expected 'Hello World'", result)
				}
			case "uuidv4_function":
				if !strings.HasPrefix(result, "ID: ") {
					t.Errorf("Apply() = %q, expected to start with 'ID: '", result)
				}
				uuid := strings.TrimPrefix(result, "ID: ")
				if len(uuid) != 36 {
					t.Errorf("Apply() UUID length = %d, expected 36", len(uuid))
				}
			case "complex_template":
				if !strings.Contains(result, `"user":"JOHN"`) {
					t.Errorf("Apply() = %q, expected to contain uppercase username", result)
				}
			}
		})
	}
}

func TestApplyWithName(t *testing.T) {
	t.Parallel()

	// Test normal operation
	result, err := ApplyWithName("test", "Hello {{ .name }}", map[string]string{"name": "World"})
	if err != nil {
		t.Fatalf("ApplyWithName() unexpected error: %v", err)
	}

	if result != "Hello World" {
		t.Errorf("ApplyWithName() = %q, expected 'Hello World'", result)
	}

	// Test with template functions
	result, err = ApplyWithName("uuid-test", "{{ uuidv4 }}", nil)
	if err != nil {
		t.Fatalf("ApplyWithName() unexpected error: %v", err)
	}

	if len(result) != 36 {
		t.Errorf("ApplyWithName() UUID length = %d, expected 36", len(result))
	}

	// Test error case
	_, err = ApplyWithName("error-test", "{{ .invalid )", nil)
	if err == nil {
		t.Error("ApplyWithName() expected error for invalid template but got none")
	}
}

func BenchmarkApply(b *testing.B) {
	template := `{"id":"{{ uuidv4 }}","user":"{{ upper .name }}","time":"{{ now }}"}`
	data := map[string]string{"name": "testuser"}

	for b.Loop() {
		_, _ = Apply(template, data)
	}
}

func BenchmarkApplySimple(b *testing.B) {
	template := "Hello {{ .name }}"
	data := map[string]string{"name": "World"}

	for b.Loop() {
		_, _ = Apply(template, data)
	}
}

func TestBase64Padding(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "single_character",
			input:    "a",
			expected: "YQ==", // Should have 2 padding characters
		},
		{
			name:     "two_characters",
			input:    "ab",
			expected: "YWI=", // Should have 1 padding character
		},
		{
			name:     "three_characters",
			input:    "abc",
			expected: "YWJj", // Should have no padding
		},
		{
			name:     "four_characters",
			input:    "abcd",
			expected: "YWJjZA==", // Should have 2 padding characters
		},
		{
			name:     "empty_string",
			input:    "",
			expected: "", // Empty string should return empty
		},
		{
			name:     "special_characters",
			input:    "hello@world!",
			expected: "aGVsbG9Ad29ybGQh", // No padding needed for this length
		},
		{
			name:     "unicode_characters",
			input:    "héllo",
			expected: "aMOpbGxv", // UTF-8 encoded then base64
		},
		{
			name:     "long_string",
			input:    "this is a longer string to test base64 encoding with proper padding",
			expected: "dGhpcyBpcyBhIGxvbmdlciBzdHJpbmcgdG8gdGVzdCBiYXNlNjQgZW5jb2Rpbmcgd2l0aCBwcm9wZXIgcGFkZGluZw==",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := base64Encode(tt.input)
			if result != tt.expected {
				t.Errorf("base64Encode(%q) = %q, expected %q", tt.input, result, tt.expected)
			}

			// Verify that the result is valid base64 and can be decoded back
			if tt.input != "" {
				decoded, err := base64.StdEncoding.DecodeString(result)
				if err != nil {
					t.Errorf("base64Encode(%q) produced invalid base64: %v", tt.input, err)
				}
				if string(decoded) != tt.input {
					t.Errorf("base64Encode(%q) round-trip failed: got %q", tt.input, string(decoded))
				}
			}
		})
	}
}
