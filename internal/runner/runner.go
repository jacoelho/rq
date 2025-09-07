package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/jacoelho/rq/internal/config"
	"github.com/jacoelho/rq/internal/exit"
	"github.com/jacoelho/rq/internal/parser"
	"github.com/jacoelho/rq/internal/ratelimit"
	"github.com/jacoelho/rq/internal/results"
)

// Runner executes HTTP test workflows.
type Runner struct {
	client      *http.Client
	variables   map[string]any
	config      *config.Config
	rateLimiter *ratelimit.Limiter
	output      io.Writer
}

// New creates a new Runner with the given configuration.
func New(cfg *config.Config) (*Runner, *exit.Result) {
	client, err := cfg.HTTPClient()
	if err != nil {
		return nil, exit.Errorf("Error creating runner: %v\n", err)
	}

	rateLimiter := ratelimit.New(cfg.RateLimit)

	return &Runner{
		client:      client,
		variables:   cfg.AllVariables(),
		config:      cfg,
		rateLimiter: rateLimiter,
		output:      os.Stdout,
	}, nil
}

// SetOutput sets a custom writer for output. Used primarily for testing.
func (r *Runner) SetOutput(w io.Writer) {
	r.output = w
}

// Run executes the test files according to the configuration.
func (r *Runner) Run(ctx context.Context) int {
	if r.config.Repeat < 0 {
		return r.runInfiniteLoop(ctx)
	}
	return r.runFiniteLoop(ctx)
}

// runInfiniteLoop handles infinite execution (repeat < 0)
func (r *Runner) runInfiniteLoop(ctx context.Context) int {
	iteration := 1

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("\nInterrupted after %d iterations\n", iteration-1)
			return 1
		default:
		}

		if r.config.Debug {
			fmt.Printf("--- Iteration %d ---\n", iteration)
		}

		result, err := r.runOnce(ctx)
		if err != nil {
			fmt.Printf("\nError in iteration %d: %v\n", iteration, err)
			return 1
		}

		// Print results immediately for each iteration in infinite mode
		if result != nil {
			if err := result.Format(results.FormatText, r.output); err != nil {
				fmt.Printf("Error formatting results: %v\n", err)
			}
		}

		iteration++
	}
}

// runFiniteLoop handles finite execution (repeat >= 0)
func (r *Runner) runFiniteLoop(ctx context.Context) int {
	var allResults []*results.Summary
	totalIterations := r.config.Repeat + 1

	for i := 1; i <= totalIterations; i++ {
		select {
		case <-ctx.Done():
			fmt.Printf("\nInterrupted after %d of %d iterations\n", i-1, totalIterations)
			return 1
		default:
		}

		if r.config.Debug && totalIterations > 1 {
			fmt.Printf("--- Iteration %d of %d ---\n", i, totalIterations)
		}

		result, err := r.runOnce(ctx)
		if err != nil {
			fmt.Printf("\nError in iteration %d: %v\n", i, err)
			return 1
		}

		if result != nil {
			allResults = append(allResults, result)
		}
	}

	if err := results.FormatAggregated(results.FormatText, r.output, allResults); err != nil {
		fmt.Printf("Error formatting results: %v\n", err)
	}
	return 0
}

// runOnce executes the test files once and returns the results
func (r *Runner) runOnce(ctx context.Context) (*results.Summary, error) {
	return r.ExecuteFiles(ctx, r.config.TestFiles)
}

// ExecuteFiles executes multiple test files and returns aggregated results.
func (r *Runner) ExecuteFiles(ctx context.Context, files []string) (*results.Summary, error) {
	s := results.NewSummary(len(files))

	overallStart := time.Now()
	var firstError error

	for _, filename := range files {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		default:
		}

		start := time.Now()
		requestCount, err := r.executeFile(ctx, filename)
		duration := time.Since(start)

		s.Add(results.NewFileResultBuilder(filename).
			WithRequestCount(requestCount).
			WithDuration(duration).
			WithError(err))

		if err != nil && firstError == nil {
			firstError = err
		}
	}

	s.SetTotalDuration(time.Since(overallStart))
	return s, firstError
}

// executeFile executes a single test file and returns the number of requests made.
func (r *Runner) executeFile(ctx context.Context, filename string) (int, error) {
	file, err := os.Open(filename)
	if err != nil {
		return 0, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	steps, err := parser.Parse(file)
	if err != nil {
		return 0, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	captures := initializeCaptures(r.variables)

	requestCount := 0

	for i, step := range steps {
		select {
		case <-ctx.Done():
			return requestCount, ctx.Err()
		default:
		}

		requestMade, err := r.executeStep(ctx, step, captures)
		if requestMade {
			requestCount++
		}
		if err != nil {
			return requestCount, fmt.Errorf("step %d failed: %w", i, err)
		}
	}

	return requestCount, nil
}
