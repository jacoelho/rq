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
	jsonPathData, jsonPathErr := parseJSONPathData(body, hasJSONPathCaptures)
	return r.executeCapturesWithJSONPathData(captures, resp, body, jsonPathData, jsonPathErr, captureMap)
}

func (r *Runner) executeCapturesWithJSONPathData(
	captures *model.Captures,
	resp *http.Response,
	body []byte,
	jsonPathData any,
	jsonPathErr error,
	captureMap map[string]CaptureValue,
) error {
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

	if err := r.executeJSONPathCaptures(captures.JSONPath, jsonPathData, jsonPathErr, captureMap); err != nil {
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

func runCaptures[T any](
	captures []T,
	extractValue func(T) (any, error),
	handleExtractionError func(T, error) (any, error),
	captureMetadata func(T) (string, bool),
	captureMap map[string]CaptureValue,
) error {
	for _, current := range captures {
		value, err := extractValue(current)
		if err != nil {
			value, err = handleExtractionError(current, err)
			if err != nil {
				return err
			}
		}

		name, redact := captureMetadata(current)
		captureMap[name] = CaptureValue{Value: value, Redact: redact}
	}

	return nil
}

// executeStatusCaptures processes status code captures.
func (r *Runner) executeStatusCaptures(captures []model.StatusCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(_ model.StatusCapture) (any, error) {
			return capture.ExtractStatusCode(resp)
		},
		func(current model.StatusCapture, err error) (any, error) {
			return nil, fmt.Errorf("status capture failed for %s: %w", current.Name, err)
		},
		func(current model.StatusCapture) (string, bool) {
			return current.Name, current.Redact
		},
		captureMap,
	)
}

// executeHeaderCaptures processes header captures.
func (r *Runner) executeHeaderCaptures(captures []model.HeaderCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(current model.HeaderCapture) (any, error) {
			return capture.ExtractHeader(resp, current.HeaderName)
		},
		func(current model.HeaderCapture, err error) (any, error) {
			if capture.IsNotFound(err) {
				return "", nil
			}
			return nil, fmt.Errorf("header capture failed for %s: %w", current.Name, err)
		},
		func(current model.HeaderCapture) (string, bool) {
			return current.Name, current.Redact
		},
		captureMap,
	)
}

// executeCertificateCaptures processes certificate captures.
func (r *Runner) executeCertificateCaptures(captures []model.CertificateCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(current model.CertificateCapture) (any, error) {
			return capture.ExtractCertificateField(resp, current.CertificateField)
		},
		func(current model.CertificateCapture, err error) (any, error) {
			return nil, fmt.Errorf("certificate capture failed for field %s: %w", current.CertificateField, err)
		},
		func(current model.CertificateCapture) (string, bool) {
			return current.Name, current.Redact
		},
		captureMap,
	)
}

// executeJSONPathCaptures processes JSONPath captures.
func (r *Runner) executeJSONPathCaptures(captures []model.JSONPathCapture, jsonPathData any, jsonPathErr error, captureMap map[string]CaptureValue) error {
	if len(captures) == 0 {
		return nil
	}
	if jsonPathErr != nil {
		return fmt.Errorf("JSONPath capture failed for %s: %w", captures[0].Name, jsonPathErr)
	}

	return runCaptures(
		captures,
		func(current model.JSONPathCapture) (any, error) {
			return capture.ExtractJSONPathFromData(jsonPathData, current.Path)
		},
		func(current model.JSONPathCapture, err error) (any, error) {
			if capture.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("JSONPath capture failed for %s: %w", current.Name, err)
		},
		func(current model.JSONPathCapture) (string, bool) {
			return current.Name, current.Redact
		},
		captureMap,
	)
}

// executeRegexCaptures processes regex captures.
func (r *Runner) executeRegexCaptures(captures []model.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	for _, current := range captures {
		if err := r.executeRegexCapture(current, body, captureMap); err != nil {
			return err
		}
	}
	return nil
}

// executeBodyCaptures processes body captures.
func (r *Runner) executeBodyCaptures(captures []model.BodyCapture, body []byte, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(_ model.BodyCapture) (any, error) {
			return capture.ExtractBody(body)
		},
		func(current model.BodyCapture, err error) (any, error) {
			return nil, fmt.Errorf("body capture failed for %s: %w", current.Name, err)
		},
		func(current model.BodyCapture) (string, bool) {
			return current.Name, current.Redact
		},
		captureMap,
	)
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(current model.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	value, err := capture.ExtractRegex(body, current.Pattern, current.Group)
	if err != nil {
		if capture.IsNotFound(err) {
			value = nil
		} else {
			return fmt.Errorf("regex capture failed for %s: %w", current.Name, err)
		}
	}

	captureMap[current.Name] = CaptureValue{Value: value, Redact: current.Redact}
	return nil
}
