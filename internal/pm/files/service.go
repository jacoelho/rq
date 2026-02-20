package files

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jacoelho/rq/internal/pathing"
	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/config"
	"github.com/jacoelho/rq/internal/pm/diagnostics"
	"github.com/jacoelho/rq/internal/pm/naming"
	"github.com/jacoelho/rq/internal/pm/normalize"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/pm/requestmap"
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/yaml"
)

var errOutputExists = errors.New("output file already exists")

// Run executes the collection-to-rq migration.
func Run(cfg config.Config) (report.Summary, error) {
	file, err := os.Open(cfg.InputFile)
	if err != nil {
		return report.Summary{}, fmt.Errorf("open input file: %w", err)
	}
	defer file.Close()

	collection, err := ast.Parse(file)
	if err != nil {
		return report.Summary{}, fmt.Errorf("parse collection: %w", err)
	}

	nodes := normalize.Requests(collection)
	planner := naming.NewPlanner()
	var summary report.Summary

	if !cfg.DryRun {
		if err := os.MkdirAll(cfg.OutputDir, 0755); err != nil {
			return report.Summary{}, fmt.Errorf("create output directory: %w", err)
		}
	}

	for _, node := range nodes {
		converted := requestmap.Request(node)
		sourcePath := strings.Join(node.FullPath(), "/")
		issues := qualifyIssues(sourcePath, converted.Issues)
		methodForName := converted.Step.Method
		if methodForName == "" {
			methodForName = node.Request.Method
		}
		relativePath := planner.Next(node.FolderPath, node.Name, methodForName)
		absolutePath := filepath.Join(cfg.OutputDir, relativePath)

		if converted.Converted {
			converted.Step.BodyFile = pathing.RebaseBodyFilePath(converted.Step.BodyFile, cfg.InputFile, absolutePath)
		}

		entry := report.RequestResult{
			SourcePath: sourcePath,
			OutputPath: relativePath,
			Converted:  converted.Converted && !report.HasErrors(issues),
			Issues:     append([]report.Issue(nil), issues...),
		}

		if entry.Converted && !cfg.DryRun {
			if err := writeStepFile(absolutePath, cfg.Overwrite, converted.Step); err != nil {
				if errors.Is(err, errOutputExists) {
					entry.Converted = false
					entry.Issues = append(entry.Issues, report.Issue{
						Code:     report.CodeOutputExists,
						Stage:    diagnostics.StageFiles,
						Severity: diagnostics.SeverityWarning,
						Path:     absolutePath,
						Message:  fmt.Sprintf("output file exists and --overwrite is false: %s", absolutePath),
					})
				} else {
					return report.Summary{}, fmt.Errorf("write output file: %w", err)
				}
			}
		}

		summary.Add(entry)
	}

	return summary, nil
}

func qualifyIssues(sourcePath string, issues []report.Issue) []report.Issue {
	if len(issues) == 0 {
		return nil
	}

	qualified := make([]report.Issue, len(issues))
	for index := range issues {
		qualified[index] = issues[index]
		if strings.TrimSpace(qualified[index].Path) == "" {
			qualified[index].Path = sourcePath
		}
	}

	return qualified
}

func writeStepFile(filename string, overwrite bool, step model.Step) error {
	if !overwrite {
		if _, err := os.Stat(filename); err == nil {
			return errOutputExists
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("stat output file: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	payload, err := yaml.EncodeStep(step)
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, payload, 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
