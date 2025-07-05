package runner

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/evaluator"
	"github.com/jacoelho/rq/internal/parser"
)

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
	}
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

	cert := resp.TLS.PeerCertificates[0]

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
