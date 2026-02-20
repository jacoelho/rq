package execute

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/rq/assert"
	"github.com/jacoelho/rq/internal/rq/capture"
	"github.com/jacoelho/rq/internal/rq/model"
	"github.com/jacoelho/rq/internal/rq/predicate"
)

func (r *Runner) executeAssertions(asserts model.Asserts, resp *http.Response, selectors selectorContext) error {
	runner := assertionRunner{
		resp:      resp,
		selectors: selectors,
		evaluator: r.assertionEvaluator(),
	}

	if err := runner.runStatus(asserts.Status); err != nil {
		return err
	}
	if err := runner.runHeaders(asserts.Headers); err != nil {
		return err
	}
	if err := runner.runCertificates(asserts.Certificate); err != nil {
		return err
	}
	if err := runner.runJSONPath(asserts.JSONPath); err != nil {
		return err
	}

	return nil
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

type assertionRunner struct {
	resp      *http.Response
	selectors selectorContext
	evaluator *assert.Evaluator
}

func (r assertionRunner) evaluate(actual any, predicateInput model.Predicate) (bool, error) {
	if r.evaluator == nil {
		return assert.Evaluate(actual, predicateInput)
	}

	return r.evaluator.Evaluate(actual, predicateInput)
}

func (r assertionRunner) runStatus(asserts []model.StatusAssert) error {
	for _, current := range asserts {
		actual, err := capture.ExtractStatusCode(r.resp)
		if err != nil {
			return fmt.Errorf("status extraction failed: %w", err)
		}

		ok, err := r.evaluate(actual, current.Predicate)
		if err != nil {
			return fmt.Errorf("status assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("status assertion failed: expected %s %v, got %v", current.Predicate.Operation, current.Predicate.Value, actual)
		}
	}

	return nil
}

func (r assertionRunner) runHeaders(asserts []model.HeaderAssert) error {
	for _, current := range asserts {
		actual, err := capture.ExtractHeader(r.resp, current.Name)
		if err != nil {
			if capture.IsNotFound(err) {
				actual = ""
			} else {
				return fmt.Errorf("header extraction failed for %s: %w", current.Name, err)
			}
		}

		ok, err := r.evaluate(actual, current.Predicate)
		if err != nil {
			return fmt.Errorf("header assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("header %s assertion failed: expected %s %v, got %v", current.Name, current.Predicate.Operation, current.Predicate.Value, actual)
		}
	}

	return nil
}

func (r assertionRunner) runCertificates(asserts []model.CertificateAssert) error {
	for _, current := range asserts {
		actual, err := capture.ExtractCertificateField(r.resp, current.Name)
		if err != nil {
			return fmt.Errorf("certificate assertion failed for field %s: %w", current.Name, err)
		}

		ok, err := r.evaluate(actual, current.Predicate)
		if err != nil {
			return fmt.Errorf("certificate assertion error: %w", err)
		}
		if !ok {
			return fmt.Errorf("certificate %s assertion failed: expected %s %v, got %v", current.Name, current.Predicate.Operation, current.Predicate.Value, actual)
		}
	}

	return nil
}

func (r assertionRunner) runJSONPath(asserts []model.JSONPathAssert) error {
	if len(asserts) == 0 {
		return nil
	}
	if r.selectors.err != nil {
		return fmt.Errorf("JSONPath assertion failed for %s: %w", asserts[0].Path, r.selectors.err)
	}

	for _, current := range asserts {
		actual, err := capture.ExtractJSONPathFromData(r.selectors.data, current.Path)
		if err != nil {
			actual, err = resolveJSONPathAssertionValue(current, err)
			if err != nil {
				return err
			}
		}

		ok, err := r.evaluate(actual, current.Predicate)
		if err != nil {
			return fmt.Errorf("JSONPath assertion failed for %s: %w", current.Path, err)
		}
		if !ok {
			return fmt.Errorf("JSONPath assertion failed for %s: expected %s %v, but condition was not met", current.Path, current.Predicate.Operation, current.Predicate.Value)
		}
	}

	return nil
}
