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

// executeStatusCaptures processes status code captures.
func (r *Runner) executeStatusCaptures(captures []parser.StatusCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		statusCode, err := extractor.ExtractStatusCode(resp)
		if err != nil {
			return fmt.Errorf("status capture failed for %s: %w", capture.Name, err)
		}
		captureMap[capture.Name] = CaptureValue{Value: statusCode, Redact: capture.Redact}
	}
	return nil
}

// executeHeaderCaptures processes header captures.
func (r *Runner) executeHeaderCaptures(captures []parser.HeaderCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		headerValue, err := extractor.ExtractHeader(resp, capture.HeaderName)
		if err != nil && !extractor.IsNotFound(err) {
			return fmt.Errorf("header capture failed for %s: %w", capture.Name, err)
		}
		// If header not found, set empty string (existing behavior)
		if extractor.IsNotFound(err) {
			headerValue = ""
		}
		captureMap[capture.Name] = CaptureValue{Value: headerValue, Redact: capture.Redact}
	}
	return nil
}

// executeCertificateCaptures processes certificate captures.
func (r *Runner) executeCertificateCaptures(captures []parser.CertificateCapture, resp *http.Response, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		certInfo, err := extractor.ExtractAllCertificateFields(resp)
		if err != nil {
			return fmt.Errorf("certificate capture failed for field %s: %w", capture.CertificateField, err)
		}

		var value any
		switch capture.CertificateField {
		case "subject":
			value = certInfo.Subject
		case "issuer":
			value = certInfo.Issuer
		case "expire_date":
			value = certInfo.ExpireDate.Format("2006-01-02T15:04:05Z07:00")
		case "serial_number":
			value = certInfo.SerialNumber
		default:
			return fmt.Errorf("unsupported certificate field: %s (supported: subject, issuer, expire_date, serial_number)", capture.CertificateField)
		}

		captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	}
	return nil
}

// executeJSONPathCaptures processes JSONPath captures.
func (r *Runner) executeJSONPathCaptures(captures []parser.JSONPathCapture, body []byte, captureMap map[string]CaptureValue) error {
	for _, capture := range captures {
		value, err := extractor.ExtractJSONPath(body, capture.Path)
		if err != nil && !extractor.IsNotFound(err) {
			return fmt.Errorf("JSONPath capture failed for %s: %w", capture.Name, err)
		}
		// If path not found, value will be nil (existing behavior)
		if extractor.IsNotFound(err) {
			value = nil
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
		bodyStr, err := extractor.ExtractBody(body)
		if err != nil {
			return fmt.Errorf("body capture failed for %s: %w", capture.Name, err)
		}
		captureMap[capture.Name] = CaptureValue{Value: bodyStr, Redact: capture.Redact}
	}
	return nil
}

// executeRegexCapture handles regex-based captures.
func (r *Runner) executeRegexCapture(capture parser.RegexCapture, body []byte, captureMap map[string]CaptureValue) error {
	value, err := extractor.ExtractRegex(body, capture.Pattern, capture.Group)
	if err != nil && !extractor.IsNotFound(err) {
		return fmt.Errorf("regex capture failed for %s: %w", capture.Name, err)
	}
	// If no match found, value will be nil (existing behavior)
	if extractor.IsNotFound(err) {
		value = nil
	}
	captureMap[capture.Name] = CaptureValue{Value: value, Redact: capture.Redact}
	return nil
}
