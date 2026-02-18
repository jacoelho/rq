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
	"github.com/jacoelho/rq/internal/results"
	"github.com/jacoelho/rq/internal/spec"
	"golang.org/x/time/rate"
)

type Runner struct {
	client      *http.Client
	variables   map[string]any
	config      *config.Config
	rateLimiter *rate.Limiter
	output      io.Writer
	errOutput   io.Writer
}

func New(cfg *config.Config) (*Runner, *exit.Result) {
	client, err := cfg.HTTPClient()
	if err != nil {
		return nil, exit.Errorf("Error creating runner: %v\n", err)
	}

	return &Runner{
		client:      client,
		variables:   cfg.AllVariables(),
		config:      cfg,
		rateLimiter: newRateLimiter(cfg.RateLimit),
		output:      os.Stdout,
		errOutput:   os.Stderr,
	}, nil
}

func newRateLimiter(requestsPerSecond float64) *rate.Limiter {
	if requestsPerSecond <= 0 {
		return rate.NewLimiter(rate.Inf, 1)
	}

	return rate.NewLimiter(rate.Limit(requestsPerSecond), 1)
}

func (r *Runner) SetOutput(w io.Writer) {
	r.output = w
}

func (r *Runner) SetErrorOutput(w io.Writer) {
	r.errOutput = w
}

func (r *Runner) payloadWriter() io.Writer {
	if r.output == nil {
		return io.Discard
	}
	return r.output
}

func (r *Runner) errorWriter() io.Writer {
	if r.errOutput == nil {
		return io.Discard
	}
	return r.errOutput
}

func (r *Runner) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(r.errorWriter(), format, args...)
}

func (r *Runner) Run(ctx context.Context) int {
	if r.config.Repeat < 0 {
		return r.runInfiniteLoop(ctx)
	}
	return r.runFiniteLoop(ctx)
}

func (r *Runner) runInfiniteLoop(ctx context.Context) int {
	iteration := 1

	for {
		select {
		case <-ctx.Done():
			r.logf("\nInterrupted after %d iterations\n", iteration-1)
			return 1
		default:
		}

		if r.config.Debug {
			r.logf("--- Iteration %d ---\n", iteration)
		}

		result, err := r.runOnce(ctx)
		if err != nil {
			r.logf("\nError in iteration %d: %v\n", iteration, err)
			return 1
		}

		if result != nil {
			if err := result.Format(r.config.OutputFormat, r.payloadWriter()); err != nil {
				r.logf("Error formatting results: %v\n", err)
			}
		}

		iteration++
	}
}

func (r *Runner) runFiniteLoop(ctx context.Context) int {
	var allResults []*results.Summary
	totalIterations := r.config.Repeat + 1

	for i := 1; i <= totalIterations; i++ {
		select {
		case <-ctx.Done():
			r.logf("\nInterrupted after %d of %d iterations\n", i-1, totalIterations)
			return 1
		default:
		}

		if r.config.Debug && totalIterations > 1 {
			r.logf("--- Iteration %d of %d ---\n", i, totalIterations)
		}

		result, err := r.runOnce(ctx)
		if err != nil {
			r.logf("\nError in iteration %d: %v\n", i, err)
			return 1
		}

		if result != nil {
			allResults = append(allResults, result)
		}
	}

	if err := results.FormatAggregated(r.config.OutputFormat, r.payloadWriter(), allResults); err != nil {
		r.logf("Error formatting results: %v\n", err)
	}
	return 0
}

func (r *Runner) runOnce(ctx context.Context) (*results.Summary, error) {
	return r.ExecuteFiles(ctx, r.config.TestFiles)
}

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

		s.Add(results.FileResult{
			Filename:     filename,
			RequestCount: requestCount,
			Duration:     duration,
			Error:        err,
		})

		if err != nil && firstError == nil {
			firstError = err
		}
	}

	s.SetTotalDuration(time.Since(overallStart))
	return s, firstError
}

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
	if err := spec.ValidateSteps(steps); err != nil {
		return 0, fmt.Errorf("failed to validate file %s: %w", filename, err)
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
