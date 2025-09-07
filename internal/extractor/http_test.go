package extractor

import (
	"net/http"
	"testing"
)

func TestExtractStatusCode(t *testing.T) {
	tests := []struct {
		name       string
		setupResp  func() *http.Response
		wantStatus int
		wantError  bool
	}{
		{
			name: "valid status code",
			setupResp: func() *http.Response {
				return &http.Response{StatusCode: 200}
			},
			wantStatus: 200,
			wantError:  false,
		},
		{
			name: "error status code",
			setupResp: func() *http.Response {
				return &http.Response{StatusCode: 404}
			},
			wantStatus: 404,
			wantError:  false,
		},
		{
			name: "nil response",
			setupResp: func() *http.Response {
				return nil
			},
			wantStatus: 0,
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResp()
			status, err := ExtractStatusCode(resp)

			if (err != nil) != tt.wantError {
				t.Errorf("ExtractStatusCode() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if status != tt.wantStatus {
				t.Errorf("ExtractStatusCode() = %v, want %v", status, tt.wantStatus)
			}
		})
	}
}

func TestExtractHeader(t *testing.T) {
	tests := []struct {
		name           string
		setupResp      func() *http.Response
		headerName     string
		wantValue      string
		wantError      bool
		expectNotFound bool
	}{
		{
			name: "existing header",
			setupResp: func() *http.Response {
				resp := &http.Response{Header: make(http.Header)}
				resp.Header.Set("Content-Type", "application/json")
				return resp
			},
			headerName: "Content-Type",
			wantValue:  "application/json",
			wantError:  false,
		},
		{
			name: "case insensitive header",
			setupResp: func() *http.Response {
				resp := &http.Response{Header: make(http.Header)}
				resp.Header.Set("Content-Type", "text/html")
				return resp
			},
			headerName: "content-type",
			wantValue:  "text/html",
			wantError:  false,
		},
		{
			name: "non-existent header",
			setupResp: func() *http.Response {
				resp := &http.Response{Header: make(http.Header)}
				return resp
			},
			headerName:     "X-Non-Existent",
			wantValue:      "",
			wantError:      true,
			expectNotFound: true,
		},
		{
			name: "nil response",
			setupResp: func() *http.Response {
				return nil
			},
			headerName: "Content-Type",
			wantValue:  "",
			wantError:  true,
		},
		{
			name: "empty header name",
			setupResp: func() *http.Response {
				return &http.Response{Header: make(http.Header)}
			},
			headerName: "",
			wantValue:  "",
			wantError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResp()
			value, err := ExtractHeader(resp, tt.headerName)

			if (err != nil) != tt.wantError {
				t.Errorf("ExtractHeader() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if tt.expectNotFound && !IsNotFound(err) {
				t.Errorf("ExtractHeader() expected ErrNotFound, got %v", err)
				return
			}

			if !tt.wantError && value != tt.wantValue {
				t.Errorf("ExtractHeader() = %v, want %v", value, tt.wantValue)
			}
		})
	}
}

func TestExtractAllHeaders(t *testing.T) {
	tests := []struct {
		name      string
		setupResp func() *http.Response
		wantError bool
		validate  func(t *testing.T, headers map[string][]string)
	}{
		{
			name: "multiple headers",
			setupResp: func() *http.Response {
				resp := &http.Response{Header: make(http.Header)}
				resp.Header.Set("Content-Type", "application/json")
				resp.Header.Set("X-API-Version", "v1.0")
				resp.Header.Add("Accept", "application/json")
				resp.Header.Add("Accept", "text/plain")
				return resp
			},
			wantError: false,
			validate: func(t *testing.T, headers map[string][]string) {
				if len(headers) != 3 {
					t.Errorf("Expected 3 headers, got %d", len(headers))
				}

				if headers["Content-Type"][0] != "application/json" {
					t.Errorf("Expected Content-Type application/json, got %v", headers["Content-Type"])
				}

				if len(headers["Accept"]) != 2 {
					t.Errorf("Expected Accept header to have 2 values, got %d", len(headers["Accept"]))
				}
			},
		},
		{
			name: "empty headers",
			setupResp: func() *http.Response {
				return &http.Response{Header: make(http.Header)}
			},
			wantError: false,
			validate: func(t *testing.T, headers map[string][]string) {
				if len(headers) != 0 {
					t.Errorf("Expected 0 headers, got %d", len(headers))
				}
			},
		},
		{
			name: "nil headers",
			setupResp: func() *http.Response {
				return &http.Response{Header: nil}
			},
			wantError: false,
			validate: func(t *testing.T, headers map[string][]string) {
				if len(headers) != 0 {
					t.Errorf("Expected 0 headers, got %d", len(headers))
				}
			},
		},
		{
			name: "nil response",
			setupResp: func() *http.Response {
				return nil
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := tt.setupResp()
			headers, err := ExtractAllHeaders(resp)

			if (err != nil) != tt.wantError {
				t.Errorf("ExtractAllHeaders() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && tt.validate != nil {
				tt.validate(t, headers)
			}
		})
	}
}

func TestExtractBody(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantBody  string
		wantError bool
	}{
		{
			name:      "valid body",
			body:      []byte("Hello, World!"),
			wantBody:  "Hello, World!",
			wantError: false,
		},
		{
			name:      "empty body",
			body:      []byte(""),
			wantBody:  "",
			wantError: false,
		},
		{
			name:      "JSON body",
			body:      []byte(`{"message": "Hello, World!"}`),
			wantBody:  `{"message": "Hello, World!"}`,
			wantError: false,
		},
		{
			name:      "nil body",
			body:      nil,
			wantBody:  "",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body, err := ExtractBody(tt.body)

			if (err != nil) != tt.wantError {
				t.Errorf("ExtractBody() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError && body != tt.wantBody {
				t.Errorf("ExtractBody() = %v, want %v", body, tt.wantBody)
			}
		})
	}
}

func TestExtractBodyBytes(t *testing.T) {
	tests := []struct {
		name      string
		body      []byte
		wantError bool
	}{
		{
			name:      "valid body",
			body:      []byte("Hello, World!"),
			wantError: false,
		},
		{
			name:      "empty body",
			body:      []byte(""),
			wantError: false,
		},
		{
			name:      "binary body",
			body:      []byte{0x00, 0x01, 0x02, 0xFF},
			wantError: false,
		},
		{
			name:      "nil body",
			body:      nil,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ExtractBodyBytes(tt.body)

			if (err != nil) != tt.wantError {
				t.Errorf("ExtractBodyBytes() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				if len(result) != len(tt.body) {
					t.Errorf("ExtractBodyBytes() length = %v, want %v", len(result), len(tt.body))
				}

				for i, b := range tt.body {
					if result[i] != b {
						t.Errorf("ExtractBodyBytes() byte at index %d = %v, want %v", i, result[i], b)
					}
				}
			}
		})
	}
}
