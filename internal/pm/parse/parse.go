package parse

import (
	"regexp"
	"strings"

	"github.com/jacoelho/rq/internal/pm/lex"
)

var (
	ifConditionPattern     = regexp.MustCompile(`^if\s*\(\s*(.+?)\s*\)\s*\{\s*$`)
	elseIfConditionPattern = regexp.MustCompile(`^(?:}\s*)?else\s+if\s*\(\s*(.+?)\s*\)\s*\{\s*$`)
	elsePattern            = regexp.MustCompile(`^(?:}\s*)?else\s*\{\s*$`)
)

// StatementKind classifies parsed script statements.
type StatementKind string

const (
	StatementCode          StatementKind = "code"
	StatementEmpty         StatementKind = "empty"
	StatementStructural    StatementKind = "structural"
	StatementControlIf     StatementKind = "control_if"
	StatementControlElseIf StatementKind = "control_else_if"
	StatementControlElse   StatementKind = "control_else"
	StatementControlClose  StatementKind = "control_close"
)

// Statement is a parsed script statement with source metadata.
type Statement struct {
	Text      string
	Line      int
	Kind      StatementKind
	Condition string
}

// Program is the parsed script program.
type Program struct {
	Statements []Statement
}

// Script builds a flat statement program from lexer tokens.
func Script(tokens []lex.Token) Program {
	if len(tokens) == 0 {
		return Program{}
	}

	statements := make([]Statement, 0, len(tokens))
	for _, token := range tokens {
		kind, condition := classifyLine(token.Text)
		statements = append(statements, Statement{
			Text:      token.Text,
			Line:      token.Line,
			Kind:      kind,
			Condition: condition,
		})
	}

	return Program{Statements: statements}
}

func classifyLine(raw string) (StatementKind, string) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return StatementEmpty, ""
	}

	if trimmed == "}" {
		return StatementControlClose, ""
	}

	if matches := elseIfConditionPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return StatementControlElseIf, strings.TrimSpace(matches[1])
	}

	if elsePattern.MatchString(trimmed) {
		return StatementControlElse, ""
	}

	if matches := ifConditionPattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return StatementControlIf, strings.TrimSpace(matches[1])
	}

	if isStructural(trimmed) {
		return StatementStructural, ""
	}

	return StatementCode, ""
}

func isStructural(trimmed string) bool {
	switch trimmed {
	case "{", "}", "try {", "try{", "catch (e) {", "catch(e) {", "catch (e){", "catch(e){", "});", "})":
		return true
	}

	if strings.HasPrefix(trimmed, "pm.test(") && strings.Contains(trimmed, "{") {
		return true
	}

	if strings.Contains(trimmed, "tests['error: ") && strings.Contains(trimmed, "= false") {
		return true
	}

	return false
}
