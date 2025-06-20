package stdout

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/jacoelho/rq/internal/formatter"
	"github.com/jacoelho/rq/internal/results"
)

// Formatter implements stdout-based output formatting.
type Formatter struct {
	writer io.Writer
}

// New creates a new stdout formatter that outputs to stdout.
func New() formatter.Formatter {
	return &Formatter{
		writer: os.Stdout,
	}
}

// NewWithWriter creates a new stdout formatter with a custom writer.
// This is useful for testing or redirecting output to files.
func NewWithWriter(writer io.Writer) formatter.Formatter {
	return &Formatter{
		writer: writer,
	}
}

// Format automatically determines whether to format as single or aggregated results
// based on the number of summaries provided.
func (f *Formatter) Format(summaries ...*results.Summary) error {
	if len(summaries) > 1 {
		return f.formatAggregated(summaries)
	} else if len(summaries) == 1 {
		return f.formatSingle(summaries[0])
	}
	// If no summaries, do nothing
	return nil
}

// formatSingle formats a single iteration summary in stdout format.
func (f *Formatter) formatSingle(s *results.Summary) error {
	// Print individual file results
	for _, fileResult := range s.FileResults {
		status := "Success"
		if fileResult.Error != nil {
			status = fmt.Sprintf("Failed: %v", fileResult.Error)
		}
		_, err := fmt.Fprintf(f.writer, "%s: %s (%d request(s) in %d ms)\n",
			fileResult.Filename, status, fileResult.RequestCount, fileResult.Duration.Milliseconds())
		if err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(f.writer, "--------------------------------------------------------------------------------"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(f.writer, "Executed files:    %d\n", s.ExecutedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Executed requests: %d (%.2f/s)\n", s.ExecutedRequests, s.RequestsPerSecond()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Succeeded files:   %d (%.1f%%)\n", s.SucceededFiles, s.SuccessPercentage()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Failed files:      %d (%.1f%%)\n", s.FailedFiles, s.FailurePercentage()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Duration:          %d ms\n", s.TotalDuration.Milliseconds()); err != nil {
		return err
	}

	return nil
}

// formatAggregated formats results from multiple iterations in stdout format.
func (f *Formatter) formatAggregated(allResults []*results.Summary) error {
	if len(allResults) == 0 {
		return nil
	}

	if len(allResults) == 1 {
		return f.formatSingle(allResults[0])
	}

	stats := results.CalculateAggregatedStats(allResults)

	if err := f.printIterationSummary(allResults); err != nil {
		return err
	}

	return f.printAggregatedSummary(stats)
}

// printIterationSummary prints per-iteration results.
func (f *Formatter) printIterationSummary(allResults []*results.Summary) error {
	if _, err := fmt.Fprintln(f.writer, "================================================================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f.writer, "ITERATION RESULTS:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f.writer, "================================================================================"); err != nil {
		return err
	}

	for i, results := range allResults {
		status := "SUCCESS"
		if results.FailedFiles > 0 {
			status = "FAILED"
		}

		_, err := fmt.Fprintf(f.writer, "Iteration %d: %s (%d files, %d requests, %d ms)\n",
			i+1, status, results.ExecutedFiles, results.ExecutedRequests,
			results.TotalDuration.Milliseconds())
		if err != nil {
			return err
		}
	}

	return nil
}

// printAggregatedSummary prints overall statistics and averages.
func (f *Formatter) printAggregatedSummary(stats results.AggregatedStats) error {
	if _, err := fmt.Fprintln(f.writer, "================================================================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f.writer, "AGGREGATED RESULTS:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f.writer, "================================================================================"); err != nil {
		return err
	}

	successRate := float64(stats.SuccessfulIterations) / float64(stats.IterationCount) * 100
	overallRequestsPerSecond := float64(stats.TotalExecutedRequests) / stats.TotalDuration.Seconds()

	if _, err := fmt.Fprintf(f.writer, "Total iterations:    %d\n", stats.IterationCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Successful iterations: %d (%.1f%%)\n", stats.SuccessfulIterations, successRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Failed iterations:   %d (%.1f%%)\n", stats.IterationCount-stats.SuccessfulIterations, 100-successRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Total executed files: %d\n", stats.TotalExecutedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Total executed requests: %d (%.2f/s)\n", stats.TotalExecutedRequests, overallRequestsPerSecond); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Total succeeded files: %d\n", stats.TotalSucceededFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Total failed files:  %d\n", stats.TotalFailedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Total duration:      %d ms\n", stats.TotalDuration.Milliseconds()); err != nil {
		return err
	}

	avgFilesPerIteration := float64(stats.TotalExecutedFiles) / float64(stats.IterationCount)
	avgRequestsPerIteration := float64(stats.TotalExecutedRequests) / float64(stats.IterationCount)
	avgDurationPerIteration := stats.TotalDuration / time.Duration(stats.IterationCount)

	if _, err := fmt.Fprintln(f.writer, "--------------------------------------------------------------------------------"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Avg files per iteration: %.1f\n", avgFilesPerIteration); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Avg requests per iteration: %.1f\n", avgRequestsPerIteration); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f.writer, "Avg duration per iteration: %d ms\n", avgDurationPerIteration.Milliseconds()); err != nil {
		return err
	}

	return nil
}
