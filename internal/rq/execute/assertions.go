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
	return runAssertions(
		asserts,
		func(_ model.StatusAssert) (any, error) {
			return capture.ExtractStatusCode(resp)
		},
		func(_ model.StatusAssert, err error) (any, error) {
			return nil, fmt.Errorf("status extraction failed: %w", err)
		},
		func(current model.StatusAssert) model.Predicate {
			return current.Predicate
		},
		func(_ model.StatusAssert, err error) error {
			return fmt.Errorf("status assertion error: %w", err)
		},
		func(current model.StatusAssert, actual any) error {
			return fmt.Errorf("status assertion failed: expected %s %v, got %v", current.Predicate.Operation, current.Predicate.Value, actual)
		},
	)
}

// executeHeaderAssertions validates header assertions.
func (r *Runner) executeHeaderAssertions(asserts []model.HeaderAssert, resp *http.Response) error {
	return runAssertions(
		asserts,
		func(current model.HeaderAssert) (any, error) {
			return capture.ExtractHeader(resp, current.Name)
		},
		func(current model.HeaderAssert, err error) (any, error) {
			if capture.IsNotFound(err) {
				return "", nil
			}
			return nil, fmt.Errorf("header extraction failed for %s: %w", current.Name, err)
		},
		func(current model.HeaderAssert) model.Predicate {
			return current.Predicate
		},
		func(_ model.HeaderAssert, err error) error {
			return fmt.Errorf("header assertion error: %w", err)
		},
		func(current model.HeaderAssert, actual any) error {
			return fmt.Errorf("header %s assertion failed: expected %s %v, got %v", current.Name, current.Predicate.Operation, current.Predicate.Value, actual)
		},
	)
}

// executeCertificateAssertions validates certificate assertions.
func (r *Runner) executeCertificateAssertions(asserts []model.CertificateAssert, resp *http.Response) error {
	return runAssertions(
		asserts,
		func(current model.CertificateAssert) (any, error) {
			return capture.ExtractCertificateField(resp, current.Name)
		},
		func(current model.CertificateAssert, err error) (any, error) {
			return nil, fmt.Errorf("certificate assertion failed for field %s: %w", current.Name, err)
		},
		func(current model.CertificateAssert) model.Predicate {
			return current.Predicate
		},
		func(_ model.CertificateAssert, err error) error {
			return fmt.Errorf("certificate assertion error: %w", err)
		},
		func(current model.CertificateAssert, actual any) error {
			return fmt.Errorf("certificate %s assertion failed: expected %s %v, got %v", current.Name, current.Predicate.Operation, current.Predicate.Value, actual)
		},
	)
}

// executeJSONPathAssertions validates JSONPath assertions.
func (r *Runner) executeJSONPathAssertions(asserts []model.JSONPathAssert, jsonPathData any, jsonPathErr error) error {
	if len(asserts) == 0 {
		return nil
	}
	if jsonPathErr != nil {
		return fmt.Errorf("JSONPath assertion failed for %s: %w", asserts[0].Path, jsonPathErr)
	}

	return runAssertions(
		asserts,
		func(current model.JSONPathAssert) (any, error) {
			return capture.ExtractJSONPathFromData(jsonPathData, current.Path)
		},
		func(current model.JSONPathAssert, err error) (any, error) {
			return resolveJSONPathAssertionValue(current, err)
		},
		func(current model.JSONPathAssert) model.Predicate {
			return current.Predicate
		},
		func(current model.JSONPathAssert, err error) error {
			return fmt.Errorf("JSONPath assertion failed for %s: %w", current.Path, err)
		},
		func(current model.JSONPathAssert, _ any) error {
			return fmt.Errorf("JSONPath assertion failed for %s: expected %s %v, but condition was not met", current.Path, current.Predicate.Operation, current.Predicate.Value)
		},
	)
}

func runAssertions[T any](
	asserts []T,
	extractValue func(T) (any, error),
	handleExtractionError func(T, error) (any, error),
	assertionPredicate func(T) model.Predicate,
	handleEvaluationError func(T, error) error,
	handleAssertionFailure func(T, any) error,
) error {
	for _, current := range asserts {
		actual, err := extractValue(current)
		if err != nil {
			actual, err = handleExtractionError(current, err)
			if err != nil {
				return err
			}
		}

		ok, err := evaluateAssertion(actual, assertionPredicate(current))
		if err != nil {
			return handleEvaluationError(current, err)
		}
		if !ok {
			return handleAssertionFailure(current, actual)
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
