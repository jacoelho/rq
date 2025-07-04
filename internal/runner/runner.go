package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httputil"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jacoelho/rq/internal/config"
	"github.com/jacoelho/rq/internal/evaluator"
	"github.com/jacoelho/rq/internal/exit"
	"github.com/jacoelho/rq/internal/formatter"
	"github.com/jacoelho/rq/internal/formatter/stdout"
	"github.com/jacoelho/rq/internal/parser"
	"github.com/jacoelho/rq/internal/ratelimit"
	"github.com/jacoelho/rq/internal/results"
	"github.com/jacoelho/rq/internal/template"
	"github.com/theory/jsonpath"
)

// Runner executes HTTP test workflows.
type Runner struct {
	client      *http.Client
	variables   map[string]any
	config      *config.Config
	rateLimiter *ratelimit.Limiter
	formatter   formatter.Formatter
}

// New creates a new Runner with the provided configuration.
// If creation fails, returns nil runner and exit result.
func New(cfg *config.Config) (*Runner, *exit.Result) {
	client, err := cfg.HTTPClient()
	if err != nil {
		return nil, exit.Errorf("Error creating runner: %v\n", err)
	}

	rateLimiter := ratelimit.New(cfg.RateLimit)

	formatter := stdout.New()

	return &Runner{
		client:      client,
		variables:   cfg.AllVariables(),
		config:      cfg,
		rateLimiter: rateLimiter,
		formatter:   formatter,
	}, nil
}

// NewDefault creates a new Runner with default configuration.
func NewDefault() *Runner {
	return &Runner{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		variables:   make(map[string]any),
		rateLimiter: ratelimit.New(0), // No rate limiting by default
	}
}

// Run executes the test files according to the configuration
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
			if err := r.formatter.Format(result); err != nil {
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

	if err := r.formatter.Format(allResults...); err != nil {
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

	// start with configured variables and add runtime captures
	captures := make(map[string]any)
	maps.Copy(captures, r.variables)

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

// validateStep validates the step configuration before execution
func (r *Runner) validateStep(step parser.Step) error {
	if step.Method == "" {
		return fmt.Errorf("step method cannot be empty")
	}

	validMethods := []string{"GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS"}
	isValidMethod := slices.Contains(validMethods, step.Method)
	if !isValidMethod {
		return fmt.Errorf("unsupported HTTP method: %s", step.Method)
	}

	if step.URL == "" {
		return fmt.Errorf("step URL cannot be empty")
	}

	if step.Options.Retries < 0 {
		return fmt.Errorf("retries must be >= 0, got: %d", step.Options.Retries)
	}

	return nil
}

// executeStep executes a single HTTP request step with retry logic.
func (r *Runner) executeStep(ctx context.Context, step parser.Step, captures map[string]any) (bool, error) {
	if err := r.validateStep(step); err != nil {
		return false, fmt.Errorf("invalid step configuration: %w", err)
	}

	maxAttempts := max(step.Options.Retries+1, 1)

	var lastErr error
	requestMade := false

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		select {
		case <-ctx.Done():
			return requestMade, ctx.Err()
		default:
		}

		if r.config != nil && r.config.Debug && attempt > 1 {
			fmt.Printf("Retry attempt %d of %d\n", attempt-1, step.Options.Retries)
		}

		attemptRequestMade, err := r.executeStepAttempt(ctx, step, captures)
		if attemptRequestMade {
			requestMade = true
		}

		if err != nil && !attemptRequestMade {
			return requestMade, err
		}

		if attempt == maxAttempts || err == nil {
			return requestMade, err
		}

		lastErr = err
	}

	return requestMade, lastErr
}

// executeStepAttempt executes a single attempt of an HTTP request step.
func (r *Runner) executeStepAttempt(ctx context.Context, step parser.Step, captures map[string]any) (bool, error) {
	url, err := template.Apply(step.URL, captures)
	if err != nil {
		return false, fmt.Errorf("failed to process URL template: %w", err)
	}

	body, err := template.Apply(step.Body, captures)
	if err != nil {
		return false, fmt.Errorf("failed to process body template: %w", err)
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, step.Method, url, bodyReader)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	for name, value := range step.Headers {
		processedValue, err := template.Apply(value, captures)
		if err != nil {
			return false, fmt.Errorf("failed to process header %s: %w", name, err)
		}
		req.Header.Set(name, processedValue)
	}

	if r.config != nil && r.config.Debug {
		r.debugRequest(req)
	}

	if err := r.rateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("rate limiting interrupted: %w", err)
	}

	client := r.getClient(step.Options)

	resp, err := client.Do(req)
	if err != nil {
		return true, fmt.Errorf("request failed: %w", err) // Request was attempted, so count it
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, fmt.Errorf("failed to read response body: %w", err)
	}

	if r.config != nil && r.config.Debug {
		r.debugResponse(resp, respBody)
	}

	if err := r.executeAssertions(step.Asserts, resp, respBody); err != nil {
		return true, fmt.Errorf("assertion failed: %w", err)
	}

	if err := r.executeCaptures(step.Captures, resp, respBody, captures); err != nil {
		return true, fmt.Errorf("capture failed: %w", err)
	}

	return true, nil
}

// getClient returns an HTTP client configured for the specific options' redirect settings.
func (r *Runner) getClient(options parser.Options) *http.Client {
	if options.FollowRedirect == nil || *options.FollowRedirect {
		return r.client
	}

	clientCopy := *r.client
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &clientCopy
}

// debugRequest outputs detailed request information when debug mode is enabled.
func (r *Runner) debugRequest(req *http.Request) {
	fmt.Println("========================================")
	fmt.Println("REQUEST:")
	fmt.Println("========================================")

	reqDump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		fmt.Printf("Error dumping request: %v\n", err)
		return
	}

	fmt.Print(string(reqDump))
	fmt.Println()
}

// debugResponse outputs detailed response information when debug mode is enabled.
func (r *Runner) debugResponse(resp *http.Response, body []byte) {
	fmt.Println("========================================")
	fmt.Println("RESPONSE:")
	fmt.Println("========================================")

	resp.Body = io.NopCloser(bytes.NewReader(body))

	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		fmt.Printf("Error dumping response: %v\n", err)
		return
	}

	fmt.Print(string(respDump))
	fmt.Println("========================================")
	fmt.Println()
}

// executeAssertions validates all assertions against the HTTP response.
func (r *Runner) executeAssertions(asserts parser.Asserts, resp *http.Response, body []byte) error {
	if err := r.executeStatusAssertions(asserts.Status, resp); err != nil {
		return err
	}

	if err := r.executeHeaderAssertions(asserts.Headers, resp); err != nil {
		return err
	}

	if err := r.executeCertificateAssertions(asserts.Certificate, resp); err != nil {
		return err
	}

	if err := r.executeJSONPathAssertions(asserts.JSONPath, body); err != nil {
		return err
	}

	// XPath assertions not yet implemented
	for _, assert := range asserts.XPath {
		return fmt.Errorf("XPath assertions not yet implemented: %s", assert.Path)
	}

	return nil
}

// executeStatusAssertions validates status code assertions
func (r *Runner) executeStatusAssertions(asserts []parser.StatusAssert, resp *http.Response) error {
	for _, assert := range asserts {
		pred, err := evaluator.NewPredicate(assert.Operation, assert.Value)
		if err != nil {
			return fmt.Errorf("invalid status predicate: %w", err)
		}

		result, err := evaluator.EvaluatePredicate(pred, resp.StatusCode)
		if err != nil {
			return fmt.Errorf("status assertion error: %w", err)
		}
		if !result {
			return fmt.Errorf("status assertion failed: expected %s %v, got %d", assert.Operation, assert.Value, resp.StatusCode)
		}
	}
	return nil
}

// executeHeaderAssertions validates header assertions
func (r *Runner) executeHeaderAssertions(asserts []parser.HeaderAssert, resp *http.Response) error {
	for _, assert := range asserts {
		headerValue := resp.Header.Get(assert.Name)
		pred, err := evaluator.NewPredicate(assert.Predicate.Operation, assert.Predicate.Value)
		if err != nil {
			return fmt.Errorf("invalid header predicate: %w", err)
		}

		result, err := evaluator.EvaluatePredicate(pred, headerValue)
		if err != nil {
			return fmt.Errorf("header assertion error: %w", err)
		}
		if !result {
			return fmt.Errorf("header %s assertion failed: expected %s %v, got %s", assert.Name, assert.Predicate.Operation, assert.Predicate.Value, headerValue)
		}
	}
	return nil
}

// executeCertificateAssertions validates certificate assertions
func (r *Runner) executeCertificateAssertions(asserts []parser.CertificateAssert, resp *http.Response) error {
	for _, assert := range asserts {
		if err := r.executeCertificateAssertion(assert, resp); err != nil {
			return err
		}
	}
	return nil
}

// executeJSONPathAssertions validates JSONPath assertions
func (r *Runner) executeJSONPathAssertions(asserts []parser.JSONPathAssert, body []byte) error {
	for _, assert := range asserts {
		if err := r.executeJSONPathAssertion(assert, body); err != nil {
			return err
		}
	}
	return nil
}

// executeJSONPathAssertion executes a single JSONPath assertion.
func (r *Runner) executeJSONPathAssertion(assert parser.JSONPathAssert, body []byte) error {
	// Use the new evaluator JSONPath integration
	result, err := evaluator.EvaluateJSONPathParserPredicate(body, assert.Path, &parser.Predicate{
		Operation: assert.Predicate.Operation,
		Value:     assert.Predicate.Value,
	})

	if err != nil {
		return fmt.Errorf("JSONPath assertion failed for %s: %w", assert.Path, err)
	}

	if !result {
		return fmt.Errorf("JSONPath assertion failed for %s: expected %s %v, but condition was not met", assert.Path, assert.Predicate.Operation, assert.Predicate.Value)
	}

	return nil
}

// executeCaptures extracts values from the response using different capture types.
func (r *Runner) executeCaptures(captures *parser.Captures, resp *http.Response, body []byte, captureMap map[string]any) error {
	if captures == nil {
		return nil
	}

	if err := r.executeStatusCaptures(captures.Status, resp, captureMap); err != nil {
		return err
	}

	if err := r.executeHeaderCaptures(captures.Headers, resp, captureMap); err != nil {
		return err
	}

	if err := r.executeCertificateCaptures(captures.Certificate, resp, captureMap); err != nil {
		return err
	}

	if err := r.executeJSONPathCaptures(captures.JSONPath, body, captureMap); err != nil {
		return err
	}

	if err := r.executeRegexCaptures(captures.Regex, body, captureMap); err != nil {
		return err
	}

	if err := r.executeBodyCaptures(captures.Body, body, captureMap); err != nil {
		return err
	}

	return nil
}

// executeStatusCaptures processes status code captures.
func (r *Runner) executeStatusCaptures(captures []parser.StatusCapture, resp *http.Response, captureMap map[string]any) error {
	for _, capture := range captures {
		captureMap[capture.Name] = resp.StatusCode
	}
	return nil
}

// executeHeaderCaptures processes header captures.
func (r *Runner) executeHeaderCaptures(captures []parser.HeaderCapture, resp *http.Response, captureMap map[string]any) error {
	for _, capture := range captures {
		headerValue := resp.Header.Get(capture.HeaderName)
		captureMap[capture.Name] = headerValue
	}
	return nil
}

// executeCertificateCaptures processes certificate captures.
func (r *Runner) executeCertificateCaptures(captures []parser.CertificateCapture, resp *http.Response, captureMap map[string]any) error {
	for _, capture := range captures {
		value, err := r.extractCertificateField(capture.CertificateField, resp)
		if err != nil {
			return fmt.Errorf("certificate capture failed for field %s: %w", capture.CertificateField, err)
		}
		captureMap[capture.Name] = value
	}
	return nil
}

// executeJSONPathCaptures processes JSONPath captures.
func (r *Runner) executeJSONPathCaptures(captures []parser.JSONPathCapture, body []byte, captureMap map[string]any) error {
	for _, capture := range captures {
		path, err := jsonpath.Parse(capture.Path)
		if err != nil {
			return fmt.Errorf("invalid capture JSONPath %s: %w", capture.Path, err)
		}

		var data any
		if err := json.Unmarshal(body, &data); err != nil {
			return fmt.Errorf("failed to parse JSON data for capture %s: %w", capture.Path, err)
		}

		results := path.Select(data)

		var value any
		if len(results) > 0 {
			value = results[0] // Take the first result
		}

		captureMap[capture.Name] = value
	}
	return nil
}

// executeRegexCaptures processes regex captures.
func (r *Runner) executeRegexCaptures(captures []parser.RegexCapture, body []byte, captureMap map[string]any) error {
	for _, capture := range captures {
		if err := r.executeRegexCapture(capture, body, captureMap); err != nil {
			return fmt.Errorf("regex capture failed for %s: %w", capture.Name, err)
		}
	}
	return nil
}

// executeBodyCaptures processes body captures.
func (r *Runner) executeBodyCaptures(captures []parser.BodyCapture, body []byte, captureMap map[string]any) error {
	for _, capture := range captures {
		captureMap[capture.Name] = string(body)
	}
	return nil
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(capture parser.RegexCapture, body []byte, captureMap map[string]any) error {
	re, err := regexp.Compile(capture.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %s: %w", capture.Pattern, err)
	}

	matches := re.FindSubmatch(body)
	if matches == nil {
		captureMap[capture.Name] = nil
		return nil
	}

	// Validate group index
	if capture.Group < 0 || capture.Group >= len(matches) {
		return fmt.Errorf("invalid capture group %d for pattern %s (found %d groups)", capture.Group, capture.Pattern, len(matches)-1)
	}

	// Store the matched group (0 = full match, 1+ = capture groups)
	captureMap[capture.Name] = string(matches[capture.Group])
	return nil
}

// executeCertificateAssertion handles certificate assertions.
func (r *Runner) executeCertificateAssertion(assert parser.CertificateAssert, resp *http.Response) error {
	value, err := r.extractCertificateField(assert.Name, resp)
	if err != nil {
		return fmt.Errorf("certificate assertion failed for field %s: %w", assert.Name, err)
	}

	pred, err := evaluator.NewPredicate(assert.Predicate.Operation, assert.Predicate.Value)
	if err != nil {
		return fmt.Errorf("invalid certificate predicate: %w", err)
	}

	result, err := evaluator.EvaluatePredicate(pred, value)
	if err != nil {
		return fmt.Errorf("certificate assertion error: %w", err)
	}
	if !result {
		return fmt.Errorf("certificate %s assertion failed: expected %s %v, got %v", assert.Name, assert.Predicate.Operation, assert.Predicate.Value, value)
	}

	return nil
}

// extractCertificateField extracts SSL certificate information from the response.
func (r *Runner) extractCertificateField(field string, resp *http.Response) (any, error) {
	if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no TLS certificate available")
	}

	cert := resp.TLS.PeerCertificates[0] // Use the first certificate (server certificate)

	switch field {
	case "subject":
		return cert.Subject.String(), nil
	case "issuer":
		return cert.Issuer.String(), nil
	case "expire_date":
		return cert.NotAfter.Format("2006-01-02T15:04:05Z07:00"), nil
	case "serial_number":
		return cert.SerialNumber.String(), nil
	default:
		return nil, fmt.Errorf("unsupported certificate field: %s (supported: subject, issuer, expire_date, serial_number)", field)
	}
}
