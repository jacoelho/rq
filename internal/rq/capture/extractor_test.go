package capture

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"errors"
	"math/big"
	"net"
	"net/http"
	"testing"
	"time"
)

// Test data
const testJSON = `{
	"user": {
		"name": "John Doe",
		"age": 30,
		"email": "john@example.com"
	},
	"items": ["apple", "banana", "orange"],
	"active": true
}`

const testHTML = `<!DOCTYPE html>
<html>
<head>
	<title>Test Page</title>
</head>
<body>
	<h1>Welcome</h1>
	<p>This is a test page.</p>
</body>
</html>`

const testFormData = "name=John+Doe&age=30&email=john%40example.com&tags=go&tags=http"

func TestParseJSONBody(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantError bool
	}{
		{
			name: "valid JSON",
			body: []byte(testJSON),
		},
		{
			name:      "empty body",
			body:      []byte{},
			wantError: true,
		},
		{
			name:      "invalid JSON",
			body:      []byte("{invalid"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := ParseJSONBody(tt.body)
			if tt.wantError {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseJSONBody() error = %v", err)
			}
			if data == nil {
				t.Fatal("ParseJSONBody() returned nil data")
			}
		})
	}
}

func TestExtractJSONPath(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		path       string
		expected   any
		wantError  bool
		isNotFound bool
	}{
		{
			name:     "extract string",
			body:     []byte(testJSON),
			path:     "$.user.name",
			expected: "John Doe",
		},
		{
			name:     "extract number",
			body:     []byte(testJSON),
			path:     "$.user.age",
			expected: float64(30), // JSON numbers are float64
		},
		{
			name:     "extract boolean",
			body:     []byte(testJSON),
			path:     "$.active",
			expected: true,
		},
		{
			name:     "extract array element",
			body:     []byte(testJSON),
			path:     "$.items[0]",
			expected: "apple",
		},
		{
			name:       "non-existent path",
			body:       []byte(testJSON),
			path:       "$.nonexistent",
			isNotFound: true,
		},
		{
			name:      "empty path",
			body:      []byte(testJSON),
			path:      "",
			wantError: true,
		},
		{
			name:      "empty body",
			body:      []byte{},
			path:      "$.user.name",
			wantError: true,
		},
		{
			name:      "invalid JSON",
			body:      []byte("{invalid json}"),
			path:      "$.user.name",
			wantError: true,
		},
		{
			name:      "invalid JSONPath",
			body:      []byte(testJSON),
			path:      "$[invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractJSONPath(tt.body, tt.path)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if tt.isNotFound {
				if !IsNotFound(err) {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractJSONPath() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ExtractJSONPath() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractJSONPathFromData(t *testing.T) {
	data, err := ParseJSONBody([]byte(testJSON))
	if err != nil {
		t.Fatalf("ParseJSONBody() error = %v", err)
	}

	tests := []struct {
		name       string
		path       string
		expected   any
		wantError  bool
		isNotFound bool
	}{
		{
			name:     "extract string",
			path:     "$.user.name",
			expected: "John Doe",
		},
		{
			name:     "extract boolean",
			path:     "$.active",
			expected: true,
		},
		{
			name:       "non-existent path",
			path:       "$.nonexistent",
			isNotFound: true,
		},
		{
			name:      "empty path",
			path:      "",
			wantError: true,
		},
		{
			name:      "invalid JSONPath",
			path:      "$[invalid",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractJSONPathFromData(data, tt.path)

			if tt.wantError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if tt.isNotFound {
				if !IsNotFound(err) {
					t.Errorf("expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractJSONPathFromData() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ExtractJSONPathFromData() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractJSONPathString(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		path      string
		expected  string
		wantError bool
	}{
		{
			name:     "extract string",
			body:     []byte(testJSON),
			path:     "$.user.name",
			expected: "John Doe",
		},
		{
			name:     "extract number as string",
			body:     []byte(testJSON),
			path:     "$.user.age",
			expected: "30",
		},
		{
			name:     "extract boolean as string",
			body:     []byte(testJSON),
			path:     "$.active",
			expected: "true",
		},
		{
			name:      "non-existent path",
			body:      []byte(testJSON),
			path:      "$.nonexistent",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractJSONPathString(tt.body, tt.path)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractJSONPathString() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ExtractJSONPathString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractRegex(t *testing.T) {
	tests := []struct {
		name       string
		body       []byte
		pattern    string
		group      int
		expected   any
		wantError  bool
		isNotFound bool
	}{
		{
			name:     "extract title group",
			body:     []byte(testHTML),
			pattern:  `<title>(.*?)</title>`,
			group:    1,
			expected: "Test Page",
		},
		{
			name:     "extract full title tag",
			body:     []byte(testHTML),
			pattern:  `<title>.*?</title>`,
			group:    0,
			expected: "<title>Test Page</title>",
		},
		{
			name:       "no match",
			body:       []byte(testHTML),
			pattern:    `<footer>(.*?)</footer>`,
			group:      1,
			isNotFound: true,
		},
		{
			name:      "invalid group",
			body:      []byte(testHTML),
			pattern:   `<title>(.*?)</title>`,
			group:     2,
			wantError: true,
		},
		{
			name:      "empty pattern",
			body:      []byte(testHTML),
			pattern:   "",
			group:     0,
			wantError: true,
		},
		{
			name:      "negative group",
			body:      []byte(testHTML),
			pattern:   `<title>(.*?)</title>`,
			group:     -1,
			wantError: true,
		},
		{
			name:      "invalid regex",
			body:      []byte(testHTML),
			pattern:   `[invalid`,
			group:     0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractRegex(tt.body, tt.pattern, tt.group)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if tt.isNotFound {
				if !IsNotFound(err) {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractRegex() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ExtractRegex() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestExtractAllRegex(t *testing.T) {
	testText := `
		<div>First</div>
		<div>Second</div>
		<div>Third</div>
	`

	tests := []struct {
		name       string
		body       []byte
		pattern    string
		group      int
		expected   []string
		wantError  bool
		isNotFound bool
	}{
		{
			name:     "extract all div contents",
			body:     []byte(testText),
			pattern:  `<div>(.*?)</div>`,
			group:    1,
			expected: []string{"First", "Second", "Third"},
		},
		{
			name:     "extract all div tags",
			body:     []byte(testText),
			pattern:  `<div>.*?</div>`,
			group:    0,
			expected: []string{"<div>First</div>", "<div>Second</div>", "<div>Third</div>"},
		},
		{
			name:       "no matches",
			body:       []byte(testText),
			pattern:    `<span>(.*?)</span>`,
			group:      1,
			isNotFound: true,
		},
		{
			name:      "invalid group",
			body:      []byte(testText),
			pattern:   `<div>(.*?)</div>`,
			group:     2,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := ExtractAllRegex(tt.body, tt.pattern, tt.group)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if tt.isNotFound {
				if !IsNotFound(err) {
					t.Errorf("Expected ErrNotFound, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractAllRegex() error = %v", err)
			}

			if len(results) != len(tt.expected) {
				t.Errorf("ExtractAllRegex() length = %d, want %d", len(results), len(tt.expected))
			}

			for i, result := range results {
				if i < len(tt.expected) && result != tt.expected[i] {
					t.Errorf("ExtractAllRegex()[%d] = %v, want %v", i, result, tt.expected[i])
				}
			}
		})
	}
}

func TestExtractRegexString(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		pattern   string
		group     int
		expected  string
		wantError bool
	}{
		{
			name:     "extract title",
			body:     []byte(testHTML),
			pattern:  `<title>(.*?)</title>`,
			group:    1,
			expected: "Test Page",
		},
		{
			name:      "no match",
			body:      []byte(testHTML),
			pattern:   `<footer>(.*?)</footer>`,
			group:     1,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractRegexString(tt.body, tt.pattern, tt.group)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractRegexString() error = %v", err)
			}

			if result != tt.expected {
				t.Errorf("ExtractRegexString() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestParseFormData(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantError bool
		checkFunc func(t *testing.T, values any)
	}{
		{
			name: "valid form data",
			body: []byte(testFormData),
			checkFunc: func(t *testing.T, values any) {
				vals := values.(map[string][]string)
				if vals["name"][0] != "John Doe" {
					t.Errorf("name = %v, want John Doe", vals["name"][0])
				}
				if vals["age"][0] != "30" {
					t.Errorf("age = %v, want 30", vals["age"][0])
				}
				if vals["email"][0] != "john@example.com" {
					t.Errorf("email = %v, want john@example.com", vals["email"][0])
				}
				if len(vals["tags"]) != 2 || vals["tags"][0] != "go" || vals["tags"][1] != "http" {
					t.Errorf("tags = %v, want [go http]", vals["tags"])
				}
			},
		},
		{
			name: "empty body",
			body: []byte{},
			checkFunc: func(t *testing.T, values any) {
				vals := values.(map[string][]string)
				if len(vals) != 0 {
					t.Errorf("Expected empty values, got %v", vals)
				}
			},
		},
		{
			name:      "invalid form data",
			body:      []byte("invalid%ZZ%form%data"),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, err := ParseFormData(tt.body)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ParseFormData() error = %v", err)
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, map[string][]string(values))
			}
		})
	}
}

func TestExtractAllCertificateFields(t *testing.T) {
	// Create a simple mock certificate
	now := time.Now()
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(12345),
		Subject: pkix.Name{
			Organization: []string{"Test Org"},
			Country:      []string{"US"},
			Locality:     []string{"Test City"},
		},
		Issuer: pkix.Name{
			Organization: []string{"Test CA"},
			Country:      []string{"US"},
			Locality:     []string{"Test City"},
		},
		NotBefore:   now,
		NotAfter:    now.Add(365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	tests := []struct {
		name      string
		resp      *http.Response
		wantError bool
		checkFunc func(t *testing.T, info *CertificateInfo)
	}{
		{
			name: "valid certificate",
			resp: &http.Response{
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{cert},
				},
			},
			checkFunc: func(t *testing.T, info *CertificateInfo) {
				if info.Subject == "" {
					t.Error("Expected non-empty subject")
				}
				if info.Issuer == "" {
					t.Error("Expected non-empty issuer")
				}
				if info.SerialNumber != "12345" {
					t.Errorf("SerialNumber = %v, want 12345", info.SerialNumber)
				}
				if info.ExpireDate.IsZero() {
					t.Error("Expected non-zero expire date")
				}
			},
		},
		{
			name:      "nil response",
			resp:      nil,
			wantError: true,
		},
		{
			name: "no TLS info",
			resp: &http.Response{
				TLS: nil,
			},
			wantError: true,
		},
		{
			name: "no certificates",
			resp: &http.Response{
				TLS: &tls.ConnectionState{
					PeerCertificates: []*x509.Certificate{},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := ExtractAllCertificateFields(tt.resp)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("ExtractAllCertificateFields() error = %v", err)
			}

			if info == nil {
				t.Fatal("Expected non-nil certificate info")
			}

			if tt.checkFunc != nil {
				tt.checkFunc(t, info)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "ErrNotFound",
			err:      ErrNotFound,
			expected: true,
		},
		{
			name:     "wrapped ErrNotFound",
			err:      errors.New("wrapped: " + ErrNotFound.Error()),
			expected: false, // errors.New doesn't wrap, use fmt.Errorf with %w
		},
		{
			name:     "other error",
			err:      ErrInvalidInput,
			expected: false,
		},
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNotFound(tt.err)
			if result != tt.expected {
				t.Errorf("IsNotFound(%v) = %v, want %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestExtractCertificateField(t *testing.T) {
	now := time.Now()
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject: pkix.Name{
			Organization: []string{"Test Subject"},
		},
		Issuer: pkix.Name{
			Organization: []string{"Test Issuer"},
		},
		NotAfter: now.Add(24 * time.Hour),
	}

	resp := &http.Response{
		TLS: &tls.ConnectionState{
			PeerCertificates: []*x509.Certificate{cert},
		},
	}

	tests := []struct {
		name      string
		field     string
		wantError bool
	}{
		{name: "subject", field: CertificateFieldSubject},
		{name: "issuer", field: CertificateFieldIssuer},
		{name: "expire_date", field: CertificateFieldExpireDate},
		{name: "serial_number", field: CertificateFieldSerialNumber},
		{name: "unsupported_field", field: "unsupported", wantError: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := ExtractCertificateField(resp, tt.field)
			if (err != nil) != tt.wantError {
				t.Fatalf("ExtractCertificateField() error = %v, wantError %v", err, tt.wantError)
			}
			if err == nil && value == nil {
				t.Fatal("ExtractCertificateField() returned nil value without error")
			}
		})
	}
}
