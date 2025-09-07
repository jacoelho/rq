package results

import (
	"fmt"
	"io"
	"time"
)

// OutputFormat represents the output format for results.
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON              // Future
	FormatXML               // Future
)

// Format formats the summary in the specified format to the given writer.
func (s *Summary) Format(format OutputFormat, w io.Writer) error {
	switch format {
	case FormatText:
		return s.formatText(w)
	case FormatJSON:
		// Future implementation
		return fmt.Errorf("JSON format not yet implemented")
	case FormatXML:
		// Future implementation
		return fmt.Errorf("XML format not yet implemented")
	default:
		return s.formatText(w)
	}
}

// FormatText is a convenience method for formatting as text.
func (s *Summary) FormatText(w io.Writer) error {
	return s.Format(FormatText, w)
}

// FormatAggregated formats results from multiple iterations.
func FormatAggregated(format OutputFormat, w io.Writer, summaries []*Summary) error {
	switch format {
	case FormatText:
		return formatAggregatedText(w, summaries)
	case FormatJSON:
		// Future implementation
		return fmt.Errorf("JSON format not yet implemented")
	case FormatXML:
		// Future implementation
		return fmt.Errorf("XML format not yet implemented")
	default:
		return formatAggregatedText(w, summaries)
	}
}

// FormatDebug outputs debug information with a description and data.
func FormatDebug(format OutputFormat, w io.Writer, description string, data []byte) error {
	switch format {
	case FormatText:
		return formatDebugText(w, description, data)
	case FormatJSON:
		// Future implementation
		return fmt.Errorf("JSON debug format not yet implemented")
	case FormatXML:
		// Future implementation
		return fmt.Errorf("XML debug format not yet implemented")
	default:
		return formatDebugText(w, description, data)
	}
}

// formatText formats a single iteration summary in text format.
func (s *Summary) formatText(w io.Writer) error {
	// Print individual file results
	for _, fileResult := range s.FileResults {
		status := "Success"
		if fileResult.Error != nil {
			status = fmt.Sprintf("Failed: %v", fileResult.Error)
		}
		_, err := fmt.Fprintf(w, "%s: %s (%d request(s) in %d ms)\n",
			fileResult.Filename, status, fileResult.RequestCount, fileResult.Duration.Milliseconds())
		if err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintln(w, "--------------------------------------------------------------------------------"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "Executed files:    %d\n", s.ExecutedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Executed requests: %d (%.2f/s)\n", s.ExecutedRequests, s.RequestsPerSecond()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Succeeded files:   %d (%.1f%%)\n", s.SucceededFiles, s.SuccessPercentage()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failed files:      %d (%.1f%%)\n", s.FailedFiles, s.FailurePercentage()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Duration:          %d ms\n", s.TotalDuration.Milliseconds()); err != nil {
		return err
	}

	return nil
}

// formatAggregatedText formats results from multiple iterations in text format.
func formatAggregatedText(w io.Writer, allResults []*Summary) error {
	if len(allResults) == 0 {
		return nil
	}

	if len(allResults) == 1 {
		return allResults[0].formatText(w)
	}

	stats := CalculateAggregatedStats(allResults)

	if err := printIterationSummary(w, allResults); err != nil {
		return err
	}

	return printAggregatedSummary(w, stats)
}

// printIterationSummary prints per-iteration results.
func printIterationSummary(w io.Writer, allResults []*Summary) error {
	if _, err := fmt.Fprintln(w, "================================================================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "ITERATION RESULTS:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "================================================================================"); err != nil {
		return err
	}

	for i, results := range allResults {
		status := "SUCCESS"
		if results.FailedFiles > 0 {
			status = "FAILED"
		}

		_, err := fmt.Fprintf(w, "Iteration %d: %s (%d files, %d requests, %d ms)\n",
			i+1, status, results.ExecutedFiles, results.ExecutedRequests,
			results.TotalDuration.Milliseconds())
		if err != nil {
			return err
		}
	}

	return nil
}

// printAggregatedSummary prints overall statistics and averages.
func printAggregatedSummary(w io.Writer, stats AggregatedStats) error {
	if _, err := fmt.Fprintln(w, "================================================================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "AGGREGATED RESULTS:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "================================================================================"); err != nil {
		return err
	}

	successRate := float64(stats.SuccessfulIterations) / float64(stats.IterationCount) * 100
	overallRequestsPerSecond := float64(stats.TotalExecutedRequests) / stats.TotalDuration.Seconds()

	if _, err := fmt.Fprintf(w, "Total iterations:    %d\n", stats.IterationCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Successful iterations: %d (%.1f%%)\n", stats.SuccessfulIterations, successRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failed iterations:   %d (%.1f%%)\n", stats.IterationCount-stats.SuccessfulIterations, 100-successRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total executed files: %d\n", stats.TotalExecutedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total executed requests: %d (%.2f/s)\n", stats.TotalExecutedRequests, overallRequestsPerSecond); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total succeeded files: %d\n", stats.TotalSucceededFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total failed files:  %d\n", stats.TotalFailedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total duration:      %d ms\n", stats.TotalDuration.Milliseconds()); err != nil {
		return err
	}

	avgFilesPerIteration := float64(stats.TotalExecutedFiles) / float64(stats.IterationCount)
	avgRequestsPerIteration := float64(stats.TotalExecutedRequests) / float64(stats.IterationCount)
	avgDurationPerIteration := stats.TotalDuration / time.Duration(stats.IterationCount)

	if _, err := fmt.Fprintln(w, "--------------------------------------------------------------------------------"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg files per iteration: %.1f\n", avgFilesPerIteration); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg requests per iteration: %.1f\n", avgRequestsPerIteration); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg duration per iteration: %d ms\n", avgDurationPerIteration.Milliseconds()); err != nil {
		return err
	}

	return nil
}

// formatDebugText outputs debug information in text format.
func formatDebugText(w io.Writer, description string, data []byte) error {
	if _, err := fmt.Fprintln(w, "========================================"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s:\n", description); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "========================================"); err != nil {
		return err
	}

	if _, err := w.Write(data); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	return nil
}
