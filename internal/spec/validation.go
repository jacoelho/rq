package spec

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/jacoelho/rq/internal/evaluator"
	"github.com/jacoelho/rq/internal/extractor"
	"github.com/jacoelho/rq/internal/parser"
)

var ErrInvalidSpec = errors.New("invalid spec")

var validMethods = []string{
	"GET",
	"POST",
	"PUT",
	"PATCH",
	"DELETE",
	"HEAD",
	"OPTIONS",
}

func ValidateSteps(steps []parser.Step) error {
	for index, step := range steps {
		if err := ValidateStep(step); err != nil {
			return fmt.Errorf("%w: step %d: %w", ErrInvalidSpec, index+1, err)
		}
	}

	return nil
}

func ValidateStep(step parser.Step) error {
	if strings.TrimSpace(step.Method) == "" {
		return errors.New("step method cannot be empty")
	}

	if !slices.Contains(validMethods, step.Method) {
		return fmt.Errorf("unsupported HTTP method: %s", step.Method)
	}

	if strings.TrimSpace(step.URL) == "" {
		return errors.New("step URL cannot be empty")
	}

	if step.Options.Retries < 0 {
		return fmt.Errorf("retries must be >= 0, got: %d", step.Options.Retries)
	}

	if err := validateAsserts(step.Asserts); err != nil {
		return err
	}

	if err := validateCaptures(step.Captures); err != nil {
		return err
	}

	return nil
}

func validateAsserts(asserts parser.Asserts) error {
	for _, assert := range asserts.Status {
		if err := validatePredicate(assert.Predicate, "status assert"); err != nil {
			return err
		}
	}

	for _, assert := range asserts.Headers {
		if err := requireField(assert.Name, "header assert", "name"); err != nil {
			return err
		}
		if err := validatePredicate(assert.Predicate, "header assert"); err != nil {
			return err
		}
	}

	for _, assert := range asserts.Certificate {
		if err := requireField(assert.Name, "certificate assert", "name"); err != nil {
			return err
		}
		if !isSupportedCertificateField(assert.Name) {
			return fmt.Errorf("unsupported certificate field: %s", assert.Name)
		}

		if err := validatePredicate(assert.Predicate, "certificate assert"); err != nil {
			return err
		}
	}

	for _, assert := range asserts.JSONPath {
		if err := requireField(assert.Path, "jsonpath assert", "path"); err != nil {
			return err
		}

		if err := validatePredicate(assert.Predicate, "jsonpath assert"); err != nil {
			return err
		}
	}

	return nil
}

func validateCaptures(captures *parser.Captures) error {
	if captures == nil {
		return nil
	}

	for _, capture := range captures.Status {
		if err := requireField(capture.Name, "status capture", "name"); err != nil {
			return err
		}
	}

	for _, capture := range captures.Headers {
		if err := requireField(capture.Name, "header capture", "name"); err != nil {
			return err
		}
		if err := requireField(capture.HeaderName, "header capture", "header_name"); err != nil {
			return err
		}
	}

	for _, capture := range captures.Certificate {
		if err := requireField(capture.Name, "certificate capture", "name"); err != nil {
			return err
		}
		if err := requireField(capture.CertificateField, "certificate capture", "certificate_field"); err != nil {
			return err
		}
		if !isSupportedCertificateField(capture.CertificateField) {
			return fmt.Errorf("unsupported certificate field: %s", capture.CertificateField)
		}
	}

	for _, capture := range captures.JSONPath {
		if err := requireField(capture.Name, "jsonpath capture", "name"); err != nil {
			return err
		}
		if err := requireField(capture.Path, "jsonpath capture", "path"); err != nil {
			return err
		}
	}

	for _, capture := range captures.Regex {
		if err := requireField(capture.Name, "regex capture", "name"); err != nil {
			return err
		}
		if err := requireField(capture.Pattern, "regex capture", "pattern"); err != nil {
			return err
		}
		if capture.Group < 0 {
			return fmt.Errorf("regex capture %q has negative group: %d", capture.Name, capture.Group)
		}
	}

	for _, capture := range captures.Body {
		if err := requireField(capture.Name, "body capture", "name"); err != nil {
			return err
		}
	}

	return nil
}

func validatePredicate(p parser.Predicate, location string) error {
	if err := evaluator.Validate(p); err != nil {
		return fmt.Errorf("%s is invalid: %w", location, err)
	}

	return nil
}

func requireField(value string, location string, fieldName string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s missing required '%s' field", location, fieldName)
	}

	return nil
}

func isSupportedCertificateField(field string) bool {
	switch field {
	case extractor.CertificateFieldSubject:
		return true
	case extractor.CertificateFieldIssuer:
		return true
	case extractor.CertificateFieldExpireDate:
		return true
	case extractor.CertificateFieldSerialNumber:
		return true
	default:
		return false
	}
}
