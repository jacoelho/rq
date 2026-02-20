package execute

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/rq/assert"
	"github.com/jacoelho/rq/internal/rq/capture"
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/predicate"
)

func (r *Runner) executeAssertionsWithJSONPathData(asserts model.Asserts, resp *http.Response, jsonPathData any, jsonPathErr error) error {
	if err := r.executeStatusAssertions(asserts.Status, resp); err != nil {
		return err
	}

	if err := r.executeHeaderAssertions(asserts.Headers, resp); err != nil {
		return err
	}

	if err := r.executeCertificateAssertions(asserts.Certificate, resp); err != nil {
		return err
	}

	if err := r.executeJSONPathAssertions(asserts.JSONPath, jsonPathData, jsonPathErr); err != nil {
		return err
	}

	return nil
}

// executeStatusAssertions validates status code assertions.
func (r *Runner) executeStatusAssertions(asserts []model.StatusAssert, resp *http.Response) error {
	for _, assert := range asserts {
		actual, err := capture.ExtractStatusCode(resp)
		if err != nil {
			return fmt.Errorf("status extraction failed: %w", err)
		}

		ok, err := evaluateAssertion(actual, assert.Predicate)
		if err != nil {
			return fmt.Errorf("status assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("status assertion failed: expected %s %v, got %v", assert.Predicate.Operation, assert.Predicate.Value, actual)
		}
	}

	return nil
}

// executeHeaderAssertions validates header assertions.
func (r *Runner) executeHeaderAssertions(asserts []model.HeaderAssert, resp *http.Response) error {
	for _, assert := range asserts {
		actual, err := capture.ExtractHeader(resp, assert.Name)
		if err != nil {
			if capture.IsNotFound(err) {
				actual = ""
			} else {
				return fmt.Errorf("header extraction failed for %s: %w", assert.Name, err)
			}
		}

		ok, err := evaluateAssertion(actual, assert.Predicate)
		if err != nil {
			return fmt.Errorf("header assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("header %s assertion failed: expected %s %v, got %v", assert.Name, assert.Predicate.Operation, assert.Predicate.Value, actual)
		}
	}

	return nil
}

// executeCertificateAssertions validates certificate assertions.
func (r *Runner) executeCertificateAssertions(asserts []model.CertificateAssert, resp *http.Response) error {
	for _, assert := range asserts {
		actual, err := capture.ExtractCertificateField(resp, assert.Name)
		if err != nil {
			return fmt.Errorf("certificate assertion failed for field %s: %w", assert.Name, err)
		}

		ok, err := evaluateAssertion(actual, assert.Predicate)
		if err != nil {
			return fmt.Errorf("certificate assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("certificate %s assertion failed: expected %s %v, got %v", assert.Name, assert.Predicate.Operation, assert.Predicate.Value, actual)
		}
	}

	return nil
}

// executeJSONPathAssertions validates JSONPath assertions.
func (r *Runner) executeJSONPathAssertions(asserts []model.JSONPathAssert, jsonPathData any, jsonPathErr error) error {
	if len(asserts) == 0 {
		return nil
	}
	if jsonPathErr != nil {
		return fmt.Errorf("JSONPath assertion failed for %s: %w", asserts[0].Path, jsonPathErr)
	}

	for _, assert := range asserts {
		actual, err := capture.ExtractJSONPathFromData(jsonPathData, assert.Path)
		if err != nil {
			actual, err = resolveJSONPathAssertionValue(assert, err)
			if err != nil {
				return err
			}
		}

		ok, err := evaluateAssertion(actual, assert.Predicate)
		if err != nil {
			return fmt.Errorf("JSONPath assertion failed for %s: %w", assert.Path, err)
		}
		if !ok {
			return fmt.Errorf("JSONPath assertion failed for %s: expected %s %v, but condition was not met", assert.Path, assert.Predicate.Operation, assert.Predicate.Value)
		}
	}

	return nil
}

func parseJSONPathData(body []byte, hasSelectors bool) (any, error) {
	if !hasSelectors {
		return nil, nil
	}
	return capture.ParseJSONBody(body)
}

func evaluateAssertion(actual any, predicateInput model.Predicate) (bool, error) {
	return assert.Evaluate(actual, predicateInput)
}

func resolveJSONPathAssertionValue(assert model.JSONPathAssert, err error) (any, error) {
	if !capture.IsNotFound(err) {
		return nil, fmt.Errorf("JSONPath assertion failed for %s: %w", assert.Path, err)
	}

	op, opErr := predicate.ParseOperator(assert.Predicate.Operation)
	if opErr != nil {
		return nil, fmt.Errorf("JSONPath assertion failed for %s: %w", assert.Path, opErr)
	}
	if op != predicate.OpExists {
		return nil, fmt.Errorf("JSONPath assertion failed for %s: selector returned no value", assert.Path)
	}

	return nil, nil
}
