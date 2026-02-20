package output

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestSummaryFormatJSON(t *testing.T) {
	t.Parallel()

	summary := NewSummary(1)
	summary.Add(FileResult{
		Filename:     "test.yaml",
		RequestCount: 2,
		Duration:     1500 * time.Millisecond,
		Error:        errors.New("boom"),
	})
	summary.SetTotalDuration(2 * time.Second)

	var out bytes.Buffer
	if err := summary.Format(FormatJSON, &out); err != nil {
		t.Fatalf("Format() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}

	if payload["executed_files"] != float64(1) {
		t.Fatalf("executed_files = %v, want 1", payload["executed_files"])
	}
	if payload["failed_files"] != float64(1) {
		t.Fatalf("failed_files = %v, want 1", payload["failed_files"])
	}
}

func TestFormatAggregatedJSON(t *testing.T) {
	t.Parallel()

	first := NewSummary(1)
	first.Add(FileResult{Filename: "first.yaml", RequestCount: 1, Duration: 100 * time.Millisecond})
	first.SetTotalDuration(200 * time.Millisecond)

	second := NewSummary(1)
	second.Add(FileResult{Filename: "second.yaml", RequestCount: 2, Duration: 100 * time.Millisecond})
	second.SetTotalDuration(300 * time.Millisecond)

	var out bytes.Buffer
	if err := FormatAggregated(FormatJSON, &out, []*Summary{first, second}); err != nil {
		t.Fatalf("FormatAggregated() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("aggregated result is not valid JSON: %v", err)
	}

	if _, ok := payload["iterations"]; !ok {
		t.Fatalf("iterations key missing from aggregated JSON payload")
	}
	if _, ok := payload["aggregated"]; !ok {
		t.Fatalf("aggregated key missing from aggregated JSON payload")
	}
}

func TestFormatDebugJSON(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := FormatDebug(FormatJSON, &out, "REQUEST", []byte("GET / HTTP/1.1")); err != nil {
		t.Fatalf("FormatDebug() error = %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(out.Bytes(), &payload); err != nil {
		t.Fatalf("debug result is not valid JSON: %v", err)
	}

	if payload["description"] != "REQUEST" {
		t.Fatalf("description = %v, want REQUEST", payload["description"])
	}
}
