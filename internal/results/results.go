package results

import (
	"time"
)

// FileResult contains the execution result for a single test file.
type FileResult struct {
	Filename     string
	RequestCount int
	Duration     time.Duration
	Error        error
}

// FileResultBuilder provides a fluent interface for constructing FileResult objects.
type FileResultBuilder struct {
	filename     string
	requestCount int
	duration     time.Duration
	err          error
}

// NewFileResultBuilder creates a new FileResultBuilder for the given filename.
func NewFileResultBuilder(filename string) *FileResultBuilder {
	return &FileResultBuilder{
		filename: filename,
	}
}

// WithRequestCount sets the request count for the file result.
func (b *FileResultBuilder) WithRequestCount(count int) *FileResultBuilder {
	b.requestCount = count
	return b
}

// WithDuration sets the duration for the file result.
func (b *FileResultBuilder) WithDuration(duration time.Duration) *FileResultBuilder {
	b.duration = duration
	return b
}

// WithError sets the error for the file result.
func (b *FileResultBuilder) WithError(err error) *FileResultBuilder {
	b.err = err
	return b
}

// Build creates the FileResult from the builder.
func (b *FileResultBuilder) Build() FileResult {
	return FileResult{
		Filename:     b.filename,
		RequestCount: b.requestCount,
		Duration:     b.duration,
		Error:        b.err,
	}
}

// Summary contains aggregated results from multiple test file executions.
type Summary struct {
	FileResults      []FileResult
	ExecutedFiles    int
	ExecutedRequests int
	SucceededFiles   int
	FailedFiles      int
	TotalDuration    time.Duration
}

// NewSummary creates a new Summary with initialized fields.
func NewSummary(expectedFiles int) *Summary {
	return &Summary{
		FileResults: make([]FileResult, 0, expectedFiles),
	}
}

// Add materializes a FileResultBuilder and adds it to the summary with aggregate counter updates.
func (s *Summary) Add(builder *FileResultBuilder) {
	result := builder.Build()

	s.FileResults = append(s.FileResults, result)
	s.ExecutedFiles++
	s.ExecutedRequests += result.RequestCount

	if result.Error != nil {
		s.FailedFiles++
	} else {
		s.SucceededFiles++
	}
}

// SetTotalDuration sets the total duration for the summary.
func (s *Summary) SetTotalDuration(duration time.Duration) {
	s.TotalDuration = duration
}

// RequestsPerSecond calculates the overall requests per second.
func (s *Summary) RequestsPerSecond() float64 {
	if s.TotalDuration == 0 {
		return 0
	}
	return float64(s.ExecutedRequests) / s.TotalDuration.Seconds()
}

// SuccessPercentage calculates the percentage of successful files.
func (s *Summary) SuccessPercentage() float64 {
	if s.ExecutedFiles == 0 {
		return 0
	}
	return (float64(s.SucceededFiles) / float64(s.ExecutedFiles)) * 100
}

// FailurePercentage calculates the percentage of failed files.
func (s *Summary) FailurePercentage() float64 {
	if s.ExecutedFiles == 0 {
		return 0
	}
	return (float64(s.FailedFiles) / float64(s.ExecutedFiles)) * 100
}

// AggregatedStats holds calculated statistics from multiple iterations.
type AggregatedStats struct {
	TotalExecutedFiles    int
	TotalExecutedRequests int
	TotalSucceededFiles   int
	TotalFailedFiles      int
	TotalDuration         time.Duration
	SuccessfulIterations  int
	IterationCount        int
}

// CalculateAggregatedStats computes aggregated statistics from multiple iterations.
func CalculateAggregatedStats(allResults []*Summary) AggregatedStats {
	var stats AggregatedStats
	stats.IterationCount = len(allResults)

	for _, results := range allResults {
		stats.TotalExecutedFiles += results.ExecutedFiles
		stats.TotalExecutedRequests += results.ExecutedRequests
		stats.TotalSucceededFiles += results.SucceededFiles
		stats.TotalFailedFiles += results.FailedFiles
		stats.TotalDuration += results.TotalDuration

		if results.FailedFiles == 0 {
			stats.SuccessfulIterations++
		}
	}

	return stats
}
