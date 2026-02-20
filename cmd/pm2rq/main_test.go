package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReturnsZeroForSuccessfulMigration(t *testing.T) {
	t.Parallel()

	exitCode, outputDir := runMigration(t, `
{
  "item": [
    {
      "name": "Health",
      "request": {
        "method": "GET",
        "url": "https://api.example.com/health"
      }
    }
  ]
}
`)

	if exitCode != 0 {
		t.Fatalf("run() exitCode = %d, want 0", exitCode)
	}
	if count := countYAMLFiles(t, outputDir); count != 1 {
		t.Fatalf("expected 1 generated file, got %d", count)
	}
}

func TestRunReturnsNonZeroForFatalDiagnostics(t *testing.T) {
	t.Parallel()

	exitCode, outputDir := runMigration(t, `
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
`)

	if exitCode != 1 {
		t.Fatalf("run() exitCode = %d, want 1", exitCode)
	}
	if count := countYAMLFiles(t, outputDir); count != 0 {
		t.Fatalf("expected no generated files on fatal diagnostics, got %d", count)
	}
}

func TestRunReturnsNonZeroForFatalScriptDiagnostics(t *testing.T) {
	t.Parallel()

	exitCode, outputDir := runMigration(t, `
{
  "item": [
    {
      "name": "Users",
      "event": [
        {
          "listen": "test",
          "script": {
            "exec": [
              "tests[\"x\"] = pm.response.json().ok === true;"
            ]
          }
        }
      ],
      "request": {
        "method": "GET",
        "url": "https://api.example.com/users"
      }
    }
  ]
}
`)

	if exitCode != 1 {
		t.Fatalf("run() exitCode = %d, want 1", exitCode)
	}
	if count := countYAMLFiles(t, outputDir); count != 0 {
		t.Fatalf("expected no generated files on fatal script diagnostics, got %d", count)
	}
}

func TestRunReturnsZeroWhenOnlyWarningsExist(t *testing.T) {
	t.Parallel()

	exitCode, outputDir := runMigration(t, `
{
  "item": [
    {
      "name": "Auth",
      "request": {
        "method": "GET",
        "url": "https://api.example.com/users",
        "auth": {"type": "basic"}
      }
    }
  ]
}
`)

	if exitCode != 0 {
		t.Fatalf("run() exitCode = %d, want 0 for warning-only diagnostics", exitCode)
	}
	if count := countYAMLFiles(t, outputDir); count != 1 {
		t.Fatalf("expected 1 generated file with warning-only diagnostics, got %d", count)
	}
}

func runMigration(t *testing.T, content string) (int, string) {
	t.Helper()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "collection.json")
	outputDir := filepath.Join(tempDir, "out")

	if err := os.WriteFile(inputFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	exitCode := run([]string{
		"pm2rq",
		"--input", inputFile,
		"--out", outputDir,
	})
	return exitCode, outputDir
}

func countYAMLFiles(t *testing.T, root string) int {
	t.Helper()

	count := 0
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return nil
			}
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(path, ".yaml") {
			count++
		}
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	return count
}
