// Package runner provides HTTP request execution functionality for the rq tool.
// It handles request execution, retries, captures, and assertions.
package runner

import (
	"bytes"
	"context"
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

	"github.com/jacoelho/rq/internal/parser"
)

// checkNumericValue is a helper function to check numeric values from JSONPath
// which can return int, float64, or json.Number depending on the JSON parser.
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

	runner := NewDefault()

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
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
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

func TestExecuteStepWithRetries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		retries         int
		serverResponses []struct {
			status int
			body   string
		}
		expectedAttempts int
		expectedError    bool
	}{
		{
			name:    "no_retries_success",
			retries: 0,
			serverResponses: []struct {
				status int
				body   string
			}{
				{status: 200, body: `{"status": "success"}`},
			},
			expectedAttempts: 1,
			expectedError:    false,
		},
		{
			name:    "no_retries_failure",
			retries: 0,
			serverResponses: []struct {
				status int
				body   string
			}{
				{status: 500, body: `{"status": "error"}`},
			},
			expectedAttempts: 1,
			expectedError:    true,
		},
		{
			name:    "retry_until_success",
			retries: 3,
			serverResponses: []struct {
				status int
				body   string
			}{
				{status: 500, body: `{"status": "error"}`},
				{status: 500, body: `{"status": "error"}`},
				{status: 200, body: `{"status": "success"}`},
			},
			expectedAttempts: 3,
			expectedError:    false,
		},
		{
			name:    "retry_all_attempts_fail",
			retries: 2,
			serverResponses: []struct {
				status int
				body   string
			}{
				{status: 500, body: `{"status": "error"}`},
				{status: 500, body: `{"status": "error"}`},
				{status: 500, body: `{"status": "error"}`},
			},
			expectedAttempts: 3,
			expectedError:    true,
		},
		{
			name:    "retry_first_attempt_success",
			retries: 3,
			serverResponses: []struct {
				status int
				body   string
			}{
				{status: 200, body: `{"status": "success"}`},
			},
			expectedAttempts: 1,
			expectedError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attemptCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if attemptCount < len(tt.serverResponses) {
					response := tt.serverResponses[attemptCount]
					w.WriteHeader(response.status)
					w.Write([]byte(response.body))
				} else {
					// Default to last response if we run out
					lastResponse := tt.serverResponses[len(tt.serverResponses)-1]
					w.WriteHeader(lastResponse.status)
					w.Write([]byte(lastResponse.body))
				}
				attemptCount++
			}))
			defer server.Close()

			runner := NewDefault()

			step := parser.Step{
				Method: "GET",
				URL:    server.URL,
				Options: parser.Options{
					Retries: tt.retries,
				},
				Asserts: parser.Asserts{
					Status: []parser.StatusAssert{
						{
							Predicate: parser.Predicate{
								Operation: "equals",
								Value:     200,
							},
						},
					},
				},
			}

			captures := make(map[string]any)
			requestMade, err := runner.executeStep(context.Background(), step, captures)

			if !requestMade {
				t.Error("Expected request to be made")
			}

			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}

			if attemptCount != tt.expectedAttempts {
				t.Errorf("Expected %d attempts, got %d", tt.expectedAttempts, attemptCount)
			}
		})
	}
}

func TestExecuteStepWithRetriesCaptureFail(t *testing.T) {
	t.Parallel()

	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(200)
		if attemptCount <= 2 {
			w.Write([]byte(`{"name": "invalid"}`)) // Will fail JSONPath capture assertion
		} else {
			w.Write([]byte(`{"name": "Alice"}`)) // Will succeed on 3rd attempt
		}
	}))
	defer server.Close()

	runner := NewDefault()

	step := parser.Step{
		Method: "GET",
		URL:    server.URL,
		Options: parser.Options{
			Retries: 3,
		},
		Asserts: parser.Asserts{
			JSONPath: []parser.JSONPathAssert{
				{
					Path: "$.name",
					Predicate: parser.Predicate{
						Operation: "equals",
						Value:     "Alice",
					},
				},
			},
		},
	}

	captures := make(map[string]any)
	requestMade, err := runner.executeStep(context.Background(), step, captures)

	if !requestMade {
		t.Error("Expected request to be made")
	}

	if err != nil {
		t.Errorf("Expected success after retries but got error: %v", err)
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts, got %d", attemptCount)
	}
}

func TestExecuteStepWithRetriesNoRetriesNeeded(t *testing.T) {
	t.Parallel()

	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(200)
		w.Write([]byte(`{"status": "success"}`))
	}))
	defer server.Close()

	runner := NewDefault()

	step := parser.Step{
		Method: "GET",
		URL:    server.URL,
		Options: parser.Options{
			Retries: 5, // Has retries configured but shouldn't need them
		},
		Asserts: parser.Asserts{
			Status: []parser.StatusAssert{
				{
					Predicate: parser.Predicate{
						Operation: "equals",
						Value:     200,
					},
				},
			},
		},
	}

	captures := make(map[string]any)
	requestMade, err := runner.executeStep(context.Background(), step, captures)

	if !requestMade {
		t.Error("Expected request to be made")
	}

	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
	}

	if attemptCount != 1 {
		t.Errorf("Expected 1 attempt (no retries needed), got %d", attemptCount)
	}
}

func TestExecuteStepRetriesWithTemplateError(t *testing.T) {
	t.Parallel()

	runner := NewDefault()

	step := parser.Step{
		Method: "GET",
		URL:    "{{.invalid_template",
		Options: parser.Options{
			Retries: 3,
		},
	}

	captures := make(map[string]any)
	requestMade, err := runner.executeStep(context.Background(), step, captures)

	if requestMade {
		t.Error("Expected no request to be made due to template error")
	}

	if err == nil {
		t.Error("Expected error due to invalid template")
	}

	if !bytes.Contains([]byte(err.Error()), []byte("failed to process URL template")) {
		t.Errorf("Expected template error, got: %v", err)
	}
}

func TestValidateStep(t *testing.T) {
	t.Parallel()

	runner := NewDefault()

	tests := []struct {
		name        string
		step        parser.Step
		expectError bool
		errorText   string
	}{
		{
			name: "valid_step",
			step: parser.Step{
				Method: "GET",
				URL:    "https://example.com",
				Options: parser.Options{
					Retries: 3,
				},
			},
			expectError: false,
		},
		{
			name: "empty_method",
			step: parser.Step{
				Method: "",
				URL:    "https://example.com",
			},
			expectError: true,
			errorText:   "step method cannot be empty",
		},
		{
			name: "invalid_method",
			step: parser.Step{
				Method: "INVALID",
				URL:    "https://example.com",
			},
			expectError: true,
			errorText:   "unsupported HTTP method: INVALID",
		},
		{
			name: "empty_url",
			step: parser.Step{
				Method: "GET",
				URL:    "",
			},
			expectError: true,
			errorText:   "step URL cannot be empty",
		},
		{
			name: "negative_retries",
			step: parser.Step{
				Method: "GET",
				URL:    "https://example.com",
				Options: parser.Options{
					Retries: -1,
				},
			},
			expectError: true,
			errorText:   "retries must be >= 0, got: -1",
		},
		{
			name: "zero_retries",
			step: parser.Step{
				Method: "GET",
				URL:    "https://example.com",
				Options: parser.Options{
					Retries: 0,
				},
			},
			expectError: false,
		},
		{
			name: "valid_post_method",
			step: parser.Step{
				Method: "POST",
				URL:    "https://example.com/api",
				Body:   `{"key": "value"}`,
			},
			expectError: false,
		},
		{
			name: "valid_put_method",
			step: parser.Step{
				Method: "PUT",
				URL:    "https://example.com/api/1",
			},
			expectError: false,
		},
		{
			name: "valid_patch_method",
			step: parser.Step{
				Method: "PATCH",
				URL:    "https://example.com/api/1",
			},
			expectError: false,
		},
		{
			name: "valid_delete_method",
			step: parser.Step{
				Method: "DELETE",
				URL:    "https://example.com/api/1",
			},
			expectError: false,
		},
		{
			name: "valid_head_method",
			step: parser.Step{
				Method: "HEAD",
				URL:    "https://example.com",
			},
			expectError: false,
		},
		{
			name: "valid_options_method",
			step: parser.Step{
				Method: "OPTIONS",
				URL:    "https://example.com",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := runner.validateStep(tt.step)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
			if tt.expectError && err != nil && !bytes.Contains([]byte(err.Error()), []byte(tt.errorText)) {
				t.Errorf("Expected error to contain %q, got: %v", tt.errorText, err)
			}
		})
	}
}
