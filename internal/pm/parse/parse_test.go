package parse

import (
	"testing"

	"github.com/jacoelho/rq/internal/pm/lex"
)

func TestScript(t *testing.T) {
	t.Parallel()

	program := Script([]lex.Token{
		{Text: "  ", Line: 1},
		{Text: `if (json.status === "ok") {`, Line: 2},
		{Text: `} else if (json.status === "ko") {`, Line: 3},
		{Text: "} else {", Line: 4},
		{Text: "}", Line: 5},
		{Text: "pm.test('x', function () {", Line: 6},
		{Text: "x = 1;", Line: 3},
		{Text: "y = 2;", Line: 4},
	})

	if len(program.Statements) != 8 {
		t.Fatalf("len(program.Statements) = %d, want 8", len(program.Statements))
	}

	if program.Statements[0].Kind != StatementEmpty {
		t.Fatalf("program.Statements[0] = %+v", program.Statements[0])
	}
	if program.Statements[1].Kind != StatementControlIf || program.Statements[1].Condition != `json.status === "ok"` {
		t.Fatalf("program.Statements[1] = %+v", program.Statements[1])
	}
	if program.Statements[2].Kind != StatementControlElseIf || program.Statements[2].Condition != `json.status === "ko"` {
		t.Fatalf("program.Statements[2] = %+v", program.Statements[2])
	}
	if program.Statements[3].Kind != StatementControlElse {
		t.Fatalf("program.Statements[3] = %+v", program.Statements[3])
	}
	if program.Statements[4].Kind != StatementControlClose {
		t.Fatalf("program.Statements[4] = %+v", program.Statements[4])
	}
	if program.Statements[5].Kind != StatementStructural {
		t.Fatalf("program.Statements[5] = %+v", program.Statements[5])
	}
	if program.Statements[6].Kind != StatementCode || program.Statements[6].Text != "x = 1;" {
		t.Fatalf("program.Statements[6] = %+v", program.Statements[6])
	}
	if program.Statements[7].Kind != StatementCode || program.Statements[7].Text != "y = 2;" {
		t.Fatalf("program.Statements[7] = %+v", program.Statements[7])
	}
}
