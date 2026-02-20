package execute

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/rq/capture"
	"github.com/jacoelho/rq/internal/rq/model"
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
func (r *Runner) executeCaptures(captures *model.Captures, resp *http.Response, body []byte, captureMap map[string]CaptureValue) error {
	hasJSONPathCaptures := captures != nil && len(captures.JSONPath) > 0
	selectors := selectorContextFromBody(body, hasJSONPathCaptures)
	return r.executeCapturesWithSelectors(captures, resp, body, selectors, captureMap)
}

func (r *Runner) executeCapturesWithSelectors(
	captures *model.Captures,
	resp *http.Response,
	body []byte,
	selectors selectorContext,
	captureMap map[string]CaptureValue,
) error {
	if captures == nil {
		return nil
	}

	runner := captureRunner{
		resp:      resp,
		body:      body,
		selectors: selectors,
		captures:  captureMap,
	}

	if err := runner.runStatus(captures.Status); err != nil {
		return err
	}

	if err := runner.runHeaders(captures.Headers); err != nil {
		return err
	}

	if err := runner.runCertificates(captures.Certificate); err != nil {
		return err
	}

	if err := runner.runJSONPath(captures.JSONPath); err != nil {
		return err
	}

	if err := runner.runRegex(captures.Regex); err != nil {
		return err
	}

	if err := runner.runBody(captures.Body); err != nil {
		return err
	}

	return nil
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(current model.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	value, err := extractRegexCaptureValue(current, body)
	if err != nil {
		return err
	}

	captureMap[current.Name] = CaptureValue{Value: value, Redact: current.Redact}
	return nil
}

func extractRegexCaptureValue(current model.RegexCapture, body []byte) (any, error) {
	value, err := capture.ExtractRegex(body, current.Pattern, current.Group)
	if err != nil {
		if capture.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("regex capture failed for %s: %w", current.Name, err)
	}

	return value, nil
}

type captureRunner struct {
	resp      *http.Response
	body      []byte
	selectors selectorContext
	captures  map[string]CaptureValue
}

func (r captureRunner) set(name string, value any, redact bool) {
	r.captures[name] = CaptureValue{Value: value, Redact: redact}
}

func (r captureRunner) runStatus(captures []model.StatusCapture) error {
	for _, current := range captures {
		value, err := capture.ExtractStatusCode(r.resp)
		if err != nil {
			return fmt.Errorf("status capture failed for %s: %w", current.Name, err)
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}

func (r captureRunner) runHeaders(captures []model.HeaderCapture) error {
	for _, current := range captures {
		value, err := capture.ExtractHeader(r.resp, current.HeaderName)
		if err != nil {
			if capture.IsNotFound(err) {
				value = ""
			} else {
				return fmt.Errorf("header capture failed for %s: %w", current.Name, err)
			}
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}

func (r captureRunner) runCertificates(captures []model.CertificateCapture) error {
	for _, current := range captures {
		value, err := capture.ExtractCertificateField(r.resp, current.CertificateField)
		if err != nil {
			return fmt.Errorf("certificate capture failed for field %s: %w", current.CertificateField, err)
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}

func (r captureRunner) runJSONPath(captures []model.JSONPathCapture) error {
	if len(captures) == 0 {
		return nil
	}
	if r.selectors.err != nil {
		return fmt.Errorf("JSONPath capture failed for %s: %w", captures[0].Name, r.selectors.err)
	}

	for _, current := range captures {
		value, err := capture.ExtractJSONPathFromData(r.selectors.data, current.Path)
		if err != nil {
			if capture.IsNotFound(err) {
				value = nil
			} else {
				return fmt.Errorf("JSONPath capture failed for %s: %w", current.Name, err)
			}
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}

func (r captureRunner) runRegex(captures []model.RegexCapture) error {
	for _, current := range captures {
		value, err := extractRegexCaptureValue(current, r.body)
		if err != nil {
			return err
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}

func (r captureRunner) runBody(captures []model.BodyCapture) error {
	for _, current := range captures {
		value, err := capture.ExtractBody(r.body)
		if err != nil {
			return fmt.Errorf("body capture failed for %s: %w", current.Name, err)
		}

		r.set(current.Name, value, current.Redact)
	}

	return nil
}
