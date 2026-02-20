package output

import (
	"errors"
	"testing"
	"time"
)

func TestSummary_RequestsPerSecond(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		expected float64
	}{
		{
			name: "normal_calculation",
			summary: Summary{
				ExecutedRequests: 10,
				TotalDuration:    2 * time.Second,
			},
			expected: 5.0,
		},
		{
			name: "zero_duration",
			summary: Summary{
				ExecutedRequests: 10,
				TotalDuration:    0,
			},
			expected: 0.0,
		},
		{
			name: "fractional_result",
			summary: Summary{
				ExecutedRequests: 3,
				TotalDuration:    2 * time.Second,
			},
			expected: 1.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.summary.RequestsPerSecond()
			if result != tt.expected {
				t.Errorf("RequestsPerSecond() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSummary_SuccessPercentage(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		expected float64
	}{
		{
			name: "all_successful",
			summary: Summary{
				ExecutedFiles:  5,
				SucceededFiles: 5,
			},
			expected: 100.0,
		},
		{
			name: "partial_success",
			summary: Summary{
				ExecutedFiles:  10,
				SucceededFiles: 7,
			},
			expected: 70.0,
		},
		{
			name: "no_files",
			summary: Summary{
				ExecutedFiles:  0,
				SucceededFiles: 0,
			},
			expected: 0.0,
		},
		{
			name: "no_success",
			summary: Summary{
				ExecutedFiles:  5,
				SucceededFiles: 0,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.summary.SuccessPercentage()
			if result != tt.expected {
				t.Errorf("SuccessPercentage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSummary_FailurePercentage(t *testing.T) {
	tests := []struct {
		name     string
		summary  Summary
		expected float64
	}{
		{
			name: "no_failures",
			summary: Summary{
				ExecutedFiles: 5,
				FailedFiles:   0,
			},
			expected: 0.0,
		},
		{
			name: "partial_failure",
			summary: Summary{
				ExecutedFiles: 10,
				FailedFiles:   3,
			},
			expected: 30.0,
		},
		{
			name: "all_failed",
			summary: Summary{
				ExecutedFiles: 5,
				FailedFiles:   5,
			},
			expected: 100.0,
		},
		{
			name: "no_files",
			summary: Summary{
				ExecutedFiles: 0,
				FailedFiles:   0,
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.summary.FailurePercentage()
			if result != tt.expected {
				t.Errorf("FailurePercentage() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateAggregatedStats(t *testing.T) {
	summaries := []*Summary{
		{
			ExecutedFiles:    2,
			ExecutedRequests: 10,
			SucceededFiles:   2,
			FailedFiles:      0,
			TotalDuration:    1 * time.Second,
		},
		{
			ExecutedFiles:    3,
			ExecutedRequests: 15,
			SucceededFiles:   2,
			FailedFiles:      1,
			TotalDuration:    2 * time.Second,
		},
		{
			ExecutedFiles:    1,
			ExecutedRequests: 5,
			SucceededFiles:   1,
			FailedFiles:      0,
			TotalDuration:    500 * time.Millisecond,
		},
	}

	stats := CalculateAggregatedStats(summaries)

	expected := AggregatedStats{
		TotalExecutedFiles:    6,
		TotalExecutedRequests: 30,
		TotalSucceededFiles:   5,
		TotalFailedFiles:      1,
		TotalDuration:         3500 * time.Millisecond,
		SuccessfulIterations:  2, // iterations 1 and 3 had no failures
		IterationCount:        3,
	}

	if stats != expected {
		t.Errorf("CalculateAggregatedStats() = %+v, want %+v", stats, expected)
	}
}

func TestNewSummary(t *testing.T) {
	expectedFiles := 3
	summary := NewSummary(expectedFiles)

	if summary == nil {
		t.Fatal("NewSummary() returned nil")
	}
	if cap(summary.FileResults) != expectedFiles {
		t.Errorf("NewSummary().FileResults capacity = %v, want %v", cap(summary.FileResults), expectedFiles)
	}
	if len(summary.FileResults) != 0 {
		t.Errorf("NewSummary().FileResults length = %v, want 0", len(summary.FileResults))
	}
}

func TestSummary_AddFileResult_Migration(t *testing.T) {
	summary := NewSummary(2)

	summary.Add(FileResult{
		Filename:     "success.yaml",
		RequestCount: 3,
		Duration:     time.Second,
	})

	if len(summary.FileResults) != 1 {
		t.Errorf("After adding 1 result, FileResults length = %v, want 1", len(summary.FileResults))
	}
	if summary.ExecutedFiles != 1 {
		t.Errorf("After adding 1 result, ExecutedFiles = %v, want 1", summary.ExecutedFiles)
	}
	if summary.ExecutedRequests != 3 {
		t.Errorf("After adding 1 result, ExecutedRequests = %v, want 3", summary.ExecutedRequests)
	}
	if summary.SucceededFiles != 1 {
		t.Errorf("After adding 1 successful result, SucceededFiles = %v, want 1", summary.SucceededFiles)
	}
	if summary.FailedFiles != 0 {
		t.Errorf("After adding 1 successful result, FailedFiles = %v, want 0", summary.FailedFiles)
	}

	summary.Add(FileResult{
		Filename:     "fail.yaml",
		RequestCount: 2,
		Duration:     time.Second,
		Error:        errors.New("test error"),
	})

	if len(summary.FileResults) != 2 {
		t.Errorf("After adding 2 results, FileResults length = %v, want 2", len(summary.FileResults))
	}
	if summary.ExecutedFiles != 2 {
		t.Errorf("After adding 2 results, ExecutedFiles = %v, want 2", summary.ExecutedFiles)
	}
	if summary.ExecutedRequests != 5 {
		t.Errorf("After adding 2 results, ExecutedRequests = %v, want 5", summary.ExecutedRequests)
	}
	if summary.SucceededFiles != 1 {
		t.Errorf("After adding 1 success + 1 failure, SucceededFiles = %v, want 1", summary.SucceededFiles)
	}
	if summary.FailedFiles != 1 {
		t.Errorf("After adding 1 success + 1 failure, FailedFiles = %v, want 1", summary.FailedFiles)
	}
}

func TestSummary_SetTotalDuration(t *testing.T) {
	summary := NewSummary(1)
	duration := 5 * time.Second

	summary.SetTotalDuration(duration)

	if summary.TotalDuration != duration {
		t.Errorf("SetTotalDuration() set duration to %v, want %v", summary.TotalDuration, duration)
	}
}

func TestSummary_Add(t *testing.T) {
	summary := NewSummary(2)

	summary.Add(FileResult{
		Filename:     "success.yaml",
		RequestCount: 3,
		Duration:     time.Second,
	})

	if len(summary.FileResults) != 1 {
		t.Errorf("After adding 1 result, FileResults length = %v, want 1", len(summary.FileResults))
	}
	if summary.ExecutedFiles != 1 {
		t.Errorf("After adding 1 result, ExecutedFiles = %v, want 1", summary.ExecutedFiles)
	}
	if summary.ExecutedRequests != 3 {
		t.Errorf("After adding 1 result, ExecutedRequests = %v, want 3", summary.ExecutedRequests)
	}
	if summary.SucceededFiles != 1 {
		t.Errorf("After adding 1 successful result, SucceededFiles = %v, want 1", summary.SucceededFiles)
	}
	if summary.FailedFiles != 0 {
		t.Errorf("After adding 1 successful result, FailedFiles = %v, want 0", summary.FailedFiles)
	}

	summary.Add(FileResult{
		Filename:     "fail.yaml",
		RequestCount: 2,
		Duration:     time.Second,
		Error:        errors.New("test error"),
	})

	if len(summary.FileResults) != 2 {
		t.Errorf("After adding 2 results, FileResults length = %v, want 2", len(summary.FileResults))
	}
	if summary.ExecutedFiles != 2 {
		t.Errorf("After adding 2 results, ExecutedFiles = %v, want 2", summary.ExecutedFiles)
	}
	if summary.ExecutedRequests != 5 {
		t.Errorf("After adding 2 results, ExecutedRequests = %v, want 5", summary.ExecutedRequests)
	}
	if summary.SucceededFiles != 1 {
		t.Errorf("After adding 1 success + 1 failure, SucceededFiles = %v, want 1", summary.SucceededFiles)
	}
	if summary.FailedFiles != 1 {
		t.Errorf("After adding 1 success + 1 failure, FailedFiles = %v, want 1", summary.FailedFiles)
	}

	// Verify the actual file results
	firstResult := summary.FileResults[0]
	if firstResult.Filename != "success.yaml" {
		t.Errorf("First result filename = %v, want success.yaml", firstResult.Filename)
	}
	if firstResult.Error != nil {
		t.Errorf("First result error = %v, want nil", firstResult.Error)
	}

	secondResult := summary.FileResults[1]
	if secondResult.Filename != "fail.yaml" {
		t.Errorf("Second result filename = %v, want fail.yaml", secondResult.Filename)
	}
	if secondResult.Error == nil {
		t.Error("Second result error = nil, want non-nil error")
	}
}
