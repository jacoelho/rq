package model

import (
	"strings"
	"testing"
)

func TestParseKeyValuesFromMappingPreservesOrder(t *testing.T) {
	t.Parallel()

	steps, err := Parse(strings.NewReader(`
- method: GET
  url: https://api.example.com/health
  headers:
    X-Zeta: last
    X-Alpha: first
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(steps))
	}

	headers := steps[0].Headers
	if len(headers) != 2 {
		t.Fatalf("headers len = %d, want 2", len(headers))
	}
	if headers[0].Key != "X-Zeta" || headers[0].Value != "last" {
		t.Fatalf("headers[0] = %+v", headers[0])
	}
	if headers[1].Key != "X-Alpha" || headers[1].Value != "first" {
		t.Fatalf("headers[1] = %+v", headers[1])
	}
}

func TestParseKeyValuesFromSequence(t *testing.T) {
	t.Parallel()

	steps, err := Parse(strings.NewReader(`
- method: GET
  url: https://api.example.com/search
  query:
    - key: limit
      value: 10
    - key: enabled
      value: true
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(steps))
	}

	query := steps[0].Query
	if len(query) != 2 {
		t.Fatalf("query len = %d, want 2", len(query))
	}
	if query[0].Key != "limit" || query[0].Value != "10" {
		t.Fatalf("query[0] = %+v", query[0])
	}
	if query[1].Key != "enabled" || query[1].Value != "true" {
		t.Fatalf("query[1] = %+v", query[1])
	}
}

func TestParseKeyValuesSequenceRejectsMissingKey(t *testing.T) {
	t.Parallel()

	_, err := Parse(strings.NewReader(`
- method: GET
  url: https://api.example.com/search
  query:
    - value: 10
`))
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
}

func TestParseKeyValuesPreservesLargeUnsignedScalarValues(t *testing.T) {
	t.Parallel()

	steps, err := Parse(strings.NewReader(`
- method: GET
  url: https://api.example.com/resource
  headers:
    X-Token: 18446744073709551615
  query:
    - key: id
      value: 18446744073709551615
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	if len(steps) != 1 {
		t.Fatalf("steps len = %d, want 1", len(steps))
	}

	headers := steps[0].Headers
	if len(headers) != 1 {
		t.Fatalf("headers len = %d, want 1", len(headers))
	}
	if headers[0].Key != "X-Token" || headers[0].Value != "18446744073709551615" {
		t.Fatalf("headers[0] = %+v", headers[0])
	}

	query := steps[0].Query
	if len(query) != 1 {
		t.Fatalf("query len = %d, want 1", len(query))
	}
	if query[0].Key != "id" || query[0].Value != "18446744073709551615" {
		t.Fatalf("query[0] = %+v", query[0])
	}
}
