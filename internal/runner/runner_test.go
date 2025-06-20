package runner

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jacoelho/rq/internal/config"
	"github.com/jacoelho/rq/internal/parser"
)

// checkNumericValue is a helper function to check numeric values from JSONPath
// which can return int, float64, or json.Number depending on the JSON parser
func checkNumericValue(t *testing.T, actual any, expected int, fieldName string) {
	t.Helper()

	switch v := actual.(type) {
	case int:
		if v != expected {
			t.Errorf("%s = %v, want %d", fieldName, v, expected)
		}
	case float64:
		if v != float64(expected) {
			t.Errorf("%s = %v, want %d", fieldName, v, expected)
		}
	case json.Number:
		if intVal, err := v.Int64(); err != nil {
			t.Errorf("%s json.Number conversion failed: %v", fieldName, err)
		} else if intVal != int64(expected) {
			t.Errorf("%s = %v, want %d", fieldName, intVal, expected)
		}
	default:
		t.Errorf("%s has unexpected type %T: %v", fieldName, v, v)
	}
}

func TestExecuteCaptures(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Custom-Header", "test-value")
		w.Header().Set("Server", "test-server/1.0")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"user": {
				"id": 123,
				"name": "Alice"
			},
			"status": "success",
			"message": "Version: 1.2.3 is available"
		}`))
	}))
	defer server.Close()

	cfg := &config.Config{}
	runner, _ := New(cfg)

	tests := []struct {
		name     string
		captures *parser.Captures
		check    func(t *testing.T, captureMap map[string]any)
	}{
		{
			name:     "nil_captures",
			captures: nil,
			check: func(t *testing.T, captureMap map[string]any) {
				if len(captureMap) != 0 {
					t.Errorf("expected empty capture map, got %v", captureMap)
				}
			},
		},
		{
			name: "structured_captures_status",
			captures: &parser.Captures{
				Status: []parser.StatusCapture{
					{Name: "response_status"},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				if captureMap["response_status"] != 200 {
					t.Errorf("response_status = %v, want 200", captureMap["response_status"])
				}
			},
		},
		{
			name: "structured_captures_headers",
			captures: &parser.Captures{
				Headers: []parser.HeaderCapture{
					{Name: "content_type", HeaderName: "Content-Type"},
					{Name: "custom_header", HeaderName: "X-Custom-Header"},
					{Name: "server_header", HeaderName: "Server"},
					{Name: "missing_header", HeaderName: "X-Missing"},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				if captureMap["content_type"] != "application/json" {
					t.Errorf("content_type = %v, want application/json", captureMap["content_type"])
				}
				if captureMap["custom_header"] != "test-value" {
					t.Errorf("custom_header = %v, want test-value", captureMap["custom_header"])
				}
				if captureMap["server_header"] == "" {
					t.Error("server_header should not be empty")
				}
				if captureMap["missing_header"] != "" {
					t.Errorf("missing_header = %v, want empty string", captureMap["missing_header"])
				}
			},
		},
		{
			name: "structured_captures_jsonpath",
			captures: &parser.Captures{
				JSONPath: []parser.JSONPathCapture{
					{Name: "user_id", Path: "$.user.id"},
					{Name: "user_name", Path: "$.user.name"},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				if userID, exists := captureMap["user_id"]; !exists {
					t.Error("expected capture user_id to exist")
				} else {
					checkNumericValue(t, userID, 123, "jsonpath capture user_id")
				}
				if captureMap["user_name"] != "Alice" {
					t.Errorf("user_name = %v, want Alice", captureMap["user_name"])
				}
			},
		},
		{
			name: "structured_captures_regex",
			captures: &parser.Captures{
				Regex: []parser.RegexCapture{
					{Name: "version", Pattern: `Version: (\d+\.\d+\.\d+)`, Group: 1},
					{Name: "full_version_text", Pattern: `Version: \d+\.\d+\.\d+`, Group: 0},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				if captureMap["version"] != "1.2.3" {
					t.Errorf("version = %v, want 1.2.3", captureMap["version"])
				}
				if captureMap["full_version_text"] != "Version: 1.2.3" {
					t.Errorf("full_version_text = %v, want 'Version: 1.2.3'", captureMap["full_version_text"])
				}
			},
		},
		{
			name: "structured_captures_body",
			captures: &parser.Captures{
				Body: []parser.BodyCapture{
					{Name: "raw_body"},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				expected := `{
			"user": {
				"id": 123,
				"name": "Alice"
			},
			"status": "success",
			"message": "Version: 1.2.3 is available"
		}`
				if captureMap["raw_body"] != expected {
					t.Errorf("raw_body = %v, want %v", captureMap["raw_body"], expected)
				}
			},
		},
		{
			name: "structured_captures_mixed",
			captures: &parser.Captures{
				Status: []parser.StatusCapture{
					{Name: "status_code"},
				},
				JSONPath: []parser.JSONPathCapture{
					{Name: "user_status", Path: "$.status"},
				},
			},
			check: func(t *testing.T, captureMap map[string]any) {
				if captureMap["status_code"] != 200 {
					t.Errorf("status_code = %v, want 200", captureMap["status_code"])
				}
				if captureMap["user_status"] != "success" {
					t.Errorf("user_status = %v, want success", captureMap["user_status"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(server.URL)
			if err != nil {
				t.Fatalf("failed to make request: %v", err)
			}
			defer resp.Body.Close()

			body := bytes.NewBuffer(nil)
			body.ReadFrom(resp.Body)
			bodyBytes := body.Bytes()

			captureMap := make(map[string]any)
			err = runner.executeCaptures(tt.captures, resp, bodyBytes, captureMap)
			if err != nil {
				t.Fatalf("executeCaptures failed: %v", err)
			}

			tt.check(t, captureMap)
		})
	}
}

func TestExecuteRegexCapture(t *testing.T) {
	t.Parallel()

	runner := &Runner{}

	tests := []struct {
		name        string
		capture     parser.RegexCapture
		body        string
		expectValue any
		expectError bool
	}{
		{
			name: "valid_pattern_group_1",
			capture: parser.RegexCapture{
				Name:    "version",
				Pattern: `version: (\d+\.\d+\.\d+)`,
				Group:   1,
			},
			body:        "version: 1.2.3",
			expectValue: "1.2.3",
		},
		{
			name: "valid_pattern_group_0",
			capture: parser.RegexCapture{
				Name:    "full_match",
				Pattern: `error: \w+`,
				Group:   0,
			},
			body:        "error: failed",
			expectValue: "error: failed",
		},
		{
			name: "no_match",
			capture: parser.RegexCapture{
				Name:    "missing",
				Pattern: `notfound: (.+)`,
				Group:   1,
			},
			body:        "version: 1.2.3",
			expectValue: nil,
		},
		{
			name: "invalid_pattern",
			capture: parser.RegexCapture{
				Name:    "invalid",
				Pattern: `[invalid`,
				Group:   1,
			},
			body:        "test",
			expectError: true,
		},
		{
			name: "invalid_group_index",
			capture: parser.RegexCapture{
				Name:    "invalid_group",
				Pattern: `version: (\d+)`,
				Group:   5, // Only groups 0 and 1 exist
			},
			body:        "version: 123",
			expectError: true,
		},
		{
			name: "multiple_groups",
			capture: parser.RegexCapture{
				Name:    "major_version",
				Pattern: `version: (\d+)\.(\d+)\.(\d+)`,
				Group:   1,
			},
			body:        "version: 1.2.3",
			expectValue: "1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			captureMap := make(map[string]any)
			err := runner.executeRegexCapture(tt.capture, []byte(tt.body), captureMap)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if captureMap[tt.capture.Name] != tt.expectValue {
				t.Errorf("capture %s = %v, want %v", tt.capture.Name, captureMap[tt.capture.Name], tt.expectValue)
			}
		})
	}
}

func TestExecuteCapturesErrorCases(t *testing.T) {
	t.Parallel()

	runner := &Runner{}

	tests := []struct {
		name     string
		captures *parser.Captures
		wantErr  bool
	}{
		{
			name: "invalid_jsonpath",
			captures: &parser.Captures{
				JSONPath: []parser.JSONPathCapture{
					{Name: "invalid", Path: "$.invalid["},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: 200,
				Header:     make(http.Header),
			}
			body := []byte(`{"test": "value"}`)
			captureMap := make(map[string]any)

			err := runner.executeCaptures(tt.captures, resp, body, captureMap)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			} else if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestExtractCertificateField(t *testing.T) {
	runner := NewDefault()

	tests := []struct {
		name          string
		field         string
		setupResponse func() *http.Response
		expectError   bool
		expectedValue any
	}{
		{
			name:  "no TLS connection",
			field: "subject",
			setupResponse: func() *http.Response {
				return &http.Response{
					TLS: nil,
				}
			},
			expectError: true,
		},
		{
			name:  "empty peer certificates",
			field: "subject",
			setupResponse: func() *http.Response {
				return &http.Response{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{},
					},
				}
			},
			expectError: true,
		},
		{
			name:  "unsupported field",
			field: "invalid_field",
			setupResponse: func() *http.Response {
				cert := createTestCertificate(t)
				return &http.Response{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{cert},
					},
				}
			},
			expectError: true,
		},
		{
			name:  "subject field",
			field: "subject",
			setupResponse: func() *http.Response {
				cert := createTestCertificate(t)
				return &http.Response{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{cert},
					},
				}
			},
			expectError:   false,
			expectedValue: "CN=example.com",
		},
		{
			name:  "issuer field",
			field: "issuer",
			setupResponse: func() *http.Response {
				cert := createTestCertificate(t)
				return &http.Response{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{cert},
					},
				}
			},
			expectError:   false,
			expectedValue: "CN=Test CA",
		},
		{
			name:  "serial_number field",
			field: "serial_number",
			setupResponse: func() *http.Response {
				cert := createTestCertificate(t)
				return &http.Response{
					TLS: &tls.ConnectionState{
						PeerCertificates: []*x509.Certificate{cert},
					},
				}
			},
			expectError:   false,
			expectedValue: "12345",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResponse()
			value, err := runner.extractCertificateField(tt.field, resp)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
				if tt.expectedValue != nil && value != tt.expectedValue {
					t.Errorf("Expected value %v, got %v", tt.expectedValue, value)
				}
			}
		})
	}
}

func createTestCertificate(t *testing.T) *x509.Certificate {
	t.Helper()

	issuerTemplate := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "Test CA",
		},
		NotBefore:             time.Now().Add(-48 * time.Hour),
		NotAfter:              time.Now().Add(2 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	subjectTemplate := x509.Certificate{
		SerialNumber: big.NewInt(12345),
		Subject: pkix.Name{
			CommonName: "example.com",
		},
		NotBefore:             time.Now().Add(-24 * time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	issuerPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate issuer private key: %v", err)
	}

	subjectPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("Failed to generate subject private key: %v", err)
	}

	certDER, err := x509.CreateCertificate(rand.Reader, &subjectTemplate, &issuerTemplate, &subjectPriv.PublicKey, issuerPriv)
	if err != nil {
		t.Fatalf("Failed to create certificate: %v", err)
	}

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("Failed to parse certificate: %v", err)
	}

	return cert
}
