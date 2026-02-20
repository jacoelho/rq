package execute

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jacoelho/rq/internal/rq/compile"
	"github.com/jacoelho/rq/internal/rq/config"
	"github.com/jacoelho/rq/internal/rq/exit"
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/output"
	"github.com/jacoelho/rq/internal/rq/yaml"
	"golang.org/x/time/rate"
)

type CompiledFile struct {
	Filename string
	BaseDir  string
	Steps    []model.Step
}

type Runner struct {
	client      *http.Client
	variables   map[string]any
	config      *config.Config
	compiled    []CompiledFile
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
	return r.runLoop(
		ctx,
		0,
		func(completed int) string {
			return fmt.Sprintf("Interrupted after %d iterations", completed)
		},
		func(iteration int) string {
			if r.config.Debug {
				return fmt.Sprintf("--- Iteration %d ---", iteration)
			}
			return ""
		},
		func(result *output.Summary) error {
			return result.Format(r.config.OutputFormat, r.payloadWriter())
		},
		nil,
	)
}

func (r *Runner) runFiniteLoop(ctx context.Context) int {
	totalIterations := r.config.Repeat + 1
	allResults := make([]*output.Summary, 0, totalIterations)

	return r.runLoop(
		ctx,
		totalIterations,
		func(completed int) string {
			return fmt.Sprintf("Interrupted after %d of %d iterations", completed, totalIterations)
		},
		func(iteration int) string {
			if r.config.Debug && totalIterations > 1 {
				return fmt.Sprintf("--- Iteration %d of %d ---", iteration, totalIterations)
			}
			return ""
		},
		func(result *output.Summary) error {
			allResults = append(allResults, result)
			return nil
		},
		func() error {
			return output.FormatAggregated(r.config.OutputFormat, r.payloadWriter(), allResults)
		},
	)
}

func (r *Runner) runLoop(
	ctx context.Context,
	totalIterations int,
	interruptMessage func(completed int) string,
	debugHeader func(iteration int) string,
	handleResult func(*output.Summary) error,
	finish func() error,
) int {
	for iteration := 1; totalIterations <= 0 || iteration <= totalIterations; iteration++ {
		select {
		case <-ctx.Done():
			r.logf("\n%s\n", interruptMessage(iteration-1))
			return 1
		default:
		}

		if header := debugHeader(iteration); header != "" {
			r.logf("%s\n", header)
		}

		result, err := r.runOnce(ctx)
		if err != nil {
			r.logf("\nError in iteration %d: %v\n", iteration, err)
			return 1
		}

		if result != nil && handleResult != nil {
			if err := handleResult(result); err != nil {
				r.logf("Error formatting results: %v\n", err)
			}
		}
	}

	if finish != nil {
		if err := finish(); err != nil {
			r.logf("Error formatting results: %v\n", err)
		}
	}

	return 0
}

func (r *Runner) runOnce(ctx context.Context) (*output.Summary, error) {
	if r.compiled == nil {
		compiled, err := compileFiles(r.config.TestFiles)
		if err != nil {
			return nil, err
		}
		r.compiled = compiled
	}

	return r.executeCompiledFiles(ctx, r.compiled)
}

func (r *Runner) ExecuteFiles(ctx context.Context, files []string) (*output.Summary, error) {
	return executeFilesWithSummary(
		ctx,
		files,
		func(filename string) string {
			return filename
		},
		func(ctx context.Context, filename string) (int, error) {
			return r.executeFile(ctx, filename)
		},
	)
}

func (r *Runner) executeFile(ctx context.Context, filename string) (int, error) {
	compiled, err := compileFile(filename)
	if err != nil {
		return 0, err
	}

	return r.executeCompiledFile(ctx, compiled)
}

func (r *Runner) executeCompiledFiles(ctx context.Context, files []CompiledFile) (*output.Summary, error) {
	return executeFilesWithSummary(
		ctx,
		files,
		func(file CompiledFile) string {
			return file.Filename
		},
		func(ctx context.Context, file CompiledFile) (int, error) {
			return r.executeCompiledFile(ctx, file)
		},
	)
}

func executeFilesWithSummary[T any](
	ctx context.Context,
	files []T,
	filename func(T) string,
	execute func(context.Context, T) (int, error),
) (*output.Summary, error) {
	s := output.NewSummary(len(files))

	overallStart := time.Now()
	var firstError error

	for _, file := range files {
		select {
		case <-ctx.Done():
			return s, ctx.Err()
		default:
		}

		start := time.Now()
		requestCount, err := execute(ctx, file)
		duration := time.Since(start)

		s.Add(output.FileResult{
			Filename:     filename(file),
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

func (r *Runner) executeCompiledFile(ctx context.Context, file CompiledFile) (int, error) {
	captures := initializeCaptures(r.variables)

	requestCount := 0

	for i, step := range file.Steps {
		select {
		case <-ctx.Done():
			return requestCount, ctx.Err()
		default:
		}

		requestMade, err := r.executeStep(ctx, step, captures, file.BaseDir)
		if requestMade {
			requestCount++
		}
		if err != nil {
			return requestCount, fmt.Errorf("step %d failed: %w", i, err)
		}
	}

	return requestCount, nil
}

func compileFiles(files []string) ([]CompiledFile, error) {
	compiled := make([]CompiledFile, 0, len(files))
	for _, filename := range files {
		file, err := compileFile(filename)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, file)
	}

	return compiled, nil
}

func compileFile(filename string) (CompiledFile, error) {
	file, err := os.Open(filename)
	if err != nil {
		return CompiledFile{}, fmt.Errorf("failed to open file %s: %w", filename, err)
	}
	defer file.Close()

	steps, err := yaml.Parse(file)
	if err != nil {
		return CompiledFile{}, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}
	if err := compile.ValidateSteps(steps); err != nil {
		return CompiledFile{}, fmt.Errorf("failed to validate file %s: %w", filename, err)
	}

	return CompiledFile{
		Filename: filename,
		BaseDir:  filepath.Dir(filename),
		Steps:    steps,
	}, nil
}
