package report

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"

	"github.com/jacoelho/rq/internal/pm/diagnostics"
)

// Format determines how summaries are printed.
type Format string

const (
	FormatText Format = "text"
	FormatJSON Format = "json"
)

// IssueCode classifies conversion limitations and skips.
type IssueCode = diagnostics.Code

const (
	CodeInvalidRequestShape             = diagnostics.CodeInvalidRequestShape
	CodeAuthNotMapped                   = diagnostics.CodeAuthNotMapped
	CodeBodyNotSupported                = diagnostics.CodeBodyNotSupported
	CodeTestNotMapped                   = diagnostics.CodeTestNotMapped
	CodeScriptLineUnmapped              = diagnostics.CodeScriptLineUnmapped
	CodeScriptExpressionNotSupported    = diagnostics.CodeScriptExpressionNotSupported
	CodeScriptJSONPathTranslationFailed = diagnostics.CodeScriptJSONPathTranslationFailed
	CodeQueryDuplicate                  = diagnostics.CodeQueryDuplicate
	CodeTemplatePlaceholderUnsupported  = diagnostics.CodeTemplatePlaceholderUnsupported
	CodeOutputExists                    = diagnostics.CodeOutputExists
)

// Issue captures a specific conversion warning/error.
type Issue = diagnostics.Issue

// HasErrors reports whether any issue is error-severity.
// Missing severity defaults to warning for backward compatibility.
func HasErrors(issues []Issue) bool {
	for _, issue := range issues {
		switch issue.Severity {
		case diagnostics.SeverityError:
			return true
		case "":
			// Keep older issue payloads non-fatal unless explicitly classified as error.
		}
	}

	return false
}

// RequestResult is the per-request migration outcome.
type RequestResult struct {
	SourcePath string  `json:"source_path"`
	OutputPath string  `json:"output_path,omitempty"`
	Converted  bool    `json:"converted"`
	Issues     []Issue `json:"issues,omitempty"`
}

// Summary aggregates outcomes across the full collection conversion.
type Summary struct {
	Total     int               `json:"total"`
	Converted int               `json:"converted"`
	Partial   int               `json:"partial"`
	Skipped   int               `json:"skipped"`
	ByCode    map[IssueCode]int `json:"by_code,omitempty"`
	Requests  []RequestResult   `json:"requests,omitempty"`
}

// HasErrors reports whether the summary contains any error-severity issue.
func (s Summary) HasErrors() bool {
	for _, request := range s.Requests {
		if HasErrors(request.Issues) {
			return true
		}
	}

	return false
}

// Add records one request result into the summary.
func (s *Summary) Add(result RequestResult) {
	s.Total++
	if s.ByCode == nil {
		s.ByCode = make(map[IssueCode]int)
	}

	for _, issue := range result.Issues {
		s.ByCode[issue.Code]++
	}

	s.Requests = append(s.Requests, result)

	switch {
	case !result.Converted:
		s.Skipped++
	case len(result.Issues) > 0:
		s.Partial++
	default:
		s.Converted++
	}
}

// Hints returns prioritized extension opportunities inferred from issues.
func (s Summary) Hints() []string {
	hintsByCode := map[IssueCode]string{
		CodeTestNotMapped:                   "Add richer script assertion extraction to map more test checks into rq asserts.",
		CodeScriptLineUnmapped:              "Add more script pattern handlers for remaining assertion/capture forms.",
		CodeScriptExpressionNotSupported:    "Add conditional/control-flow aware script translation.",
		CodeScriptJSONPathTranslationFailed: "Expand JavaScript expression to JSONPath translation support.",
		CodeAuthNotMapped:                   "Add direct auth strategy conversion (basic, bearer, oauth2) to rq-native fields/headers.",
		CodeBodyNotSupported:                "Add multipart/file body mapping support.",
		CodeTemplatePlaceholderUnsupported:  "Map unsupported placeholder syntaxes to rq templates/functions or adjust generated templates manually.",
	}

	type pair struct {
		code  IssueCode
		count int
	}
	var ranked []pair
	for code, count := range s.ByCode {
		if _, ok := hintsByCode[code]; !ok {
			continue
		}
		ranked = append(ranked, pair{code: code, count: count})
	}

	slices.SortFunc(ranked, func(a, b pair) int {
		if a.count == b.count {
			if a.code < b.code {
				return -1
			}
			if a.code > b.code {
				return 1
			}
			return 0
		}
		if a.count > b.count {
			return -1
		}
		return 1
	})

	hints := make([]string, 0, len(ranked))
	for _, entry := range ranked {
		hints = append(hints, hintsByCode[entry.code])
	}

	return hints
}

// Write prints the summary in the requested format.
func (s Summary) Write(w io.Writer, format Format) error {
	switch format {
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(s)
	case FormatText, "":
		writef := func(format string, args ...any) error {
			if _, err := fmt.Fprintf(w, format, args...); err != nil {
				return err
			}
			return nil
		}

		if err := writef("Collection migration summary\n"); err != nil {
			return err
		}
		if err := writef("  total requests: %d\n", s.Total); err != nil {
			return err
		}
		if err := writef("  converted: %d\n", s.Converted); err != nil {
			return err
		}
		if err := writef("  partial: %d\n", s.Partial); err != nil {
			return err
		}
		if err := writef("  skipped: %d\n", s.Skipped); err != nil {
			return err
		}

		if len(s.ByCode) > 0 {
			if err := writef("\nIssues by code:\n"); err != nil {
				return err
			}
			codes := make([]IssueCode, 0, len(s.ByCode))
			for code := range s.ByCode {
				codes = append(codes, code)
			}
			slices.Sort(codes)
			for _, code := range codes {
				if err := writef("  - %s: %d\n", code, s.ByCode[code]); err != nil {
					return err
				}
			}
		}

		hints := s.Hints()
		if len(hints) > 0 {
			if err := writef("\nExtension opportunities:\n"); err != nil {
				return err
			}
			for _, hint := range hints {
				if err := writef("  - %s\n", hint); err != nil {
					return err
				}
			}
		}

		return nil
	default:
		return fmt.Errorf("unsupported report format: %s", format)
	}
}
