package files

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/rq/internal/pm/config"
	"github.com/jacoelho/rq/internal/pm/report"
)

func TestPipelineGeneratedYAMLExecutesInRQ(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/users" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	content := `{
  "item": [
    {
      "name": "List users",
      "event": [{"listen":"test","script":{"exec":["tests[\"response code is 200\"] = responseCode.code === 200;"]}}],
      "request": {
        "method": "GET",
        "url": "` + server.URL + `/users"
      }
    }
  ]
}`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		Overwrite:    false,
		DryRun:       false,
		ReportFormat: report.FormatJSON,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Converted != 1 || summary.HasErrors() {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	generatedFile := filepath.Join(outputDir, "list-users-get.yaml")
	if _, err := os.Stat(generatedFile); err != nil {
		t.Fatalf("expected generated file: %v", err)
	}

	assertGeneratedFileRunsInRQ(t, generatedFile)
}
