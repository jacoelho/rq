package assert

import (
	"testing"

	"github.com/jacoelho/rq/internal/rq/model"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	predicateInput := model.Predicate{
		Operation: "equals",
		Value:     200,
		HasValue:  true,
	}

	ok, err := Evaluate(200, predicateInput)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !ok {
		t.Fatalf("Evaluate() = false, want true")
	}
}

func TestEvaluateTypeIs(t *testing.T) {
	t.Parallel()

	predicateInput := model.Predicate{
		Operation: "type_is",
		Value:     "array",
		HasValue:  true,
	}

	ok, err := Evaluate([]any{1, 2, 3}, predicateInput)
	if err != nil {
		t.Fatalf("Evaluate() error = %v", err)
	}
	if !ok {
		t.Fatalf("Evaluate() = false, want true")
	}
}

func TestValidate(t *testing.T) {
	t.Parallel()

	err := Validate(model.Predicate{
		Operation: "exists",
		Value:     true,
		HasValue:  true,
	})
	if err == nil {
		t.Fatalf("Validate() expected error for exists with value")
	}
}
