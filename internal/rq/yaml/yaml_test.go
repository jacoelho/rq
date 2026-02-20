package yaml

import (
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/rq/model"
)

func TestEncodeStep(t *testing.T) {
	t.Parallel()

	step := model.Step{
		Method:   "GET",
		URL:      "https://api.example.com/health",
		When:     "status_code == 200",
		BodyFile: "./payload.bin",
		Asserts: model.Asserts{
			Status: []model.StatusAssert{{
				Predicate: model.Predicate{Operation: "equals", Value: int64(200), HasValue: true},
			}},
		},
	}

	payload, err := EncodeStep(step)
	if err != nil {
		t.Fatalf("EncodeStep() error = %v", err)
	}

	parsed, err := model.Parse(strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("generated YAML failed to parse: %v\n%s", err, string(payload))
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed steps = %d", len(parsed))
	}
	if parsed[0].Method != "GET" {
		t.Fatalf("parsed method = %s", parsed[0].Method)
	}
	if parsed[0].BodyFile != "./payload.bin" {
		t.Fatalf("parsed body_file = %q", parsed[0].BodyFile)
	}
	if parsed[0].When != "status_code == 200" {
		t.Fatalf("parsed when = %q", parsed[0].When)
	}
}

func TestEncodeStepKeepsExplicitNullPredicateValue(t *testing.T) {
	t.Parallel()

	step := model.Step{
		Method: "GET",
		URL:    "https://api.example.com/users",
		Asserts: model.Asserts{
			JSONPath: []model.JSONPathAssert{{
				Path: "$.user.deleted_at",
				Predicate: model.Predicate{
					Operation: "equals",
					HasValue:  true,
					Value:     nil,
				},
			}},
		},
	}

	payload, err := EncodeStep(step)
	if err != nil {
		t.Fatalf("EncodeStep() error = %v", err)
	}
	if !strings.Contains(string(payload), "value: null") {
		t.Fatalf("generated YAML should contain explicit null value, got:\n%s", string(payload))
	}

	parsed, err := model.Parse(strings.NewReader(string(payload)))
	if err != nil {
		t.Fatalf("generated YAML failed to parse: %v\n%s", err, string(payload))
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed steps = %d", len(parsed))
	}
	if len(parsed[0].Asserts.JSONPath) != 1 {
		t.Fatalf("jsonpath asserts = %d", len(parsed[0].Asserts.JSONPath))
	}
	predicate := parsed[0].Asserts.JSONPath[0].Predicate
	if !predicate.HasValue {
		t.Fatal("predicate.HasValue = false, expected true")
	}
	if predicate.Value != nil {
		t.Fatalf("predicate.Value = %#v, expected nil", predicate.Value)
	}
}

func TestEncodeStepWritesOrderedKeyValuesAsSequence(t *testing.T) {
	t.Parallel()

	step := model.Step{
		Method: "GET",
		URL:    "https://api.example.com/search",
		Headers: model.KeyValues{
			{Key: "X-Zeta", Value: "last"},
			{Key: "X-Alpha", Value: "first"},
		},
		Query: model.KeyValues{
			{Key: "q", Value: "rq"},
			{Key: "limit", Value: "10"},
		},
	}

	payload, err := EncodeStep(step)
	if err != nil {
		t.Fatalf("EncodeStep() error = %v", err)
	}

	yamlPayload := string(payload)
	if !strings.Contains(yamlPayload, "- key: X-Zeta") {
		t.Fatalf("expected sequence key-value format for headers, got:\n%s", yamlPayload)
	}
	if !strings.Contains(yamlPayload, "- key: q") {
		t.Fatalf("expected sequence key-value format for query, got:\n%s", yamlPayload)
	}

	parsed, err := model.Parse(strings.NewReader(yamlPayload))
	if err != nil {
		t.Fatalf("generated YAML failed to parse: %v\n%s", err, yamlPayload)
	}
	if len(parsed) != 1 {
		t.Fatalf("parsed steps = %d", len(parsed))
	}
	if len(parsed[0].Headers) != 2 || parsed[0].Headers[0].Key != "X-Zeta" {
		t.Fatalf("parsed headers = %+v", parsed[0].Headers)
	}
	if len(parsed[0].Query) != 2 || parsed[0].Query[0].Key != "q" {
		t.Fatalf("parsed query = %+v", parsed[0].Query)
	}
}
