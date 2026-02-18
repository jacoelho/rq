package evaluator

import (
	"testing"

	"github.com/jacoelho/rq/internal/parser"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	predicateInput := parser.Predicate{
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

func TestValidate(t *testing.T) {
	t.Parallel()

	err := Validate(parser.Predicate{
		Operation: "exists",
		Value:     true,
		HasValue:  true,
	})
	if err == nil {
		t.Fatalf("Validate() expected error for exists with value")
	}
}
