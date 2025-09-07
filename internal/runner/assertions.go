package runner

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/evaluator"
	"github.com/jacoelho/rq/internal/extractor"
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
		statusCode, err := extractor.ExtractStatusCode(resp)
		if err != nil {
			return fmt.Errorf("status extraction failed: %w", err)
		}

		op, err := evaluator.ParseOperation(assert.Predicate.Operation)
		if err != nil {
			return fmt.Errorf("invalid status operation: %w", err)
		}

		result, err := evaluator.Evaluate(op, statusCode, assert.Predicate.Value)
		if err != nil {
			return fmt.Errorf("status assertion error: %w", err)
		}
		if !result {
			return fmt.Errorf("status assertion failed: expected %s %v, got %d", assert.Predicate.Operation, assert.Predicate.Value, statusCode)
		}
	}
	return nil
}

// executeHeaderAssertions validates header assertions
func (r *Runner) executeHeaderAssertions(asserts []parser.HeaderAssert, resp *http.Response) error {
	for _, assert := range asserts {
		headerValue, err := extractor.ExtractHeader(resp, assert.Name)
		if err != nil && !extractor.IsNotFound(err) {
			return fmt.Errorf("header extraction failed for %s: %w", assert.Name, err)
		}
		// If header not found, use empty string for assertion
		if extractor.IsNotFound(err) {
			headerValue = ""
		}

		op, err := evaluator.ParseOperation(assert.Predicate.Operation)
		if err != nil {
			return fmt.Errorf("invalid header operation: %w", err)
		}

		result, err := evaluator.Evaluate(op, headerValue, assert.Predicate.Value)
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
	// Extract all certificate fields using the extractor package
	certInfo, err := extractor.ExtractAllCertificateFields(resp)
	if err != nil {
		return fmt.Errorf("certificate assertion failed for field %s: %w", assert.Name, err)
	}

	// Get the specific field value
	var value any
	switch assert.Name {
	case "subject":
		value = certInfo.Subject
	case "issuer":
		value = certInfo.Issuer
	case "expire_date":
		value = certInfo.ExpireDate.Format("2006-01-02T15:04:05Z07:00")
	case "serial_number":
		value = certInfo.SerialNumber
	default:
		return fmt.Errorf("unsupported certificate field: %s (supported: subject, issuer, expire_date, serial_number)", assert.Name)
	}

	op, err := evaluator.ParseOperation(assert.Predicate.Operation)
	if err != nil {
		return fmt.Errorf("invalid certificate operation: %w", err)
	}

	result, err := evaluator.Evaluate(op, value, assert.Predicate.Value)
	if err != nil {
		return fmt.Errorf("certificate assertion error: %w", err)
	}
	if !result {
		return fmt.Errorf("certificate %s assertion failed: expected %s %v, got %v", assert.Name, assert.Predicate.Operation, assert.Predicate.Value, value)
	}

	return nil
}
