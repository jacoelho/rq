package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jacoelho/rq/internal/pm/report"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "collection.json")
	if err := os.WriteFile(input, []byte(`{"item":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Parse([]string{"pm2rq", "--input", input, "--out", filepath.Join(tempDir, "out"), "--report", "json", "--overwrite", "--dry-run"})
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if cfg.InputFile != input {
		t.Fatalf("InputFile = %q", cfg.InputFile)
	}
	if cfg.ReportFormat != report.FormatJSON {
		t.Fatalf("ReportFormat = %q", cfg.ReportFormat)
	}
	if !cfg.Overwrite {
		t.Fatal("expected Overwrite=true")
	}
	if !cfg.DryRun {
		t.Fatal("expected DryRun=true")
	}
}

func TestParseErrors(t *testing.T) {
	t.Parallel()

	_, err := Parse([]string{"pm2rq"})
	if !errors.Is(err, ErrMissingInput) {
		t.Fatalf("expected ErrMissingInput, got %v", err)
	}

	_, err = Parse([]string{"pm2rq", "--input", "missing.json", "--out", "out"})
	if err == nil {
		t.Fatal("expected error for missing input file")
	}

	tempDir := t.TempDir()
	input := filepath.Join(tempDir, "collection.json")
	if err := os.WriteFile(input, []byte(`{"item":[]}`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err = Parse([]string{"pm2rq", "--input", input, "--out", "out", "--report", "xml"})
	if !errors.Is(err, ErrInvalidReportFormat) {
		t.Fatalf("expected ErrInvalidReportFormat, got %v", err)
	}

	_, err = Parse([]string{"pm2rq", "--help"})
	if !errors.Is(err, ErrHelp) {
		t.Fatalf("expected ErrHelp, got %v", err)
	}
}
