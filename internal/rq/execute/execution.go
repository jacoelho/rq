package execute

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jacoelho/rq/internal/pathing"
	"github.com/jacoelho/rq/internal/rq/capture"
	"github.com/jacoelho/rq/internal/rq/expr"
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/templating"
)

// executeStep executes a single HTTP request step with retry logic.
func (r *Runner) executeStep(ctx context.Context, step model.Step, captures map[string]CaptureValue, stepBaseDir string) (bool, error) {
	shouldExecute, err := evaluateStepCondition(step, captures)
	if err != nil {
		return false, err
	}
	if !shouldExecute {
		if r.config != nil && r.config.Debug {
			r.logf("Skipping step: when condition evaluated to false (%s)\n", step.When)
		}
		return false, nil
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
			r.logf("Retry attempt %d of %d\n", attempt-1, step.Options.Retries)
		}

		attemptRequestMade, err := r.executeStepAttempt(ctx, step, captures, stepBaseDir)
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
func (r *Runner) executeStepAttempt(ctx context.Context, step model.Step, captures map[string]CaptureValue, stepBaseDir string) (bool, error) {
	req, err := prepareRequest(ctx, step, captures, stepBaseDir)
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
func (r *Runner) getClient(options model.Options) *http.Client {
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

func evaluateStepCondition(step model.Step, captures map[string]CaptureValue) (bool, error) {
	when := strings.TrimSpace(step.When)
	if when == "" {
		return true, nil
	}

	variables := captureMapForTemplate(captures)
	matched, err := expr.Eval(when, variables)
	if err != nil {
		return false, fmt.Errorf("failed to evaluate step when condition: %w", err)
	}

	return matched, nil
}

func prepareRequest(ctx context.Context, step model.Step, captures map[string]CaptureValue, stepBaseDir string) (*http.Request, error) {
	tmplVars := captureMapForTemplate(captures)

	requestURL, err := templating.Apply(step.URL, tmplVars)
	if err != nil {
		return nil, fmt.Errorf("failed to process URL template: %w", err)
	}

	if len(step.Query) > 0 {
		requestURL, err = processQueryParameters(requestURL, step.Query, tmplVars)
		if err != nil {
			return nil, fmt.Errorf("failed to process query parameters: %w", err)
		}
	}

	body, err := resolveRequestBodyWithBaseDir(step, tmplVars, stepBaseDir)
	if err != nil {
		return nil, err
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

func resolveRequestBody(step model.Step, templateVars map[string]any) (string, error) {
	return resolveRequestBodyWithBaseDir(step, templateVars, "")
}

func resolveRequestBodyWithBaseDir(step model.Step, templateVars map[string]any, baseDir string) (string, error) {
	body, err := templating.Apply(step.Body, templateVars)
	if err != nil {
		return "", fmt.Errorf("failed to process body template: %w", err)
	}

	if step.BodyFile == "" {
		return body, nil
	}

	filePath, err := templating.Apply(step.BodyFile, templateVars)
	if err != nil {
		return "", fmt.Errorf("failed to process body_file template: %w", err)
	}
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return body, nil
	}
	filePath = pathing.ResolveBodyFilePath(filePath, baseDir)

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read body_file %s: %w", filePath, err)
	}

	return string(content), nil
}

func applyTemplatedHeaders(req *http.Request, headers model.KeyValues, templateVars map[string]any) error {
	for _, header := range headers {
		name := strings.TrimSpace(header.Key)
		if name == "" {
			continue
		}

		value := header.Value
		processedValue, err := templating.Apply(value, templateVars)
		if err != nil {
			return fmt.Errorf("failed to process header %s: %w", name, err)
		}
		req.Header.Add(name, processedValue)
	}

	return nil
}

func (r *Runner) executeRequest(ctx context.Context, options model.Options, req *http.Request) (*http.Response, []byte, error) {
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

func (r *Runner) processStepResponse(step model.Step, resp *http.Response, respBody []byte, captures map[string]CaptureValue) error {
	hasJSONPathSelectors := len(step.Asserts.JSONPath) > 0
	if step.Captures != nil && len(step.Captures.JSONPath) > 0 {
		hasJSONPathSelectors = true
	}

	var (
		jsonPathData any
		jsonPathErr  error
	)
	if hasJSONPathSelectors {
		jsonPathData, jsonPathErr = capture.ParseJSONBody(respBody)
	}

	if err := r.executeAssertionsWithJSONPathData(step.Asserts, resp, jsonPathData, jsonPathErr); err != nil {
		return fmt.Errorf("assertion failed: %w", err)
	}

	if err := r.executeCapturesWithJSONPathData(step.Captures, resp, respBody, jsonPathData, jsonPathErr, captures); err != nil {
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
func processQueryParameters(requestURL string, queryParams model.KeyValues, captures map[string]any) (string, error) {
	if len(queryParams) == 0 {
		return requestURL, nil
	}

	parsedURL, err := url.Parse(requestURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	orderedQuery := make([]string, 0, len(queryParams))
	if parsedURL.RawQuery != "" {
		orderedQuery = append(orderedQuery, strings.Split(parsedURL.RawQuery, "&")...)
	}

	for _, queryParam := range queryParams {
		name := strings.TrimSpace(queryParam.Key)
		if name == "" {
			continue
		}

		value := queryParam.Value
		processedValue, err := templating.Apply(value, captures)
		if err != nil {
			return "", fmt.Errorf("failed to process query parameter %s: %w", name, err)
		}
		orderedQuery = append(orderedQuery, url.QueryEscape(name)+"="+url.QueryEscape(processedValue))
	}

	parsedURL.RawQuery = strings.Join(orderedQuery, "&")
	return parsedURL.String(), nil
}
