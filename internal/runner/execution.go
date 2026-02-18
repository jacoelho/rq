package runner

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jacoelho/rq/internal/parser"
	"github.com/jacoelho/rq/internal/template"
)

// executeStep executes a single HTTP request step with retry logic.
func (r *Runner) executeStep(ctx context.Context, step parser.Step, captures map[string]CaptureValue) (bool, error) {
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
			r.logf("Retry attempt %d of %d\n", attempt-1, step.Options.Retries)
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
	req, err := prepareRequest(ctx, step, captures)
	if err != nil {
		return false, err
	}

	staticSecrets := r.staticSecrets()
	valuesToRedact := redactValues(captures, staticSecrets)
	if r.config != nil && r.config.Debug {
		r.debugRequest(req, valuesToRedact)
	}

	resp, respBody, err := r.executeRequest(ctx, step.Options, req)
	if err != nil {
		return true, err
	}

	if err := r.processStepResponse(step, resp, respBody, captures); err != nil {
		return true, err
	}

	if r.config != nil && r.config.Debug {
		valuesToRedact = redactValues(captures, staticSecrets)
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

func prepareRequest(ctx context.Context, step parser.Step, captures map[string]CaptureValue) (*http.Request, error) {
	tmplVars := captureMapForTemplate(captures)

	requestURL, err := template.Apply(step.URL, tmplVars)
	if err != nil {
		return nil, fmt.Errorf("failed to process URL template: %w", err)
	}

	if len(step.Query) > 0 {
		requestURL, err = processQueryParameters(requestURL, step.Query, tmplVars)
		if err != nil {
			return nil, fmt.Errorf("failed to process query parameters: %w", err)
		}
	}

	body, err := template.Apply(step.Body, tmplVars)
	if err != nil {
		return nil, fmt.Errorf("failed to process body template: %w", err)
	}

	req, err := newHTTPRequest(ctx, step.Method, requestURL, body)
	if err != nil {
		return nil, err
	}

	if err := applyTemplatedHeaders(req, step.Headers, tmplVars); err != nil {
		return nil, err
	}

	return req, nil
}

func newHTTPRequest(ctx context.Context, method string, requestURL string, body string) (*http.Request, error) {
	var bodyReader io.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, requestURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	return req, nil
}

func applyTemplatedHeaders(req *http.Request, headers map[string]string, templateVars map[string]any) error {
	for name, value := range headers {
		processedValue, err := template.Apply(value, templateVars)
		if err != nil {
			return fmt.Errorf("failed to process header %s: %w", name, err)
		}
		req.Header.Set(name, processedValue)
	}

	return nil
}

func (r *Runner) executeRequest(ctx context.Context, options parser.Options, req *http.Request) (*http.Response, []byte, error) {
	if err := r.rateLimiter.Wait(ctx); err != nil {
		return nil, nil, fmt.Errorf("rate limiting interrupted: %w", err)
	}

	resp, err := r.getClient(options).Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return resp, respBody, nil
}

func (r *Runner) processStepResponse(step parser.Step, resp *http.Response, respBody []byte, captures map[string]CaptureValue) error {
	if err := r.executeAssertions(step.Asserts, resp, respBody); err != nil {
		return fmt.Errorf("assertion failed: %w", err)
	}

	if err := r.executeCaptures(step.Captures, resp, respBody, captures); err != nil {
		return fmt.Errorf("capture failed: %w", err)
	}

	return nil
}

func (r *Runner) staticSecrets() map[string]any {
	if r.config == nil {
		return nil
	}
	return r.config.Secrets
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
