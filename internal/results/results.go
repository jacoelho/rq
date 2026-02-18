package results

import (
	"time"
)

type FileResult struct {
	Filename     string
	RequestCount int
	Duration     time.Duration
	Error        error
}

type Summary struct {
	FileResults      []FileResult
	ExecutedFiles    int
	ExecutedRequests int
	SucceededFiles   int
	FailedFiles      int
	TotalDuration    time.Duration
}

func NewSummary(expectedFiles int) *Summary {
	return &Summary{
		FileResults: make([]FileResult, 0, expectedFiles),
	}
}

func (s *Summary) Add(result FileResult) {
	s.FileResults = append(s.FileResults, result)
	s.ExecutedFiles++
	s.ExecutedRequests += result.RequestCount

	if result.Error != nil {
		s.FailedFiles++
	} else {
		s.SucceededFiles++
	}
}

func (s *Summary) SetTotalDuration(duration time.Duration) {
	s.TotalDuration = duration
}

func (s *Summary) RequestsPerSecond() float64 {
	if s.TotalDuration == 0 {
		return 0
	}
	return float64(s.ExecutedRequests) / s.TotalDuration.Seconds()
}

func (s *Summary) SuccessPercentage() float64 {
	if s.ExecutedFiles == 0 {
		return 0
	}
	return (float64(s.SucceededFiles) / float64(s.ExecutedFiles)) * 100
}

func (s *Summary) FailurePercentage() float64 {
	if s.ExecutedFiles == 0 {
		return 0
	}
	return (float64(s.FailedFiles) / float64(s.ExecutedFiles)) * 100
}

type AggregatedStats struct {
	TotalExecutedFiles    int
	TotalExecutedRequests int
	TotalSucceededFiles   int
	TotalFailedFiles      int
	TotalDuration         time.Duration
	SuccessfulIterations  int
	IterationCount        int
}

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

func (s AggregatedStats) FailedIterations() int {
	return s.IterationCount - s.SuccessfulIterations
}

func (s AggregatedStats) IterationSuccessRate() float64 {
	if s.IterationCount == 0 {
		return 0
	}

	return float64(s.SuccessfulIterations) / float64(s.IterationCount) * 100
}

func (s AggregatedStats) OverallRequestsPerSecond() float64 {
	if s.TotalDuration <= 0 {
		return 0
	}

	return float64(s.TotalExecutedRequests) / s.TotalDuration.Seconds()
}

func (s AggregatedStats) AvgFilesPerIteration() float64 {
	if s.IterationCount == 0 {
		return 0
	}

	return float64(s.TotalExecutedFiles) / float64(s.IterationCount)
}

func (s AggregatedStats) AvgRequestsPerIteration() float64 {
	if s.IterationCount == 0 {
		return 0
	}

	return float64(s.TotalExecutedRequests) / float64(s.IterationCount)
}

func (s AggregatedStats) AvgDurationPerIteration() time.Duration {
	if s.IterationCount == 0 {
		return 0
	}

	return s.TotalDuration / time.Duration(s.IterationCount)
}
