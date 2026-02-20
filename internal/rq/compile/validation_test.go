package compile

import (
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/rq/model"
)

func mustParseStep(t *testing.T, yamlContent string) model.Step {
	t.Helper()

	steps, err := model.Parse(strings.NewReader(yamlContent))
	if err != nil {
		t.Fatalf("failed to parse YAML fixture: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("expected one step, got %d", len(steps))
	}
	return steps[0]
}

func TestValidateStep(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		step      model.Step
		wantError bool
	}{
		{
			name: "valid_step",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - op: equals
        value: 200
`),
		},
		{
			name: "exists_with_value_is_invalid",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - op: exists
        value: true
`),
			wantError: true,
		},
		{
			name: "length_without_value_is_invalid",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  asserts:
    status:
      - op: length
`),
			wantError: true,
		},
		{
			name: "missing_certificate_assert_name",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  asserts:
    certificate:
      - op: equals
        value: "CN=example.com"
`),
			wantError: true,
		},
		{
			name: "invalid_certificate_field",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  captures:
    certificate:
      - name: cert_info
        certificate_field: unknown_field
`),
			wantError: true,
		},
		{
			name: "invalid_method",
			step: mustParseStep(t, `
- method: TRACE
  url: https://api.example.com/health
`),
			wantError: true,
		},
		{
			name: "body_and_body_file_together_is_invalid",
			step: mustParseStep(t, `
- method: POST
  url: https://api.example.com/upload
  body: "inline"
  body_file: ./payload.bin
`),
			wantError: true,
		},
		{
			name: "valid_when_condition",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  when: status_code == 200 && is_ready
`),
		},
		{
			name: "invalid_when_condition",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  when: status_code ==
`),
			wantError: true,
		},
		{
			name: "non_boolean_when_condition",
			step: mustParseStep(t, `
- method: GET
  url: https://api.example.com/health
  when: 1
`),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateStep(tt.step)
			if (err != nil) != tt.wantError {
				t.Fatalf("ValidateStep() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestValidateStep_AcceptsExtractorCertificateConstants(t *testing.T) {
	t.Parallel()

	step := model.Step{
		Method: "GET",
		URL:    "https://api.example.com/health",
		Asserts: model.Asserts{
			Certificate: []model.CertificateAssert{
				{
					Name: model.CertificateFieldSubject,
					Predicate: model.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: model.CertificateFieldIssuer,
					Predicate: model.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: model.CertificateFieldExpireDate,
					Predicate: model.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: model.CertificateFieldSerialNumber,
					Predicate: model.Predicate{
						Operation: "exists",
					},
				},
			},
		},
		Captures: &model.Captures{
			Certificate: []model.CertificateCapture{
				{Name: "cert_subject", CertificateField: model.CertificateFieldSubject},
				{Name: "cert_issuer", CertificateField: model.CertificateFieldIssuer},
				{Name: "cert_expire", CertificateField: model.CertificateFieldExpireDate},
				{Name: "cert_serial", CertificateField: model.CertificateFieldSerialNumber},
			},
		},
	}

	if err := ValidateStep(step); err != nil {
		t.Fatalf("ValidateStep() error = %v, want nil", err)
	}
}
