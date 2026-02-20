package execute

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/rq/model"
)

func TestResolveRequestBody(t *testing.T) {
	t.Parallel()

	t.Run("inline body", func(t *testing.T) {
		t.Parallel()

		body, err := resolveRequestBody(model.Step{Body: `{"id":"{{.id}}"}`}, map[string]any{"id": "123"})
		if err != nil {
			t.Fatalf("resolveRequestBody() error = %v", err)
		}
		if body != `{"id":"123"}` {
			t.Fatalf("body = %q", body)
		}
	})

	t.Run("body_file content", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		filePath := filepath.Join(tempDir, "payload.bin")
		if err := os.WriteFile(filePath, []byte("binary-content"), 0644); err != nil {
			t.Fatal(err)
		}

		step := model.Step{BodyFile: filePath}
		body, err := resolveRequestBody(step, nil)
		if err != nil {
			t.Fatalf("resolveRequestBody() error = %v", err)
		}
		if body != "binary-content" {
			t.Fatalf("body = %q", body)
		}
	})

	t.Run("empty body_file path uses inline body", func(t *testing.T) {
		t.Parallel()

		step := model.Step{Body: "fallback", BodyFile: "   "}
		body, err := resolveRequestBody(step, nil)
		if err != nil {
			t.Fatalf("resolveRequestBody() error = %v", err)
		}
		if body != "fallback" {
			t.Fatalf("body = %q", body)
		}
	})
}

func TestPrepareRequestResolvesBodyFileRelativeToSpecDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tempDir, "payload.txt"), []byte("from-file"), 0644); err != nil {
		t.Fatal(err)
	}

	step := model.Step{
		Method:   "POST",
		URL:      "https://api.example.com/upload",
		BodyFile: "payload.txt",
	}

	req, err := prepareRequest(context.Background(), step, nil, tempDir)
	if err != nil {
		t.Fatalf("prepareRequest() error = %v", err)
	}
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(req.Body) error = %v", err)
	}
	if string(body) != "from-file" {
		t.Fatalf("request body = %q", string(body))
	}
}

func TestPrepareRequestResolvesTemplatedAbsoluteBodyFile(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	payloadDir := filepath.Join(tempDir, "payloads")
	specDir := filepath.Join(tempDir, "spec")
	if err := os.MkdirAll(payloadDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(payloadDir, "payload.bin"), []byte("templated-absolute"), 0644); err != nil {
		t.Fatal(err)
	}

	step := model.Step{
		Method:   "POST",
		URL:      "https://api.example.com/upload",
		BodyFile: "{{.payload_dir}}/payload.bin",
	}

	req, err := prepareRequest(context.Background(), step, map[string]CaptureValue{
		"payload_dir": {Value: payloadDir},
	}, specDir)
	if err != nil {
		t.Fatalf("prepareRequest() error = %v", err)
	}
	defer req.Body.Close()

	body, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("ReadAll(req.Body) error = %v", err)
	}
	if string(body) != "templated-absolute" {
		t.Fatalf("request body = %q", string(body))
	}
}

func TestResolveRequestBodyWithBaseDirKeepsAbsoluteLikePath(t *testing.T) {
	t.Parallel()

	baseDir := t.TempDir()
	step := model.Step{
		BodyFile: `C:/tmp/payload.bin`,
	}

	_, err := resolveRequestBodyWithBaseDir(step, nil, baseDir)
	if err == nil {
		t.Fatal("expected read error for missing body_file")
	}

	joined := filepath.Join(baseDir, `C:/tmp/payload.bin`)
	if strings.Contains(err.Error(), joined) {
		t.Fatalf("error path should not use baseDir join, got: %v", err)
	}
	if !strings.Contains(err.Error(), `C:/tmp/payload.bin`) {
		t.Fatalf("error should include absolute-like path, got: %v", err)
	}
}

func TestExecuteStepWhenCondition(t *testing.T) {
	t.Parallel()

	t.Run("false condition skips request", func(t *testing.T) {
		t.Parallel()

		calls := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		runner := newDefault()
		step := model.Step{
			Method: "GET",
			URL:    server.URL,
			When:   "is_ready == true",
		}
		captures := map[string]CaptureValue{
			"is_ready": {Value: false},
		}

		requestMade, err := runner.executeStep(context.Background(), step, captures, "")
		if err != nil {
			t.Fatalf("executeStep() error = %v", err)
		}
		if requestMade {
			t.Fatal("expected requestMade=false")
		}
		if calls != 0 {
			t.Fatalf("expected 0 calls, got %d", calls)
		}
	})

	t.Run("invalid condition returns error", func(t *testing.T) {
		t.Parallel()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		runner := newDefault()
		step := model.Step{
			Method: "GET",
			URL:    server.URL,
			When:   "missing_var == true",
		}

		requestMade, err := runner.executeStep(context.Background(), step, map[string]CaptureValue{}, "")
		if requestMade {
			t.Fatal("expected requestMade=false")
		}
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "unknown variable") {
			t.Fatalf("expected unknown variable error, got: %v", err)
		}
	})
}

func TestProcessQueryParametersPreservesInsertionOrder(t *testing.T) {
	t.Parallel()

	captures := map[string]any{
		"term": "Install Linux",
	}

	gotURL, err := processQueryParameters(
		"https://api.example.com/search?z=9&sig=abc",
		model.KeyValues{
			{Key: "q", Value: "{{.term}}"},
			{Key: "lang", Value: "en"},
			{Key: "q", Value: "two"},
		},
		captures,
	)
	if err != nil {
		t.Fatalf("processQueryParameters() error = %v", err)
	}

	parsedURL, err := url.Parse(gotURL)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", gotURL, err)
	}

	wantRawQuery := "z=9&sig=abc&q=Install+Linux&lang=en&q=two"
	if parsedURL.RawQuery != wantRawQuery {
		t.Fatalf("RawQuery = %q, want %q", parsedURL.RawQuery, wantRawQuery)
	}
}
