package runner

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/jacoelho/rq/internal/parser"
	"github.com/theory/jsonpath"
)

// CaptureValue represents a captured value with redaction flag.
type CaptureValue struct {
	Value  any
	Redact bool
}

// initializeCaptures creates a capture map from variables.
func initializeCaptures(vars map[string]any) map[string]CaptureValue {
	captures := make(map[string]CaptureValue, len(vars))
	for k, v := range vars {
		captures[k] = CaptureValue{Value: v, Redact: false}
	}
	return captures
}

// executeCaptures extracts values from the response using different capture types.
func (r *Runner) executeCaptures(captures *parser.Captures, resp *http.Response, body []byte, captureMap map[string]CaptureValue) error {
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
func (r *Runner) executeStatusCaptures(captures []parser.StatusCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		captureMap[capture.Name] = CaptureValue{Value: resp.StatusCode, Redact: capture.Redact}
	}
	return nil
}

// executeHeaderCaptures processes header captures.
func (r *Runner) executeHeaderCaptures(captures []parser.HeaderCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		headerValue := resp.Header.Get(capture.HeaderName)
		captureMap[capture.Name] = CaptureValue{Value: headerValue, Redact: capture.Redact}
	}
	return nil
}

// executeCertificateCaptures processes certificate captures.
func (r *Runner) executeCertificateCaptures(captures []parser.CertificateCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		value, err := r.extractCertificateField(capture.CertificateField, resp)
		if err != nil {
			return fmt.Errorf("certificate capture failed for field %s: %w", capture.CertificateField, err)
		}
		captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	}
	return nil
}

// executeJSONPathCaptures processes JSONPath captures.
func (r *Runner) executeJSONPathCaptures(captures []parser.JSONPathCapture, body []byte, captureMap map[string]CaptureValue) error {
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
			value = results[0]
		}

		captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	}
	return nil
}

// executeRegexCaptures processes regex captures.
func (r *Runner) executeRegexCaptures(captures []parser.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		if err := r.executeRegexCapture(capture, body, captureMap); err != nil {
			return fmt.Errorf("regex capture failed for %s: %w", capture.Name, err)
		}
	}
	return nil
}

// executeBodyCaptures processes body captures.
func (r *Runner) executeBodyCaptures(captures []parser.BodyCapture, body []byte, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		captureMap[capture.Name] = CaptureValue{Value: string(body), Redact: capture.Redact}
	}
	return nil
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(capture parser.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	re, err := regexp.Compile(capture.Pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern %s: %w", capture.Pattern, err)
	}

	matches := re.FindSubmatch(body)
	if matches == nil {
		captureMap[capture.Name] = CaptureValue{Value: nil, Redact: capture.Redact}
		return nil
	}

	if capture.Group < 0 || capture.Group >= len(matches) {
		return fmt.Errorf("invalid capture group %d for pattern %s (found %d groups)", capture.Group, capture.Pattern, len(matches)-1)
	}

	value := string(matches[capture.Group])
	captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	return nil
}
