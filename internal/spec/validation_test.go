package spec

import (
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/extractor"
	"github.com/jacoelho/rq/internal/parser"
)

func mustParseStep(t *testing.T, yamlContent string) parser.Step {
	t.Helper()

	steps, err := parser.Parse(strings.NewReader(yamlContent))
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
		step      parser.Step
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

	step := parser.Step{
		Method: "GET",
		URL:    "https://api.example.com/health",
		Asserts: parser.Asserts{
			Certificate: []parser.CertificateAssert{
				{
					Name: extractor.CertificateFieldSubject,
					Predicate: parser.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: extractor.CertificateFieldIssuer,
					Predicate: parser.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: extractor.CertificateFieldExpireDate,
					Predicate: parser.Predicate{
						Operation: "exists",
					},
				},
				{
					Name: extractor.CertificateFieldSerialNumber,
					Predicate: parser.Predicate{
						Operation: "exists",
					},
				},
			},
		},
		Captures: &parser.Captures{
			Certificate: []parser.CertificateCapture{
				{Name: "cert_subject", CertificateField: extractor.CertificateFieldSubject},
				{Name: "cert_issuer", CertificateField: extractor.CertificateFieldIssuer},
				{Name: "cert_expire", CertificateField: extractor.CertificateFieldExpireDate},
				{Name: "cert_serial", CertificateField: extractor.CertificateFieldSerialNumber},
			},
		},
	}

	if err := ValidateStep(step); err != nil {
		t.Fatalf("ValidateStep() error = %v, want nil", err)
	}
}
