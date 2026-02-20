package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/pathing"
	"github.com/jacoelho/rq/internal/pm/config"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/rq/model"
)

func TestRunWritesOneFilePerRequest(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	content := `
{
  "info": {"name": "sample", "schema": "v2"},
  "item": [
    {
      "name": "Users",
      "item": [
        {
          "name": "List",
          "event": [{"listen":"test","script":{"exec":["tests[\"response code is 200\"] = responseCode.code === 200;"]}}],
          "request": {
            "method": "GET",
            "url": "https://api.example.com/users"
          }
        },
        {
          "name": "List",
          "request": {
            "method": "GET",
            "url": "https://api.example.com/users/{{id}}"
          }
        }
      ]
    }
  ]
}
`

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		Overwrite:    false,
		DryRun:       false,
		ReportFormat: report.FormatText,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.Total != 2 {
		t.Fatalf("summary.Total = %d", summary.Total)
	}
	if summary.Converted != 2 {
		t.Fatalf("summary.Converted = %d", summary.Converted)
	}

	files, err := filepath.Glob(filepath.Join(outputDir, "users", "list*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("expected 2 generated files, got %d", len(files))
	}
	if _, err := os.Stat(filepath.Join(outputDir, "users", "list-get.yaml")); err != nil {
		t.Fatalf("expected file list-get.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "users", "list-get-1.yaml")); err != nil {
		t.Fatalf("expected file list-get-1.yaml: %v", err)
	}

	for _, generated := range files {
		payload, err := os.ReadFile(generated)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := model.Parse(strings.NewReader(string(payload))); err != nil {
			t.Fatalf("generated file failed model.Parse: %s: %v", generated, err)
		}
	}
}

func TestRunUsesMethodInFilename(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	content := `
{
  "item": [
    {"name":"Health","request":{"method":"GET","url":"https://api.example.com/health"}},
    {"name":"Health","request":{"method":"POST","url":"https://api.example.com/health"}}
  ]
}
`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		DryRun:       false,
		ReportFormat: report.FormatText,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Total != 2 || summary.Converted != 2 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	if _, err := os.Stat(filepath.Join(outputDir, "health-get.yaml")); err != nil {
		t.Fatalf("expected health-get.yaml: %v", err)
	}
	if _, err := os.Stat(filepath.Join(outputDir, "health-post.yaml")); err != nil {
		t.Fatalf("expected health-post.yaml: %v", err)
	}
}

func TestRunDryRunDoesNotWriteFiles(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	content := `{"item":[{"name":"Health","request":{"method":"GET","url":"https://api.example.com/health"}}]}`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		DryRun:       true,
		ReportFormat: report.FormatText,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Total != 1 {
		t.Fatalf("summary.Total = %d", summary.Total)
	}

	if _, err := os.Stat(outputDir); !os.IsNotExist(err) {
		t.Fatalf("output dir should not exist in dry-run, stat err = %v", err)
	}
}

func TestRunSkipsWriteOnFatalConversionIssues(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	content := `
{
  "item": [
    {
      "name": "Upload",
      "request": {
        "method": "POST",
        "url": "https://api.example.com/upload",
        "body": {
          "mode": "formdata",
          "formdata": [
            {"key":"file","type":"file"}
          ]
        }
      }
    }
  ]
}
`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		DryRun:       false,
		ReportFormat: report.FormatText,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if summary.Total != 1 {
		t.Fatalf("summary.Total = %d", summary.Total)
	}
	if summary.Converted != 0 {
		t.Fatalf("summary.Converted = %d, want 0", summary.Converted)
	}
	if summary.Skipped != 1 {
		t.Fatalf("summary.Skipped = %d, want 1", summary.Skipped)
	}
	if len(summary.Requests) != 1 || len(summary.Requests[0].Issues) == 0 {
		t.Fatalf("expected one request with issues, got %+v", summary.Requests)
	}
	if got := summary.Requests[0].Issues[0].Path; got != "Upload" {
		t.Fatalf("issue path = %q, want Upload", got)
	}

	files, err := filepath.Glob(filepath.Join(outputDir, "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Fatalf("expected no written files, got %d", len(files))
	}
}

func TestQualifyIssuesSetsSourcePathWhenMissing(t *testing.T) {
	t.Parallel()

	issues := []report.Issue{
		{Code: report.CodeBodyNotSupported, Message: "x"},
	}

	qualified := qualifyIssues("Folder/Request", issues)
	if len(qualified) != 1 {
		t.Fatalf("len(qualified) = %d", len(qualified))
	}
	if qualified[0].Path != "Folder/Request" {
		t.Fatalf("qualified[0].Path = %q", qualified[0].Path)
	}
}

func TestQualifyIssuesPreservesExistingPath(t *testing.T) {
	t.Parallel()

	issues := []report.Issue{
		{Code: report.CodeOutputExists, Message: "x", Path: "/tmp/out.yaml"},
	}

	qualified := qualifyIssues("Folder/Request", issues)
	if len(qualified) != 1 {
		t.Fatalf("len(qualified) = %d", len(qualified))
	}
	if qualified[0].Path != "/tmp/out.yaml" {
		t.Fatalf("qualified[0].Path = %q", qualified[0].Path)
	}
}

func TestRunRebasesRelativeFileBodyPathFromInputDir(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputDir := filepath.Join(tempDir, "input")
	outputDir := filepath.Join(tempDir, "out")
	if err := os.MkdirAll(inputDir, 0755); err != nil {
		t.Fatal(err)
	}

	inputFile := filepath.Join(inputDir, "collection.json")
	payloadFile := filepath.Join(inputDir, "payload.bin")
	if err := os.WriteFile(payloadFile, []byte("binary-payload"), 0644); err != nil {
		t.Fatal(err)
	}

	content := `
{
  "item": [
    {
      "name": "Upload",
      "request": {
        "method": "PUT",
        "url": "https://api.example.com/upload",
        "body": {
          "mode": "file",
          "file": {
            "src": "payload.bin"
          }
        }
      }
    }
  ]
}
`
	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	summary, err := Run(config.Config{
		InputFile:    inputFile,
		OutputDir:    outputDir,
		DryRun:       false,
		ReportFormat: report.FormatText,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Total != 1 || summary.Converted != 1 {
		t.Fatalf("unexpected summary: %+v", summary)
	}

	generatedPath := filepath.Join(outputDir, "upload-put.yaml")
	generatedRaw, err := os.ReadFile(generatedPath)
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}

	steps, err := model.Parse(strings.NewReader(string(generatedRaw)))
	if err != nil {
		t.Fatalf("model.Parse generated file: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}

	expectedRelative, err := filepath.Rel(filepath.Dir(generatedPath), payloadFile)
	if err != nil {
		t.Fatalf("filepath.Rel: %v", err)
	}
	if steps[0].BodyFile != filepath.ToSlash(expectedRelative) {
		t.Fatalf("body_file = %q, want %q", steps[0].BodyFile, filepath.ToSlash(expectedRelative))
	}
}

func TestRebaseBodyFilePathPreservesAbsoluteLikePaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bodyFile string
	}{
		{
			name:     "posix absolute",
			bodyFile: "/tmp/payload.bin",
		},
		{
			name:     "windows drive absolute backslash",
			bodyFile: `C:\tmp\payload.bin`,
		},
		{
			name:     "windows drive absolute slash",
			bodyFile: `C:/tmp/payload.bin`,
		},
		{
			name:     "windows UNC path",
			bodyFile: `\\server\share\payload.bin`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathing.RebaseBodyFilePath(tt.bodyFile, "/input/collection.json", "/out/request.yaml")
			if got != tt.bodyFile {
				t.Fatalf("RebaseBodyFilePath(%q) = %q, want unchanged", tt.bodyFile, got)
			}
		})
	}
}

func TestRebaseBodyFilePathRebasesTemplatedRelativePath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "in", "collection.json")
	outputFile := filepath.Join(tempDir, "out", "request.yaml")
	bodyFile := "payloads/{{.name}}.bin"

	got := pathing.RebaseBodyFilePath(bodyFile, inputFile, outputFile)

	sourceAbsolute := filepath.Clean(filepath.Join(filepath.Dir(inputFile), bodyFile))
	expectedRelative, err := filepath.Rel(filepath.Dir(outputFile), sourceAbsolute)
	if err != nil {
		t.Fatalf("filepath.Rel failed: %v", err)
	}
	if got != filepath.ToSlash(expectedRelative) {
		t.Fatalf("RebaseBodyFilePath(%q) = %q, want %q", bodyFile, got, filepath.ToSlash(expectedRelative))
	}
}

func TestRebaseBodyFilePathPreservesTemplatedPrefixPath(t *testing.T) {
	t.Parallel()

	tests := []string{
		"{{.payload_dir}}/payload.bin",
		"{{.upload_path}}",
	}

	for _, bodyFile := range tests {
		got := pathing.RebaseBodyFilePath(bodyFile, "/input/collection.json", "/out/request.yaml")
		if got != bodyFile {
			t.Fatalf("RebaseBodyFilePath(%q) = %q, want unchanged", bodyFile, got)
		}
	}
}
