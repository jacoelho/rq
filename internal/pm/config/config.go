package config

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/jacoelho/rq/internal/pm/report"
)

var (
	ErrNoArguments         = errors.New("no arguments provided")
	ErrHelp                = errors.New("help requested")
	ErrMissingInput        = errors.New("--input is required")
	ErrMissingOutput       = errors.New("--out is required")
	ErrInvalidReportFormat = errors.New("--report must be one of: text, json")
)

// Config defines CLI options for the collection migration command.
type Config struct {
	InputFile    string
	OutputDir    string
	Overwrite    bool
	DryRun       bool
	ReportFormat report.Format
}

// Parse parses and validates CLI arguments.
func Parse(args []string) (*Config, error) {
	if len(args) == 0 {
		return nil, ErrNoArguments
	}

	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Usage = func() {}

	input := fs.String("input", "", "Path to source collection JSON file")
	out := fs.String("out", "", "Output directory for generated rq YAML files")
	overwrite := fs.Bool("overwrite", false, "Overwrite existing output files")
	dryRun := fs.Bool("dry-run", false, "Run conversion without writing files")
	reportFormat := fs.String("report", "text", "Report format: text or json")

	if err := fs.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return nil, ErrHelp
		}
		return nil, fmt.Errorf("parse arguments: %w", err)
	}

	if *input == "" {
		return nil, ErrMissingInput
	}
	if *out == "" {
		return nil, ErrMissingOutput
	}

	if _, err := os.Stat(*input); err != nil {
		return nil, fmt.Errorf("input file not accessible: %w", err)
	}

	parsedReportFormat, err := parseReportFormat(*reportFormat)
	if err != nil {
		return nil, err
	}

	return &Config{
		InputFile:    *input,
		OutputDir:    *out,
		Overwrite:    *overwrite,
		DryRun:       *dryRun,
		ReportFormat: parsedReportFormat,
	}, nil
}

func parseReportFormat(input string) (report.Format, error) {
	switch strings.ToLower(strings.TrimSpace(input)) {
	case "", string(report.FormatText):
		return report.FormatText, nil
	case string(report.FormatJSON):
		return report.FormatJSON, nil
	default:
		return "", fmt.Errorf("%w, got: %s", ErrInvalidReportFormat, input)
	}
}

// Usage returns command usage text.
func Usage() string {
	return `pm2rq - migrate collection JSON into rq YAML files

Usage:
  pm2rq --input collection.json --out ./migrated [--overwrite] [--dry-run] [--report text|json]

Options:
  --input FILE      Path to source collection JSON file
  --out DIR         Output directory for generated rq YAML files
  --overwrite       Overwrite existing files
  --dry-run         Run conversion without writing files
  --report FORMAT   Report format: text or json (default: text)
  -h, --help        Show this help message`
}
