package runner

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/rq/internal/config"
	"github.com/jacoelho/rq/internal/formatter/stdout"
)

// TestRunnerEndToEnd tests the complete runner workflow with httptest server
func TestRunnerEndToEnd(t *testing.T) {
	tests := []struct {
		name             string
		yamlContent      string
		serverHandler    http.HandlerFunc
		wantFileCount    int
		wantRequestCount int
		wantSuccess      bool
		wantOutput       []string // Strings that should appear in output
	}{
		{
			name: "successful_single_request",
			yamlContent: `- method: GET
  url: {{.baseURL}}/api/users
  asserts:
    status:
      - op: equals
        value: 200
    headers:
      - name: Content-Type
        op: equals
        value: application/json
    jsonpath:
      - path: $.users[0].name
        op: equals
        value: "Alice"`,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"users": [{"name": "Alice", "id": 1}]}`))
			},
			wantFileCount:    1,
			wantRequestCount: 1,
			wantSuccess:      true,
			wantOutput:       []string{"Success", "Executed files:    1", "Executed requests: 1"},
		},
		{
			name: "multiple_requests_with_captures",
			yamlContent: `- method: GET
  url: {{.baseURL}}/api/users/1
  asserts:
    status:
      - op: equals
        value: 200
  captures:
    jsonpath:
      - name: user_id
        path: $.id
      - name: user_name
        path: $.name

- method: POST
  url: {{.baseURL}}/api/posts
  headers:
    Content-Type: application/json
  body: |
    {
      "title": "Test Post",
      "author_id": {{.user_id}},
      "author_name": "{{.user_name}}"
    }
  asserts:
    status:
      - op: equals
        value: 201
    jsonpath:
      - path: $.title
        op: equals
        value: "Test Post"`,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/users/1":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					w.Write([]byte(`{"id": 123, "name": "Alice"}`))
				case "/api/posts":
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusCreated)
					w.Write([]byte(`{"id": 456, "title": "Test Post", "author_id": 123}`))
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			},
			wantFileCount:    1,
			wantRequestCount: 2,
			wantSuccess:      true,
			wantOutput:       []string{"Success", "Executed requests: 2"},
		},
		{
			name: "failed_assertion",
			yamlContent: `- method: GET
  url: {{.baseURL}}/api/status
  asserts:
    status:
      - op: equals
        value: 200
    jsonpath:
      - path: $.status
        op: equals
        value: "active"`,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"status": "inactive"}`))
			},
			wantFileCount:    1,
			wantRequestCount: 1,
			wantSuccess:      false,
			wantOutput:       []string{"Failed", "assertion failed"},
		},
		{
			name: "header_captures_and_assertions",
			yamlContent: `- method: GET
  url: {{.baseURL}}/api/info
  asserts:
    status:
      - op: equals
        value: 200
    headers:
      - name: X-API-Version
        op: equals
        value: "v1.0"
      - name: X-Rate-Limit
        op: regex
        value: "\\d+"
  captures:
    headers:
      - name: api_version
        header: X-API-Version
      - name: rate_limit
        header: X-Rate-Limit`,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("X-API-Version", "v1.0")
				w.Header().Set("X-Rate-Limit", "1000")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(`{"info": "API Information"}`))
			},
			wantFileCount:    1,
			wantRequestCount: 1,
			wantSuccess:      true,
			wantOutput:       []string{"Success", "Executed requests: 1"},
		},
		{
			name: "regex_captures",
			yamlContent: `- method: GET
  url: {{.baseURL}}/api/version
  asserts:
    status:
      - op: equals
        value: 200
  captures:
    regex:
      - name: version_number
        pattern: "Version: (\\d+\\.\\d+\\.\\d+)"
        group: 1
      - name: build_number
        pattern: "Build: (\\d+)"
        group: 1`,
			serverHandler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/plain")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("API Version: 2.1.0, Build: 12345"))
			},
			wantFileCount:    1,
			wantRequestCount: 1,
			wantSuccess:      true,
			wantOutput:       []string{"Success"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(tt.serverHandler)
			defer server.Close()

			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "test.yaml")

			yamlContent := strings.ReplaceAll(tt.yamlContent, "{{.baseURL}}", server.URL)
			if err := os.WriteFile(testFile, []byte(yamlContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			cfg := &config.Config{
				TestFiles: []string{testFile},
				Debug:     false,
				Repeat:    0,
			}

			runner, exitResult := New(cfg)
			if exitResult != nil {
				t.Fatalf("Failed to create runner: %s", exitResult.Message)
			}

			var outputBuf bytes.Buffer
			runner.formatter = stdout.NewWithWriter(&outputBuf)

			ctx := context.Background()
			result, err := runner.ExecuteFiles(ctx, cfg.TestFiles)

			if result != nil {
				if formatErr := runner.formatter.Format(result); formatErr != nil {
					t.Fatalf("Failed to format results: %v", formatErr)
				}
			}
			if tt.wantSuccess && err != nil {
				t.Errorf("Expected success but got error: %v", err)
			}
			if !tt.wantSuccess && err == nil {
				t.Error("Expected error but got success")
			}

			if result != nil {
				if result.ExecutedFiles != tt.wantFileCount {
					t.Errorf("ExecutedFiles = %d, want %d", result.ExecutedFiles, tt.wantFileCount)
				}
				if result.ExecutedRequests != tt.wantRequestCount {
					t.Errorf("ExecutedRequests = %d, want %d", result.ExecutedRequests, tt.wantRequestCount)
				}

				if tt.wantSuccess && result.FailedFiles > 0 {
					t.Errorf("Expected no failed files but got %d", result.FailedFiles)
				}
				if !tt.wantSuccess && result.FailedFiles == 0 {
					t.Error("Expected failed files but got none")
				}
			}

			output := outputBuf.String()
			for _, expected := range tt.wantOutput {
				if !strings.Contains(output, expected) {
					t.Errorf("Output should contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestRunnerEndToEndMultipleFiles(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/test1":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"test": "file1"}`))
		case "/api/test2":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"test": "file2"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()

	file1Content := fmt.Sprintf(`- method: GET
  url: %s/api/test1
  asserts:
    status:
      - op: equals
        value: 200
    jsonpath:
      - path: $.test
        op: equals
        value: "file1"`, server.URL)

	file2Content := fmt.Sprintf(`- method: GET
  url: %s/api/test2
  asserts:
    status:
      - op: equals
        value: 200
    jsonpath:
      - path: $.test
        op: equals
        value: "file2"`, server.URL)

	testFile1 := filepath.Join(tempDir, "test1.yaml")
	testFile2 := filepath.Join(tempDir, "test2.yaml")

	if err := os.WriteFile(testFile1, []byte(file1Content), 0644); err != nil {
		t.Fatalf("Failed to write test file 1: %v", err)
	}
	if err := os.WriteFile(testFile2, []byte(file2Content), 0644); err != nil {
		t.Fatalf("Failed to write test file 2: %v", err)
	}

	cfg := &config.Config{
		TestFiles: []string{testFile1, testFile2},
		Debug:     false,
		Repeat:    0,
	}

	runner, exitResult := New(cfg)
	if exitResult != nil {
		t.Fatalf("Failed to create runner: %s", exitResult.Message)
	}

	var outputBuf bytes.Buffer
	runner.formatter = stdout.NewWithWriter(&outputBuf)

	ctx := context.Background()
	result, err := runner.ExecuteFiles(ctx, cfg.TestFiles)

	if result != nil {
		if formatErr := runner.formatter.Format(result); formatErr != nil {
			t.Fatalf("Failed to format results: %v", formatErr)
		}
	}
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.ExecutedFiles != 2 {
		t.Errorf("ExecutedFiles = %d, want 2", result.ExecutedFiles)
	}
	if result.ExecutedRequests != 2 {
		t.Errorf("ExecutedRequests = %d, want 2", result.ExecutedRequests)
	}
	if result.FailedFiles != 0 {
		t.Errorf("FailedFiles = %d, want 0", result.FailedFiles)
	}
	if result.SucceededFiles != 2 {
		t.Errorf("SucceededFiles = %d, want 2", result.SucceededFiles)
	}

	output := outputBuf.String()
	expectedStrings := []string{
		"test1.yaml: Success",
		"test2.yaml: Success",
		"Executed files:    2",
		"Executed requests: 2",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q, but got:\n%s", expected, output)
		}
	}
}

func TestRunnerEndToEndWithRepeat(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(fmt.Appendf(nil, `{"iteration": %d}`, requestCount))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	yamlContent := fmt.Sprintf(`- method: GET
  url: %s/api/counter
  asserts:
    status:
      - op: equals
        value: 200`, server.URL)

	if err := os.WriteFile(testFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := &config.Config{
		TestFiles: []string{testFile},
		Debug:     false,
		Repeat:    2,
	}

	runner, exitResult := New(cfg)
	if exitResult != nil {
		t.Fatalf("Failed to create runner: %s", exitResult.Message)
	}

	var outputBuf bytes.Buffer
	runner.formatter = stdout.NewWithWriter(&outputBuf)

	ctx := context.Background()
	exitCode := runner.Run(ctx)
	if exitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", exitCode)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}

	output := outputBuf.String()
	expectedStrings := []string{
		"ITERATION RESULTS:",
		"AGGREGATED RESULTS:",
		"Total iterations:    3",
		"Total executed requests: 3",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("Output should contain %q, but got:\n%s", expected, output)
		}
	}
}

func TestRunnerEndToEndWithContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yaml")

	yamlContent := fmt.Sprintf(`- method: GET
  url: %s/api/slow
  asserts:
    status:
      - op: equals
        value: 200`, server.URL)

	if err := os.WriteFile(testFile, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cfg := &config.Config{
		TestFiles: []string{testFile},
		Debug:     false,
		Repeat:    0,
	}

	runner, exitResult := New(cfg)
	if exitResult != nil {
		t.Fatalf("Failed to create runner: %s", exitResult.Message)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := runner.ExecuteFiles(ctx, cfg.TestFiles)
	if err == nil {
		t.Error("Expected context timeout error, got nil")
	}
	if !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Errorf("Expected context deadline exceeded error, got: %v", err)
	}
}

func TestRunnerEndToEndErrorHandling(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantError   bool
		errorSubstr string
	}{
		{
			name: "invalid_yaml",
			yamlContent: `- method: GET
  url: invalid yaml content [
  asserts:
    status: invalid`,
			wantError:   true,
			errorSubstr: "failed to parse",
		},
		{
			name: "invalid_url",
			yamlContent: `- method: GET
  url: "not-a-valid-url"
  asserts:
    status:
      - op: equals
        value: 200`,
			wantError:   true,
			errorSubstr: "unsupported protocol",
		},
		{
			name: "nonexistent_host",
			yamlContent: `- method: GET
  url: "http://nonexistent-host-12345.invalid/api"
  asserts:
    status:
      - op: equals
        value: 200`,
			wantError:   true,
			errorSubstr: "no such host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()
			testFile := filepath.Join(tempDir, "test.yaml")

			if err := os.WriteFile(testFile, []byte(tt.yamlContent), 0644); err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			cfg := &config.Config{
				TestFiles: []string{testFile},
				Debug:     false,
				Repeat:    0,
			}

			runner, exitResult := New(cfg)
			if exitResult != nil {
				t.Fatalf("Failed to create runner: %s", exitResult.Message)
			}

			ctx := context.Background()
			_, err := runner.ExecuteFiles(ctx, cfg.TestFiles)

			if tt.wantError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !strings.Contains(err.Error(), tt.errorSubstr) {
					t.Errorf("Expected error containing %q, got: %v", tt.errorSubstr, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}
