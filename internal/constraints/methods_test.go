package constraints

import (
	"testing"

	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/normalize"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/pm/requestmap"
	"github.com/jacoelho/rq/internal/rq/compile"
	"github.com/jacoelho/rq/internal/rq/model"
)

func TestRuntimeAndMigrationShareMethodSet(t *testing.T) {
	t.Parallel()

	for _, method := range model.SupportedMethods() {
		method := method
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			step := model.Step{
				Method: method,
				URL:    "https://api.example.com/health",
			}
			if err := compile.ValidateStep(step); err != nil {
				t.Fatalf("compile.ValidateStep(%q) error = %v", method, err)
			}

			node := normalize.RequestNode{
				Name: "Health",
				Request: ast.Request{
					Method: method,
					URL: ast.URLValue{
						Raw: "https://api.example.com/health",
					},
				},
			}
			result := requestmap.Request(node)
			if !result.Converted {
				t.Fatalf("requestmap.Request(%q) not converted; issues=%+v", method, result.Issues)
			}
		})
	}
}

func TestUnsupportedMethodRejectedAcrossBoundaries(t *testing.T) {
	t.Parallel()

	const method = "TRACE"

	step := model.Step{
		Method: method,
		URL:    "https://api.example.com/health",
	}
	if err := compile.ValidateStep(step); err == nil {
		t.Fatalf("compile.ValidateStep(%q) expected error", method)
	}

	node := normalize.RequestNode{
		Name: "Health",
		Request: ast.Request{
			Method: method,
			URL: ast.URLValue{
				Raw: "https://api.example.com/health",
			},
		},
	}
	result := requestmap.Request(node)
	if result.Converted {
		t.Fatalf("requestmap.Request(%q) expected converted=false", method)
	}
	if len(result.Issues) == 0 {
		t.Fatalf("requestmap.Request(%q) expected at least one issue", method)
	}
	if result.Issues[0].Code != report.CodeInvalidRequestShape {
		t.Fatalf("requestmap.Request(%q) first issue code = %q, want %q", method, result.Issues[0].Code, report.CodeInvalidRequestShape)
	}
}
