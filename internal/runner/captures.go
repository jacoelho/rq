package runner

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/extractor"
	"github.com/jacoelho/rq/internal/parser"
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

func runCaptures[T any](
	captures []T,
	extractValue func(T) (any, error),
	handleExtractionError func(T, error) (any, error),
	captureMetadata func(T) (string, bool),
	captureMap map[string]CaptureValue,
) error {
	for _, capture := range captures {
		value, err := extractValue(capture)
		if err != nil {
			value, err = handleExtractionError(capture, err)
			if err != nil {
				return err
			}
		}

		name, redact := captureMetadata(capture)
		captureMap[name] = CaptureValue{Value: value, Redact: redact}
	}

	return nil
}

// executeStatusCaptures processes status code captures.
func (r *Runner) executeStatusCaptures(captures []parser.StatusCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(_ parser.StatusCapture) (any, error) {
			return extractor.ExtractStatusCode(resp)
		},
		func(capture parser.StatusCapture, err error) (any, error) {
			return nil, fmt.Errorf("status capture failed for %s: %w", capture.Name, err)
		},
		func(capture parser.StatusCapture) (string, bool) {
			return capture.Name, capture.Redact
		},
		captureMap,
	)
}

// executeHeaderCaptures processes header captures.
func (r *Runner) executeHeaderCaptures(captures []parser.HeaderCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(capture parser.HeaderCapture) (any, error) {
			return extractor.ExtractHeader(resp, capture.HeaderName)
		},
		func(capture parser.HeaderCapture, err error) (any, error) {
			if extractor.IsNotFound(err) {
				return "", nil
			}
			return nil, fmt.Errorf("header capture failed for %s: %w", capture.Name, err)
		},
		func(capture parser.HeaderCapture) (string, bool) {
			return capture.Name, capture.Redact
		},
		captureMap,
	)
}

// executeCertificateCaptures processes certificate captures.
func (r *Runner) executeCertificateCaptures(captures []parser.CertificateCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(capture parser.CertificateCapture) (any, error) {
			return extractor.ExtractCertificateField(resp, capture.CertificateField)
		},
		func(capture parser.CertificateCapture, err error) (any, error) {
			return nil, fmt.Errorf("certificate capture failed for field %s: %w", capture.CertificateField, err)
		},
		func(capture parser.CertificateCapture) (string, bool) {
			return capture.Name, capture.Redact
		},
		captureMap,
	)
}

// executeJSONPathCaptures processes JSONPath captures.
func (r *Runner) executeJSONPathCaptures(captures []parser.JSONPathCapture, body []byte, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(capture parser.JSONPathCapture) (any, error) {
			return extractor.ExtractJSONPath(body, capture.Path)
		},
		func(capture parser.JSONPathCapture, err error) (any, error) {
			if extractor.IsNotFound(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("JSONPath capture failed for %s: %w", capture.Name, err)
		},
		func(capture parser.JSONPathCapture) (string, bool) {
			return capture.Name, capture.Redact
		},
		captureMap,
	)
}

// executeRegexCaptures processes regex captures.
func (r *Runner) executeRegexCaptures(captures []parser.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		if err := r.executeRegexCapture(capture, body, captureMap); err != nil {
			return err
		}
	}
	return nil
}

// executeBodyCaptures processes body captures.
func (r *Runner) executeBodyCaptures(captures []parser.BodyCapture, body []byte, captureMap map[string]CaptureValue) error {
	return runCaptures(
		captures,
		func(_ parser.BodyCapture) (any, error) {
			return extractor.ExtractBody(body)
		},
		func(capture parser.BodyCapture, err error) (any, error) {
			return nil, fmt.Errorf("body capture failed for %s: %w", capture.Name, err)
		},
		func(capture parser.BodyCapture) (string, bool) {
			return capture.Name, capture.Redact
		},
		captureMap,
	)
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(capture parser.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	value, err := extractor.ExtractRegex(body, capture.Pattern, capture.Group)
	if err != nil {
		if extractor.IsNotFound(err) {
			value = nil
		} else {
			return fmt.Errorf("regex capture failed for %s: %w", capture.Name, err)
		}
	}

	captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	return nil
}
