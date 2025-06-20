package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		yaml    string
		check   func(t *testing.T, steps []Step)
		wantErr bool
	}{
		{
			name: "status_equals",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - op: equals
        value: 200
`,
			check: func(t *testing.T, steps []Step) {
				assertSingleStep(t, steps, "GET", "https://api.example.com/health")
				s := steps[0]
				if len(s.Asserts.Status) != 1 || s.Asserts.Status[0].Operation != "equals" {
					t.Errorf("Status = %+v, want Operation=equals", s.Asserts.Status)
				} else {
					switch v := s.Asserts.Status[0].Value.(type) {
					case int, int64, float64, uint64:
						if fmt.Sprint(v) != "200" {
							t.Errorf("Status value = %v, want 200", v)
						}
					default:
						t.Errorf("Status value type = %T, want int", v)
					}
				}
			},
		},
		{
			name: "header_regex_and_exists",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    headers:
      - name: Content-Type
        op: regex
        value: '^application/json'
      - name: X-Cache
        op: exists
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if len(s.Asserts.Headers) != 2 {
					t.Fatalf("expected 2 header asserts, got %d", len(s.Asserts.Headers))
				}
				h := s.Asserts.Headers[0]
				if h.Name != "Content-Type" || h.Predicate.Operation != "regex" {
					t.Errorf("Header[0] = %+v, want Name=Content-Type, Operation=regex", h)
				}
				if h.Predicate.Value != "^application/json" {
					t.Errorf("Header[0] Value = %v, want ^application/json", h.Predicate.Value)
				}
				h2 := s.Asserts.Headers[1]
				if h2.Name != "X-Cache" || h2.Predicate.Operation != "exists" {
					t.Errorf("Header[1] = %+v, want Name=X-Cache, Operation=exists", h2)
				}
			},
		},
		{
			name: "jsonpath_length_and_contains",
			yaml: `
- method: GET
  url: https://api.example.com/services
  asserts:
    jsonpath:
      - path: $.services
        op: length
        value: 3
      - path: $.roles[*]
        op: contains
        value: admin
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if len(s.Asserts.JSONPath) != 2 {
					t.Fatalf("expected 2 jsonpath asserts, got %d", len(s.Asserts.JSONPath))
				}
				jp := s.Asserts.JSONPath[0]
				if jp.Path != "$.services" || jp.Predicate.Operation != "length" {
					t.Errorf("JSONPath[0] = %+v, want Path=$.services, Operation=length", jp)
				}
				jp2 := s.Asserts.JSONPath[1]
				if jp2.Path != "$.roles[*]" || jp2.Predicate.Operation != "contains" {
					t.Errorf("JSONPath[1] = %+v, want Path=$.roles[*], Operation=contains", jp2)
				}
			},
		},
		{
			name: "xpath_equals",
			yaml: `
- method: GET
  url: https://api.example.com/profile
  asserts:
    xpath:
      - path: string(//profile/name)
        op: equals
        value: "Alice"
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if len(s.Asserts.XPath) != 1 {
					t.Fatalf("expected 1 xpath assert, got %d", len(s.Asserts.XPath))
				}
				xp := s.Asserts.XPath[0]
				if xp.Path != "string(//profile/name)" || xp.Predicate.Operation != "equals" {
					t.Errorf("XPath[0] = %+v, want Path=string(//profile/name), Operation=equals", xp)
				}
				if xp.Predicate.Value != "Alice" {
					t.Errorf("XPath[0] Value = %v, want Alice", xp.Predicate.Value)
				}
			},
		},
		{
			name: "structured_captures_status",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    status:
      - name: response_status
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}
				if len(s.Captures.Status) != 1 {
					t.Fatalf("expected 1 status capture, got %d", len(s.Captures.Status))
				}
				if s.Captures.Status[0].Name != "response_status" {
					t.Errorf("Status[0].Name = %q, want response_status", s.Captures.Status[0].Name)
				}
			},
		},
		{
			name: "structured_captures_headers",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    headers:
      - name: content_type
        header_name: Content-Type
      - name: server_header
        header_name: Server
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}
				if len(s.Captures.Headers) != 2 {
					t.Fatalf("expected 2 header captures, got %d", len(s.Captures.Headers))
				}
				h1 := s.Captures.Headers[0]
				if h1.Name != "content_type" || h1.HeaderName != "Content-Type" {
					t.Errorf("Headers[0] = %+v, want Name=content_type, HeaderName=Content-Type", h1)
				}
				h2 := s.Captures.Headers[1]
				if h2.Name != "server_header" || h2.HeaderName != "Server" {
					t.Errorf("Headers[1] = %+v, want Name=server_header, HeaderName=Server", h2)
				}
			},
		},
		{
			name: "structured_captures_jsonpath",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    jsonpath:
      - name: user_id
        path: $.user.id
      - name: token
        path: $.auth.token
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}
				if len(s.Captures.JSONPath) != 2 {
					t.Fatalf("expected 2 jsonpath captures, got %d", len(s.Captures.JSONPath))
				}
				jp1 := s.Captures.JSONPath[0]
				if jp1.Name != "user_id" || jp1.Path != "$.user.id" {
					t.Errorf("JSONPath[0] = %+v, want Name=user_id, Path=$.user.id", jp1)
				}
				jp2 := s.Captures.JSONPath[1]
				if jp2.Name != "token" || jp2.Path != "$.auth.token" {
					t.Errorf("JSONPath[1] = %+v, want Name=token, Path=$.auth.token", jp2)
				}
			},
		},
		{
			name: "structured_captures_regex",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    regex:
      - name: version
        pattern: "version: (\\d+\\.\\d+\\.\\d+)"
        group: 1
      - name: full_match
        pattern: "error: (.+)"
        group: 0
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}
				if len(s.Captures.Regex) != 2 {
					t.Fatalf("expected 2 regex captures, got %d", len(s.Captures.Regex))
				}
				r1 := s.Captures.Regex[0]
				if r1.Name != "version" || r1.Pattern != "version: (\\d+\\.\\d+\\.\\d+)" || r1.Group != 1 {
					t.Errorf("Regex[0] = %+v, want Name=version, Pattern=version: (\\d+\\.\\d+\\.\\d+), Group=1", r1)
				}
				r2 := s.Captures.Regex[1]
				if r2.Name != "full_match" || r2.Pattern != "error: (.+)" || r2.Group != 0 {
					t.Errorf("Regex[1] = %+v, want Name=full_match, Pattern=error: (.+), Group=0", r2)
				}
			},
		},
		{
			name: "structured_captures_body",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    body:
      - name: raw_response
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}
				if len(s.Captures.Body) != 1 {
					t.Fatalf("expected 1 body capture, got %d", len(s.Captures.Body))
				}
				if s.Captures.Body[0].Name != "raw_response" {
					t.Errorf("Body[0].Name = %q, want raw_response", s.Captures.Body[0].Name)
				}
			},
		},
		{
			name: "structured_captures_mixed",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures:
    status:
      - name: status_code
    headers:
      - name: content_type
        header_name: Content-Type
    jsonpath:
      - name: user_id
        path: $.user.id
    regex:
      - name: version
        pattern: "v(\\d+)"
        group: 1
    body:
      - name: full_response
`,
			check: func(t *testing.T, steps []Step) {
				s := steps[0]
				if s.Captures == nil {
					t.Fatal("expected structured captures, got nil")
				}

				if len(s.Captures.Status) != 1 || s.Captures.Status[0].Name != "status_code" {
					t.Errorf("Status captures = %+v, want 1 capture named status_code", s.Captures.Status)
				}

				if len(s.Captures.Headers) != 1 || s.Captures.Headers[0].Name != "content_type" {
					t.Errorf("Header captures = %+v, want 1 capture named content_type", s.Captures.Headers)
				}

				if len(s.Captures.JSONPath) != 1 || s.Captures.JSONPath[0].Name != "user_id" {
					t.Errorf("JSONPath captures = %+v, want 1 capture named user_id", s.Captures.JSONPath)
				}

				if len(s.Captures.Regex) != 1 || s.Captures.Regex[0].Name != "version" {
					t.Errorf("Regex captures = %+v, want 1 capture named version", s.Captures.Regex)
				}

				if len(s.Captures.Body) != 1 || s.Captures.Body[0].Name != "full_response" {
					t.Errorf("Body captures = %+v, want 1 capture named full_response", s.Captures.Body)
				}
			},
		},
		{
			name: "missing_method",
			yaml: `
- url: https://api.example.com/health
`,
			check: func(t *testing.T, steps []Step) {
				assertSingleStep(t, steps, "", "https://api.example.com/health")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.yaml)
			steps, err := Parse(r)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil {
				tt.check(t, steps)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "invalid_yaml_syntax",
			yaml: `
- method: GET
  url: https://api.example.com/health
  headers:
    Content-Type: application/json
    Authorization: [unclosed bracket
`,
		},
		{
			name: "steps_not_list",
			yaml: `
method: GET
url: https://api.example.com/health
`,
		},
		{
			name: "step_not_mapping",
			yaml: `
- "not a mapping"
- method: GET
  url: https://api.example.com/health
`,
		},
		{
			name: "invalid_url_type",
			yaml: `
- method: GET
  url: [not a string]
`,
		},
		{
			name: "invalid_headers_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  headers: "not a map"
`,
		},
		{
			name: "invalid_asserts_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts: "not a map"
`,
		},
		{
			name: "invalid_captures_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  captures: [not a map]
`,
		},
		{
			name: "empty_predicate_mapping",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - {}
`,
		},
		{
			name: "header_missing_name",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    headers:
      - op: exists
`,
		},
		{
			name: "predicate_key_not_string",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - 123: value
`,
		},
		{
			name: "header_key_not_string",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    headers:
      - 123: value
`,
		},
		{
			name: "path_key_not_string",
			yaml: `
- method: GET
  url: https://api.example.com/health
  asserts:
    jsonpath:
      - 123: value
`,
		},
		{
			name: "invalid_options_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  options: "not a map"
`,
		},
		{
			name: "invalid_retries_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  options:
    retries: "not a number"
`,
		},
		{
			name: "invalid_follow_redirect_type",
			yaml: `
- method: GET
  url: https://api.example.com/health
  options:
    follow_redirect: "not a boolean"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.yaml)
			_, err := Parse(r)
			if err == nil {
				t.Error("expected error, got nil")
			}
		})
	}
}

// assertSingleStep is a test helper that verifies basic step properties.
func assertSingleStep(t *testing.T, steps []Step, wantMethod, wantURL string) {
	t.Helper()

	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	s := steps[0]
	if s.Method != wantMethod {
		t.Errorf("Method = %q, want %q", s.Method, wantMethod)
	}
	if s.URL != wantURL {
		t.Errorf("URL = %q, want %q", s.URL, wantURL)
	}
}

func TestParseCertificateAssertions(t *testing.T) {
	yamlContent := `
- method: GET
  url: https://example.com
  asserts:
    certificate:
      - name: subject
        equals: "CN=example.com"
      - name: issuer
        contains: "Let's Encrypt"
      - name: expire_date
        exists: true
      - name: serial_number
        exists: true
`

	steps, err := Parse(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	step := steps[0]
	if len(step.Asserts.Certificate) != 4 {
		t.Fatalf("Expected 4 certificate assertions, got %d", len(step.Asserts.Certificate))
	}

	// Test subject assertion
	subjectAssert := step.Asserts.Certificate[0]
	if subjectAssert.Name != "subject" {
		t.Errorf("Expected name 'subject', got %q", subjectAssert.Name)
	}
	if subjectAssert.Predicate.Operation != "equals" {
		t.Errorf("Expected operation 'equals', got %q", subjectAssert.Predicate.Operation)
	}
	if subjectAssert.Predicate.Value != "CN=example.com" {
		t.Errorf("Expected value 'CN=example.com', got %q", subjectAssert.Predicate.Value)
	}

	// Test issuer assertion
	issuerAssert := step.Asserts.Certificate[1]
	if issuerAssert.Name != "issuer" {
		t.Errorf("Expected name 'issuer', got %q", issuerAssert.Name)
	}
	if issuerAssert.Predicate.Operation != "contains" {
		t.Errorf("Expected operation 'contains', got %q", issuerAssert.Predicate.Operation)
	}

	// Test expire_date assertion
	expireAssert := step.Asserts.Certificate[2]
	if expireAssert.Name != "expire_date" {
		t.Errorf("Expected name 'expire_date', got %q", expireAssert.Name)
	}
	if expireAssert.Predicate.Operation != "exists" {
		t.Errorf("Expected operation 'exists', got %q", expireAssert.Predicate.Operation)
	}

	// Test serial_number assertion
	serialAssert := step.Asserts.Certificate[3]
	if serialAssert.Name != "serial_number" {
		t.Errorf("Expected name 'serial_number', got %q", serialAssert.Name)
	}
	if serialAssert.Predicate.Operation != "exists" {
		t.Errorf("Expected operation 'exists', got %q", serialAssert.Predicate.Operation)
	}
}

func TestParseCertificateCaptures(t *testing.T) {
	yamlContent := `
- method: GET
  url: https://example.com
  captures:
    certificate:
      - name: cert_subject
        certificate_field: subject
      - name: cert_issuer
        certificate_field: issuer
      - name: cert_expiry
        certificate_field: expire_date
      - name: cert_serial
        certificate_field: serial_number
`

	steps, err := Parse(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(steps))
	}

	step := steps[0]
	if step.Captures == nil {
		t.Fatal("Expected captures to be non-nil")
	}
	if len(step.Captures.Certificate) != 4 {
		t.Fatalf("Expected 4 certificate captures, got %d", len(step.Captures.Certificate))
	}

	// Test subject capture
	subjectCapture := step.Captures.Certificate[0]
	if subjectCapture.Name != "cert_subject" {
		t.Errorf("Expected name 'cert_subject', got %q", subjectCapture.Name)
	}
	if subjectCapture.CertificateField != "subject" {
		t.Errorf("Expected certificate_field 'subject', got %q", subjectCapture.CertificateField)
	}

	// Test issuer capture
	issuerCapture := step.Captures.Certificate[1]
	if issuerCapture.Name != "cert_issuer" {
		t.Errorf("Expected name 'cert_issuer', got %q", issuerCapture.Name)
	}
	if issuerCapture.CertificateField != "issuer" {
		t.Errorf("Expected certificate_field 'issuer', got %q", issuerCapture.CertificateField)
	}

	// Test expire_date capture
	expireCapture := step.Captures.Certificate[2]
	if expireCapture.Name != "cert_expiry" {
		t.Errorf("Expected name 'cert_expiry', got %q", expireCapture.Name)
	}
	if expireCapture.CertificateField != "expire_date" {
		t.Errorf("Expected certificate_field 'expire_date', got %q", expireCapture.CertificateField)
	}

	// Test serial_number capture
	serialCapture := step.Captures.Certificate[3]
	if serialCapture.Name != "cert_serial" {
		t.Errorf("Expected name 'cert_serial', got %q", serialCapture.Name)
	}
	if serialCapture.CertificateField != "serial_number" {
		t.Errorf("Expected certificate_field 'serial_number', got %q", serialCapture.CertificateField)
	}
}

func TestCertificateAssertMissingField(t *testing.T) {
	yamlContent := `
- method: GET
  url: https://example.com
  asserts:
    certificate:
      - equals: "CN=example.com"
`

	_, err := Parse(strings.NewReader(yamlContent))
	if err == nil {
		t.Fatal("Expected error for missing name, got nil")
	}

	expectedError := "missing required 'name' field"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error containing %q, got %q", expectedError, err.Error())
	}
}
