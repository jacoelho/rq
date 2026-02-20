package output

import (
	"encoding/json"
	"fmt"
	"io"
)

// OutputFormat represents the output format for output.
type OutputFormat int

const (
	FormatText OutputFormat = iota
	FormatJSON
)

// Format formats the summary in the specified format to the given writer.
func (s *Summary) Format(format OutputFormat, w io.Writer) error {
	switch format {
	case FormatJSON:
		return s.formatJSON(w)
	case FormatText:
		fallthrough
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
	case FormatJSON:
		return formatAggregatedJSON(w, summaries)
	case FormatText:
		fallthrough
	default:
		return formatAggregatedText(w, summaries)
	}
}

// FormatDebug outputs debug information with a description and data.
func FormatDebug(format OutputFormat, w io.Writer, description string, data []byte) error {
	switch format {
	case FormatJSON:
		return formatDebugJSON(w, description, data)
	case FormatText:
		fallthrough
	default:
		return formatDebugText(w, description, data)
	}
}

// formatText formats a single iteration summary in text format.
func (s *Summary) formatText(w io.Writer) error {
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

type jsonFileResult struct {
	Filename             string `json:"filename"`
	RequestCount         int    `json:"request_count"`
	DurationMilliseconds int64  `json:"duration_ms"`
	Success              bool   `json:"success"`
	Error                string `json:"error,omitempty"`
}

type jsonSummary struct {
	FileResults          []jsonFileResult `json:"file_results"`
	ExecutedFiles        int              `json:"executed_files"`
	ExecutedRequests     int              `json:"executed_requests"`
	SucceededFiles       int              `json:"succeeded_files"`
	FailedFiles          int              `json:"failed_files"`
	DurationMilliseconds int64            `json:"duration_ms"`
	RequestsPerSecond    float64          `json:"requests_per_second"`
	SuccessPercentage    float64          `json:"success_percentage"`
	FailurePercentage    float64          `json:"failure_percentage"`
}

func (s *Summary) toJSONSummary() jsonSummary {
	fileResults := make([]jsonFileResult, 0, len(s.FileResults))
	for _, result := range s.FileResults {
		item := jsonFileResult{
			Filename:             result.Filename,
			RequestCount:         result.RequestCount,
			DurationMilliseconds: result.Duration.Milliseconds(),
			Success:              result.Error == nil,
		}
		if result.Error != nil {
			item.Error = result.Error.Error()
		}
		fileResults = append(fileResults, item)
	}

	return jsonSummary{
		FileResults:          fileResults,
		ExecutedFiles:        s.ExecutedFiles,
		ExecutedRequests:     s.ExecutedRequests,
		SucceededFiles:       s.SucceededFiles,
		FailedFiles:          s.FailedFiles,
		DurationMilliseconds: s.TotalDuration.Milliseconds(),
		RequestsPerSecond:    s.RequestsPerSecond(),
		SuccessPercentage:    s.SuccessPercentage(),
		FailurePercentage:    s.FailurePercentage(),
	}
}

func (s *Summary) formatJSON(w io.Writer) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(s.toJSONSummary())
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

type jsonAggregatedStats struct {
	TotalIterations           int     `json:"total_iterations"`
	SuccessfulIterations      int     `json:"successful_iterations"`
	FailedIterations          int     `json:"failed_iterations"`
	IterationSuccessRate      float64 `json:"iteration_success_rate"`
	TotalExecutedFiles        int     `json:"total_executed_files"`
	TotalExecutedRequests     int     `json:"total_executed_requests"`
	TotalSucceededFiles       int     `json:"total_succeeded_files"`
	TotalFailedFiles          int     `json:"total_failed_files"`
	TotalDurationMilliseconds int64   `json:"total_duration_ms"`
	OverallRequestsPerSecond  float64 `json:"overall_requests_per_second"`
	AvgFilesPerIteration      float64 `json:"avg_files_per_iteration"`
	AvgRequestsPerIteration   float64 `json:"avg_requests_per_iteration"`
	AvgDurationMilliseconds   int64   `json:"avg_duration_ms"`
}

type jsonAggregatedResults struct {
	Iterations []jsonSummary       `json:"iterations"`
	Aggregated jsonAggregatedStats `json:"aggregated"`
}

func formatAggregatedJSON(w io.Writer, allResults []*Summary) error {
	if len(allResults) == 0 {
		return nil
	}

	if len(allResults) == 1 {
		return allResults[0].formatJSON(w)
	}

	stats := CalculateAggregatedStats(allResults)
	iterationResults := make([]jsonSummary, 0, len(allResults))
	for _, summary := range allResults {
		iterationResults = append(iterationResults, summary.toJSONSummary())
	}

	payload := jsonAggregatedResults{
		Iterations: iterationResults,
		Aggregated: jsonAggregatedStats{
			TotalIterations:           stats.IterationCount,
			SuccessfulIterations:      stats.SuccessfulIterations,
			FailedIterations:          stats.FailedIterations(),
			IterationSuccessRate:      stats.IterationSuccessRate(),
			TotalExecutedFiles:        stats.TotalExecutedFiles,
			TotalExecutedRequests:     stats.TotalExecutedRequests,
			TotalSucceededFiles:       stats.TotalSucceededFiles,
			TotalFailedFiles:          stats.TotalFailedFiles,
			TotalDurationMilliseconds: stats.TotalDuration.Milliseconds(),
			OverallRequestsPerSecond:  stats.OverallRequestsPerSecond(),
			AvgFilesPerIteration:      stats.AvgFilesPerIteration(),
			AvgRequestsPerIteration:   stats.AvgRequestsPerIteration(),
			AvgDurationMilliseconds:   stats.AvgDurationPerIteration().Milliseconds(),
		},
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

// printIterationSummary prints per-iteration output.
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

	successRate := stats.IterationSuccessRate()
	failureRate := 100 - successRate

	if _, err := fmt.Fprintf(w, "Total iterations:    %d\n", stats.IterationCount); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Successful iterations: %d (%.1f%%)\n", stats.SuccessfulIterations, successRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Failed iterations:   %d (%.1f%%)\n", stats.FailedIterations(), failureRate); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total executed files: %d\n", stats.TotalExecutedFiles); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Total executed requests: %d (%.2f/s)\n", stats.TotalExecutedRequests, stats.OverallRequestsPerSecond()); err != nil {
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

	if _, err := fmt.Fprintln(w, "--------------------------------------------------------------------------------"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg files per iteration: %.1f\n", stats.AvgFilesPerIteration()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg requests per iteration: %.1f\n", stats.AvgRequestsPerIteration()); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Avg duration per iteration: %d ms\n", stats.AvgDurationPerIteration().Milliseconds()); err != nil {
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

type debugOutput struct {
	Description string `json:"description"`
	Data        string `json:"data"`
}

func formatDebugJSON(w io.Writer, description string, data []byte) error {
	encoder := json.NewEncoder(w)
	return encoder.Encode(debugOutput{
		Description: description,
		Data:        string(data),
	})
}
