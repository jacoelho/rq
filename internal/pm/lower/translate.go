package lower

import (
	"regexp"
	"strings"

	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/lex"
	"github.com/jacoelho/rq/internal/pm/parse"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/rq/model"
)

var (
	statusDirectAssertionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^pm\.expect\(\s*pm\.response\.code\s*\)\.to\.(?:eql|equal)\(\s*(\d{3})\s*\)\s*;?$`),
		regexp.MustCompile(`^pm\.response\.to\.have\.status\(\s*(\d{3})\s*\)\s*;?$`),
	}

	statusTestExpressionPatterns = []*regexp.Regexp{
		regexp.MustCompile(`^responseCode\.code\s*={2,3}\s*(\d{3})\s*;?$`),
		regexp.MustCompile(`^pm\.response\.code\s*={2,3}\s*(\d{3})\s*;?$`),
	}

	hasPattern = regexp.MustCompile(`^_\.\s*has\(\s*([^,]+?)\s*,\s*['"]([^'"]+)['"]\s*\)\s*;?$`)

	testExpressionPattern = regexp.MustCompile(`^tests\[[^\]]+\]\s*=\s*(.+?)\s*;?$`)

	jsonComparisonPattern = regexp.MustCompile(`^(json(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[^\]]+\])*)\s*(===|!==)\s*(.+?)\s*;?$`)

	arrayIsArrayPattern = regexp.MustCompile(`^Array\.isArray\(\s*(json(?:\.[A-Za-z_][A-Za-z0-9_]*|\[[^\]]+\])*)\s*\)$`)

	setEnvironmentPattern = regexp.MustCompile(`^(?:postman\.setEnvironmentVariable|pm\.environment\.set)\(\s*['"]([^'"]+)['"]\s*,\s*(.+?)\s*\)\s*;?$`)

	headerCapturePattern = regexp.MustCompile(`^responseHeaders\[['"]([^'"]+)['"]\]$`)
	pmHeaderCaptureRegex = regexp.MustCompile(`^pm\.response\.headers\.get\(\s*['"]([^'"]+)['"]\s*\)$`)
)

// Result contains the translated rq assertions/captures and diagnostics.
type Result struct {
	Asserts       model.Asserts
	Captures      *model.Captures
	Issues        []report.Issue
	MappedLines   int
	IgnoredLines  int
	UnmappedLines int
}

type conditionFrame struct {
	supported bool
}

type conditionalGuard struct {
	path         string
	op           string
	value        any
	hasValue     bool
	requiresJSON bool
}

type hasPathSegment struct {
	key     string
	index   string
	isIndex bool
}

// Translate maps source test scripts into rq assertions/captures.
func Translate(events []ast.Event) Result {
	result := Result{}

	unmappedCounts := make(map[report.IssueCode]int)
	unmappedFirstLine := make(map[report.IssueCode]int)
	recordUnmapped := func(code report.IssueCode, line int) {
		unmappedCounts[code]++
		result.UnmappedLines++
		if line > 0 {
			if _, exists := unmappedFirstLine[code]; !exists {
				unmappedFirstLine[code] = line
			}
		}
	}
	statusSeen := make(map[int]struct{})
	assertSeen := make(map[string]struct{})

	captured := model.Captures{}
	jsonParseIntent := false
	jsonSemanticsEnforced := false
	conditionStack := make([]conditionFrame, 0)

	for _, event := range events {
		if strings.ToLower(strings.TrimSpace(event.Listen)) != "test" {
			continue
		}

		program := parse.Script(lex.Script(event.Script.Exec))
		for _, statement := range program.Statements {
			switch statement.Kind {
			case parse.StatementEmpty:
				result.IgnoredLines++
				continue
			case parse.StatementControlClose:
				if len(conditionStack) > 0 {
					conditionStack = conditionStack[:len(conditionStack)-1]
				}
				result.IgnoredLines++
				continue
			case parse.StatementControlElse, parse.StatementControlElseIf:
				if len(conditionStack) > 0 {
					conditionStack = conditionStack[:len(conditionStack)-1]
				}
				conditionStack = append(conditionStack, conditionFrame{supported: false})
				recordUnmapped(report.CodeScriptExpressionNotSupported, statement.Line)
				continue
			case parse.StatementControlIf:
				if hasUnsupportedCondition(conditionStack) {
					conditionStack = append(conditionStack, conditionFrame{supported: false})
					recordUnmapped(report.CodeScriptExpressionNotSupported, statement.Line)
					continue
				}

				guard, supported := parseConditionalExpression(statement.Condition)
				conditionStack = append(conditionStack, conditionFrame{supported: supported})
				if !supported {
					recordUnmapped(report.CodeScriptExpressionNotSupported, statement.Line)
					continue
				}

				addJSONPathAssert(&result.Asserts, assertSeen, guard.path, guard.op, guard.value, guard.hasValue)
				if guard.requiresJSON {
					jsonSemanticsEnforced = true
				}
				result.MappedLines++
				continue
			case parse.StatementStructural:
				result.IgnoredLines++
				continue
			}

			line := strings.TrimSpace(statement.Text)
			if hasUnsupportedCondition(conditionStack) {
				recordUnmapped(report.CodeScriptExpressionNotSupported, statement.Line)
				continue
			}

			if code, ok := extractStatusAssertionCode(line); ok {
				addStatusAssert(&result.Asserts, statusSeen, code)
				result.MappedLines++
				continue
			}

			if isJSONParseLine(line) || isJSONValidityLine(line) {
				jsonParseIntent = true
				result.MappedLines++
				continue
			}

			if mapped, needsJSON := mapHasAssertion(&result.Asserts, assertSeen, line); mapped {
				if needsJSON {
					jsonSemanticsEnforced = true
				}
				result.MappedLines++
				continue
			}

			if mapped, needsJSON := mapJSONComparison(&result.Asserts, assertSeen, line); mapped {
				if needsJSON {
					jsonSemanticsEnforced = true
				}
				result.MappedLines++
				continue
			}

			if mapped, needsJSON := mapArrayTypeAssertion(&result.Asserts, assertSeen, line); mapped {
				if needsJSON {
					jsonSemanticsEnforced = true
				}
				result.MappedLines++
				continue
			}

			captureResult := mapEnvironmentCapture(&captured, line)
			if captureResult.mapped {
				if captureResult.requiresJSON {
					jsonSemanticsEnforced = true
				}
				result.MappedLines++
				continue
			}
			if captureResult.issueCode != "" {
				recordUnmapped(captureResult.issueCode, statement.Line)
				continue
			}

			recordUnmapped(report.CodeScriptLineUnmapped, statement.Line)
		}
	}

	if jsonParseIntent && !jsonSemanticsEnforced {
		recordUnmapped(report.CodeScriptExpressionNotSupported, 0)
	}

	if hasAnyCaptures(captured) {
		result.Captures = &captured
	}

	result.Issues = append(result.Issues, buildUnmappedIssues(unmappedCounts, unmappedFirstLine, result.UnmappedLines)...)
	return result
}
