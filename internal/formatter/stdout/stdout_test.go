package stdout

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/jacoelho/rq/internal/results"
)

func TestFormatter_Format_SingleSummary(t *testing.T) {
	tests := []struct {
		name     string
		summary  *results.Summary
		expected []string
	}{
		{
			name: "successful_single_file",
			summary: &results.Summary{
				FileResults: []results.FileResult{
					{
						Filename:     "test.yaml",
						RequestCount: 3,
						Duration:     500 * time.Millisecond,
						Error:        nil,
					},
				},
				ExecutedFiles:    1,
				ExecutedRequests: 3,
				SucceededFiles:   1,
				FailedFiles:      0,
				TotalDuration:    500 * time.Millisecond,
			},
			expected: []string{
				"test.yaml: Success (3 request(s) in 500 ms)",
				"Executed files:    1",
				"Executed requests: 3 (6.00/s)",
				"Succeeded files:   1 (100.0%)",
				"Failed files:      0 (0.0%)",
				"Duration:          500 ms",
			},
		},
		{
			name: "failed_single_file",
			summary: &results.Summary{
				FileResults: []results.FileResult{
					{
						Filename:     "test.yaml",
						RequestCount: 2,
						Duration:     200 * time.Millisecond,
						Error:        errors.New("assertion failed"),
					},
				},
				ExecutedFiles:    1,
				ExecutedRequests: 2,
				SucceededFiles:   0,
				FailedFiles:      1,
				TotalDuration:    200 * time.Millisecond,
			},
			expected: []string{
				"test.yaml: Failed: assertion failed (2 request(s) in 200 ms)",
				"Executed files:    1",
				"Executed requests: 2 (10.00/s)",
				"Succeeded files:   0 (0.0%)",
				"Failed files:      1 (100.0%)",
				"Duration:          200 ms",
			},
		},
		{
			name: "multiple_files_mixed_results",
			summary: &results.Summary{
				FileResults: []results.FileResult{
					{
						Filename:     "success.yaml",
						RequestCount: 2,
						Duration:     300 * time.Millisecond,
						Error:        nil,
					},
					{
						Filename:     "failure.yaml",
						RequestCount: 1,
						Duration:     100 * time.Millisecond,
						Error:        errors.New("test failed"),
					},
				},
				ExecutedFiles:    2,
				ExecutedRequests: 3,
				SucceededFiles:   1,
				FailedFiles:      1,
				TotalDuration:    400 * time.Millisecond,
			},
			expected: []string{
				"success.yaml: Success (2 request(s) in 300 ms)",
				"failure.yaml: Failed: test failed (1 request(s) in 100 ms)",
				"Executed files:    2",
				"Executed requests: 3 (7.50/s)",
				"Succeeded files:   1 (50.0%)",
				"Failed files:      1 (50.0%)",
				"Duration:          400 ms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewWithWriter(&buf)
			err := formatter.Format(tt.summary)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			output := buf.String()
			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatter_Format_MultipleSummaries(t *testing.T) {
	tests := []struct {
		name       string
		allResults []*results.Summary
		expected   []string
	}{
		{
			name:       "empty_results",
			allResults: []*results.Summary{},
			expected:   []string{},
		},
		{
			name: "single_result_delegates_to_FormatSingle",
			allResults: []*results.Summary{
				{
					ExecutedFiles:    1,
					ExecutedRequests: 2,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    100 * time.Millisecond,
					FileResults: []results.FileResult{
						{
							Filename:     "single.yaml",
							RequestCount: 2,
							Duration:     100 * time.Millisecond,
							Error:        nil,
						},
					},
				},
			},
			expected: []string{
				"single.yaml: Success (2 request(s) in 100 ms)",
				"Executed files:    1",
			},
		},
		{
			name: "multiple_iterations",
			allResults: []*results.Summary{
				{
					ExecutedFiles:    1,
					ExecutedRequests: 3,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    500 * time.Millisecond,
				},
				{
					ExecutedFiles:    1,
					ExecutedRequests: 3,
					SucceededFiles:   0,
					FailedFiles:      1,
					TotalDuration:    300 * time.Millisecond,
				},
				{
					ExecutedFiles:    1,
					ExecutedRequests: 3,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    400 * time.Millisecond,
				},
			},
			expected: []string{
				"ITERATION RESULTS:",
				"Iteration 1: SUCCESS (1 files, 3 requests, 500 ms)",
				"Iteration 2: FAILED (1 files, 3 requests, 300 ms)",
				"Iteration 3: SUCCESS (1 files, 3 requests, 400 ms)",
				"AGGREGATED RESULTS:",
				"Total iterations:    3",
				"Successful iterations: 2 (66.7%)",
				"Failed iterations:   1 (33.3%)",
				"Total executed files: 3",
				"Total executed requests: 9 (7.50/s)",
				"Total succeeded files: 2",
				"Total failed files:  1",
				"Total duration:      1200 ms",
				"Avg files per iteration: 1.0",
				"Avg requests per iteration: 3.0",
				"Avg duration per iteration: 400 ms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewWithWriter(&buf)
			err := formatter.Format(tt.allResults...)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			output := buf.String()
			if len(tt.expected) == 0 {
				if output != "" {
					t.Errorf("Expected empty output for empty results, got: %s", output)
				}
				return
			}

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestFormatter_Format_AutoDetection(t *testing.T) {
	tests := []struct {
		name      string
		summaries []*results.Summary
		expected  []string
	}{
		{
			name:      "empty_summaries",
			summaries: []*results.Summary{},
			expected:  []string{},
		},
		{
			name: "single_summary",
			summaries: []*results.Summary{
				{
					FileResults: []results.FileResult{
						{
							Filename:     "test.yaml",
							RequestCount: 2,
							Duration:     300 * time.Millisecond,
							Error:        nil,
						},
					},
					ExecutedFiles:    1,
					ExecutedRequests: 2,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    300 * time.Millisecond,
				},
			},
			expected: []string{
				"test.yaml: Success (2 request(s) in 300 ms)",
				"Executed files:    1",
				"Duration:          300 ms",
			},
		},
		{
			name: "multiple_summaries",
			summaries: []*results.Summary{
				{
					ExecutedFiles:    1,
					ExecutedRequests: 2,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    200 * time.Millisecond,
				},
				{
					ExecutedFiles:    1,
					ExecutedRequests: 2,
					SucceededFiles:   1,
					FailedFiles:      0,
					TotalDuration:    300 * time.Millisecond,
				},
			},
			expected: []string{
				"ITERATION RESULTS:",
				"AGGREGATED RESULTS:",
				"Total iterations:    2",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			formatter := NewWithWriter(&buf)
			err := formatter.Format(tt.summaries...)
			if err != nil {
				t.Fatalf("Format() error = %v", err)
			}

			output := buf.String()
			if len(tt.expected) == 0 {
				if output != "" {
					t.Errorf("Expected empty output for empty summaries, got: %s", output)
				}
				return
			}

			for _, expected := range tt.expected {
				if !strings.Contains(output, expected) {
					t.Errorf("Expected output to contain %q, but got:\n%s", expected, output)
				}
			}
		})
	}
}

func TestNew(t *testing.T) {
	formatter := New()
	if formatter == nil {
		t.Error("New() returned nil")
	}
}

func TestNewWithWriter(t *testing.T) {
	var buf bytes.Buffer
	formatter := NewWithWriter(&buf)
	if formatter == nil {
		t.Error("NewWithWriter() returned nil")
	}
}
