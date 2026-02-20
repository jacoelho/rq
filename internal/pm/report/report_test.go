package report

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/pm/diagnostics"
)

func TestSummaryAddAndHints(t *testing.T) {
	t.Parallel()

	var summary Summary
	summary.Add(RequestResult{Converted: true})
	summary.Add(RequestResult{Converted: true, Issues: []Issue{{Code: CodeTestNotMapped, Message: "x"}}})
	summary.Add(RequestResult{Converted: false, Issues: []Issue{{Code: CodeBodyNotSupported, Message: "y"}}})

	if summary.Total != 3 {
		t.Fatalf("Total = %d", summary.Total)
	}
	if summary.Converted != 1 {
		t.Fatalf("Converted = %d", summary.Converted)
	}
	if summary.Partial != 1 {
		t.Fatalf("Partial = %d", summary.Partial)
	}
	if summary.Skipped != 1 {
		t.Fatalf("Skipped = %d", summary.Skipped)
	}

	hints := summary.Hints()
	if len(hints) == 0 {
		t.Fatal("expected hints")
	}
}

func TestSummaryHintsIncludeTemplatePlaceholderGuidance(t *testing.T) {
	t.Parallel()

	var summary Summary
	summary.Add(RequestResult{
		Converted: true,
		Issues: []Issue{
			{Code: CodeTemplatePlaceholderUnsupported, Message: "unsupported template"},
		},
	})

	hints := summary.Hints()
	if len(hints) != 1 {
		t.Fatalf("hints len = %d", len(hints))
	}
	if !strings.Contains(hints[0], "placeholder") {
		t.Fatalf("unexpected hint: %q", hints[0])
	}
}

func TestWriteText(t *testing.T) {
	t.Parallel()

	summary := Summary{
		Total:     1,
		Converted: 1,
		ByCode: map[IssueCode]int{
			CodeTestNotMapped: 2,
		},
	}

	var buf bytes.Buffer
	if err := summary.Write(&buf, FormatText); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Collection migration summary") {
		t.Fatalf("unexpected output: %s", output)
	}
	if !strings.Contains(output, string(CodeTestNotMapped)) {
		t.Fatalf("missing code in output: %s", output)
	}
}

func TestWriteTextPropagatesWriterError(t *testing.T) {
	t.Parallel()

	summary := Summary{
		Total: 1,
	}

	writer := &failingWriter{}
	if err := summary.Write(writer, FormatText); err == nil {
		t.Fatal("expected write error")
	}
}

func TestHasErrors(t *testing.T) {
	t.Parallel()

	if HasErrors([]Issue{{Code: CodeAuthNotMapped, Severity: diagnostics.SeverityWarning}}) {
		t.Fatal("expected warnings not to be treated as errors")
	}

	if !HasErrors([]Issue{{Code: CodeBodyNotSupported, Severity: diagnostics.SeverityError}}) {
		t.Fatal("expected error severity to be treated as fatal")
	}
}

func TestSummaryHasErrors(t *testing.T) {
	t.Parallel()

	summary := Summary{
		Requests: []RequestResult{
			{
				Converted: false,
				Issues: []Issue{
					{Code: CodeAuthNotMapped, Severity: diagnostics.SeverityWarning},
				},
			},
		},
	}
	if summary.HasErrors() {
		t.Fatal("expected summary with only warnings not to report errors")
	}

	summary.Requests = append(summary.Requests, RequestResult{
		Converted: false,
		Issues: []Issue{
			{Code: CodeBodyNotSupported, Severity: diagnostics.SeverityError},
		},
	})
	if !summary.HasErrors() {
		t.Fatal("expected summary with error issue to report errors")
	}
}

type failingWriter struct{}

func (f *failingWriter) Write(_ []byte) (int, error) {
	return 0, errors.New("write failed")
}
