package execute

import (
	"net/http"
	"testing"

	"github.com/jacoelho/rq/internal/rq/model"
)

func TestExecuteStatusAssertionsFailureMessage(t *testing.T) {
	t.Parallel()

	runner := newDefault()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}

	err := runner.executeStatusAssertions([]model.StatusAssert{
		{
			Predicate: model.Predicate{
				Operation: "equals",
				Value:     201,
			},
		},
	}, resp)
	if err == nil {
		t.Fatal("expected assertion failure error")
	}

	want := "status assertion failed: expected equals 201, got 200"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}

func TestExecuteHeaderAssertionsMissingHeaderUsesEmptyValue(t *testing.T) {
	t.Parallel()

	runner := newDefault()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
	}

	err := runner.executeHeaderAssertions([]model.HeaderAssert{
		{
			Name: "X-Missing",
			Predicate: model.Predicate{
				Operation: "equals",
				Value:     "",
			},
		},
	}, resp)
	if err != nil {
		t.Fatalf("executeHeaderAssertions() error = %v", err)
	}
}

func TestExecuteJSONPathAssertionsMissingPathHandling(t *testing.T) {
	t.Parallel()

	runner := newDefault()
	jsonPathData := map[string]any{
		"name": "alice",
	}

	err := runner.executeJSONPathAssertions([]model.JSONPathAssert{
		{
			Path: "$.missing",
			Predicate: model.Predicate{
				Operation: "exists",
			},
		},
	}, jsonPathData, nil)
	if err == nil {
		t.Fatal("expected exists assertion to fail for missing path")
	}
	existsWant := "JSONPath assertion failed for $.missing: expected exists <nil>, but condition was not met"
	if err.Error() != existsWant {
		t.Fatalf("error = %q, want %q", err.Error(), existsWant)
	}

	err = runner.executeJSONPathAssertions([]model.JSONPathAssert{
		{
			Path: "$.missing",
			Predicate: model.Predicate{
				Operation: "equals",
				Value:     "value",
			},
		},
	}, jsonPathData, nil)
	if err == nil {
		t.Fatal("expected equals assertion to fail for missing path")
	}

	want := "JSONPath assertion failed for $.missing: selector returned no value"
	if err.Error() != want {
		t.Fatalf("error = %q, want %q", err.Error(), want)
	}
}
