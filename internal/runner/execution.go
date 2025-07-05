package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/jacoelho/rq/internal/parser"
	"github.com/jacoelho/rq/internal/template"
)

// validateStep validates the step configuration before execution.
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
func (r *Runner) executeStep(ctx context.Context, step parser.Step, captures map[string]CaptureValue) (bool, error) {
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
func (r *Runner) executeStepAttempt(ctx context.Context, step parser.Step, captures map[string]CaptureValue) (bool, error) {
	tmplVars := captureMapForTemplate(captures)

	requestURL, err := template.Apply(step.URL, tmplVars)
	if err != nil {
		return false, fmt.Errorf("failed to process URL template: %w", err)
	}

	if len(step.Query) > 0 {
		requestURL, err = processQueryParameters(requestURL, step.Query, tmplVars)
		if err != nil {
			return false, fmt.Errorf("failed to process query parameters: %w", err)
		}
	}

	body, err := template.Apply(step.Body, tmplVars)
	if err != nil {
		return false, fmt.Errorf("failed to process body template: %w", err)
	}

	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, step.Method, requestURL, bodyReader)
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}

	for name, value := range step.Headers {
		processedValue, err := template.Apply(value, tmplVars)
		if err != nil {
			return false, fmt.Errorf("failed to process header %s: %w", name, err)
		}
		req.Header.Set(name, processedValue)
	}

	var staticSecrets map[string]any
	if r.config != nil {
		staticSecrets = r.config.Secrets
	}
	valuesToRedact := redactValues(captures, staticSecrets)

	if r.config != nil && r.config.Debug {
		r.debugRequest(req, valuesToRedact)
	}

	if err := r.rateLimiter.Wait(ctx); err != nil {
		return false, fmt.Errorf("rate limiting interrupted: %w", err)
	}

	client := r.getClient(step.Options)

	resp, err := client.Do(req)
	if err != nil {
		return true, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, fmt.Errorf("failed to read response body: %w", err)
	}

	if err := r.executeAssertions(step.Asserts, resp, respBody); err != nil {
		return true, fmt.Errorf("assertion failed: %w", err)
	}

	if err := r.executeCaptures(step.Captures, resp, respBody, captures); err != nil {
		return true, fmt.Errorf("capture failed: %w", err)
	}

	valuesToRedact = redactValues(captures, staticSecrets)

	if r.config != nil && r.config.Debug {
		r.debugResponse(resp, respBody, valuesToRedact)
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

// captureMapForTemplate converts capture map to map[string]any for template expansion
func captureMapForTemplate(captures map[string]CaptureValue) map[string]any {
	m := make(map[string]any, len(captures))
	for k, v := range captures {
		m[k] = v.Value
	}
	return m
}

// processQueryParameters processes query parameters from a step and appends them to the given URL.
func processQueryParameters(requestURL string, queryParams map[string]string, captures map[string]any) (string, error) {
	if len(queryParams) == 0 {
		return requestURL, nil
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	query := parsedURL.Query()

	for name, value := range queryParams {
		processedValue, err := template.Apply(value, captures)
		if err != nil {
			return "", fmt.Errorf("failed to process query parameter %s: %w", name, err)
		}
		query.Set(name, processedValue)
	}

	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}
