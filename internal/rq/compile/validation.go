package compile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/jacoelho/rq/internal/rq/assert"
	"github.com/jacoelho/rq/internal/rq/expr"
	"github.com/jacoelho/rq/internal/rq/model"
)

var ErrInvalidSpec = errors.New("invalid spec")

func ValidateSteps(steps []model.Step) error {
	for index, step := range steps {
		if err := ValidateStep(step); err != nil {
			return fmt.Errorf("%w: step %d: %w", ErrInvalidSpec, index+1, err)
		}
	}

	return nil
}

func ValidateStep(step model.Step) error {
	if strings.TrimSpace(step.Method) == "" {
		return errors.New("step method cannot be empty")
	}

	if !model.IsSupportedMethod(step.Method) {
		return fmt.Errorf("unsupported HTTP method: %s", step.Method)
	}

	if strings.TrimSpace(step.URL) == "" {
		return errors.New("step URL cannot be empty")
	}

	if strings.TrimSpace(step.When) != "" {
		if err := expr.ValidateBoolean(step.When); err != nil {
			return fmt.Errorf("step when is invalid: %w", err)
		}
	}

	if strings.TrimSpace(step.Body) != "" && strings.TrimSpace(step.BodyFile) != "" {
		return errors.New("step cannot define both body and body_file")
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

func validateAsserts(asserts model.Asserts) error {
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

func validateCaptures(captures *model.Captures) error {
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

func validatePredicate(p model.Predicate, location string) error {
	if err := assert.Validate(p); err != nil {
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
	case model.CertificateFieldSubject:
		return true
	case model.CertificateFieldIssuer:
		return true
	case model.CertificateFieldExpireDate:
		return true
	case model.CertificateFieldSerialNumber:
		return true
	default:
		return false
	}
}
