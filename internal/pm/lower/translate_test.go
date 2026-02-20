package lower

import (
	"testing"

	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/diagnostics"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/rq/model"
)

func TestTranslateMapsCommonPatterns(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["response code is 200"] = responseCode.code === 200;`,
			`var json = JSON.parse(responseBody);`,
			`tests['id is present'] = _.has(json.data, 'id');`,
			`tests['token type is bearer'] = json.token_type === 'Bearer';`,
			`tests['data is list'] = Array.isArray(json.data);`,
			`postman.setEnvironmentVariable("payment_id", json.data.id);`,
			`postman.setEnvironmentVariable("status_code", responseCode.code);`,
		}},
	}}

	result := Translate(events)

	if result.UnmappedLines != 0 {
		t.Fatalf("UnmappedLines = %d, expected 0", result.UnmappedLines)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("Issues = %+v, expected none", result.Issues)
	}
	if len(result.Asserts.Status) != 1 {
		t.Fatalf("status asserts = %d", len(result.Asserts.Status))
	}
	if !hasJSONPathAssert(result.Asserts.JSONPath, "$.data.id", "exists") {
		t.Fatal("missing json field exists assertion")
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.token_type", "equals", "Bearer") {
		t.Fatal("missing json equality assertion")
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.data", "type_is", "array") {
		t.Fatal("missing array type assertion")
	}

	if result.Captures == nil {
		t.Fatal("expected captures")
	}
	if len(result.Captures.JSONPath) != 1 {
		t.Fatalf("jsonpath captures = %d", len(result.Captures.JSONPath))
	}
	if result.Captures.JSONPath[0].Name != "payment_id" || result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected jsonpath capture: %+v", result.Captures.JSONPath[0])
	}
	if len(result.Captures.Status) != 1 || result.Captures.Status[0].Name != "status_code" {
		t.Fatalf("unexpected status captures: %+v", result.Captures.Status)
	}
}

func TestTranslateParseOnlyJSONIntentIsReportedAsUnsupported(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`var json = JSON.parse(responseBody);`,
		}},
	}}

	result := Translate(events)
	if result.UnmappedLines != 1 {
		t.Fatalf("UnmappedLines = %d, expected 1", result.UnmappedLines)
	}
	if hasJSONPathAssert(result.Asserts.JSONPath, "$", "exists") {
		t.Fatalf("unexpected synthetic root exists assertion: %+v", result.Asserts.JSONPath)
	}
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("missing unsupported expression issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateReportsLineSpanForUnsupportedLines(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["unsupported"] = pm.response.json().ok === true;`,
		}},
	}}

	result := Translate(events)
	issue := findIssue(result.Issues, report.CodeScriptLineUnmapped)
	if issue == nil {
		t.Fatalf("missing unmapped line issue: %+v", result.Issues)
	}
	if issue.Stage != diagnostics.StageLower {
		t.Fatalf("issue stage = %q, want %q", issue.Stage, diagnostics.StageLower)
	}
	if issue.Severity != diagnostics.SeverityError {
		t.Fatalf("issue severity = %q, want %q", issue.Severity, diagnostics.SeverityError)
	}
	if issue.Span == nil || issue.Span.Line != 1 {
		t.Fatalf("issue span = %+v, want line 1", issue.Span)
	}
}

func TestTranslateConditionalMapsSimpleGuard(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (json.data.attributes.status === 'delivery_confirmed') {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`}`,
		}},
	}}

	result := Translate(events)
	if result.UnmappedLines != 0 {
		t.Fatalf("expected no unmapped lines, got %d", result.UnmappedLines)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.data.attributes.status", "equals", "delivery_confirmed") {
		t.Fatalf("missing guard assertion: %+v", result.Asserts.JSONPath)
	}
	if result.Captures == nil || len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one capture, got %+v", result.Captures)
	}
	if result.Captures.JSONPath[0].Name != "x" || result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected capture %+v", result.Captures.JSONPath[0])
	}
}

func TestTranslateConditionalElseBranchDoesNotLeakGuard(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (json.data.status === 'ready') {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`} else {`,
			`postman.setEnvironmentVariable("x", json.data.error.id);`,
			`}`,
		}},
	}}

	result := Translate(events)
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("missing unsupported expression issue: %+v", result.Issues)
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.data.status", "equals", "ready") {
		t.Fatalf("missing if guard assertion: %+v", result.Asserts.JSONPath)
	}
	if result.Captures == nil || len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one translated capture, got %+v", result.Captures)
	}
	if result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected capture path %+v", result.Captures.JSONPath[0])
	}
}

func TestTranslateConditionalElseIfBranchDoesNotLeakGuard(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (json.data.status === 'ready') {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`} else if (json.data.status === 'error') {`,
			`postman.setEnvironmentVariable("x", json.data.error.id);`,
			`}`,
		}},
	}}

	result := Translate(events)
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("missing unsupported expression issue: %+v", result.Issues)
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.data.status", "equals", "ready") {
		t.Fatalf("missing if guard assertion: %+v", result.Asserts.JSONPath)
	}
	if result.Captures == nil || len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one translated capture, got %+v", result.Captures)
	}
	if result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected capture path %+v", result.Captures.JSONPath[0])
	}
}

func TestTranslateJSONPathFailure(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`postman.setEnvironmentVariable("x", json.data.status.toUpperCase());`,
		}},
	}}

	result := Translate(events)
	if !hasIssue(result.Issues, report.CodeScriptJSONPathTranslationFailed) {
		t.Fatalf("missing translation issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateStatusComparisonCaptureDoesNotProduceStatusAssert(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`postman.setEnvironmentVariable("ok", responseCode.code === 200);`,
		}},
	}}

	result := Translate(events)
	if len(result.Asserts.Status) != 0 {
		t.Fatalf("status asserts = %d", len(result.Asserts.Status))
	}
	if !hasIssue(result.Issues, report.CodeScriptJSONPathTranslationFailed) {
		t.Fatalf("missing translation issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateStatusDirectAssertionsStillMap(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`pm.expect(pm.response.code).to.eql(200);`,
			`pm.response.to.have.status(201);`,
		}},
	}}

	result := Translate(events)
	if len(result.Asserts.Status) != 2 {
		t.Fatalf("status asserts = %d", len(result.Asserts.Status))
	}
}

func TestTranslateLooseJSONEqualityIsNotMapped(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["coercive equals"] = json.id == '123';`,
			`tests["coercive not equals"] = json.id != '123';`,
		}},
	}}

	result := Translate(events)
	if hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.id", "equals", "123") {
		t.Fatalf("unexpected equals assertion from loose equality: %+v", result.Asserts.JSONPath)
	}
	if hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.id", "not_equals", "123") {
		t.Fatalf("unexpected not_equals assertion from loose inequality: %+v", result.Asserts.JSONPath)
	}
	if result.UnmappedLines != 2 {
		t.Fatalf("UnmappedLines = %d, expected 2", result.UnmappedLines)
	}
	if !hasIssue(result.Issues, report.CodeScriptLineUnmapped) {
		t.Fatalf("missing unmapped line issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateHasExpressionRequiresFullMatch(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["negated has"] = !_.has(json.data, 'id');`,
			`tests["compound has"] = _.has(json.data, 'id') && true;`,
		}},
	}}

	result := Translate(events)
	if hasJSONPathAssert(result.Asserts.JSONPath, "$.data.id", "exists") {
		t.Fatalf("unexpected exists assertion from unsupported has expression: %+v", result.Asserts.JSONPath)
	}
	if !hasIssue(result.Issues, report.CodeScriptLineUnmapped) {
		t.Fatalf("missing unmapped line issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateHasExpressionDottedPathMapsNestedJSONPath(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["nested has"] = _.has(json.data, 'attributes.status');`,
		}},
	}}

	result := Translate(events)
	if !hasJSONPathAssert(result.Asserts.JSONPath, "$.data.attributes.status", "exists") {
		t.Fatalf("missing nested exists assertion: %+v", result.Asserts.JSONPath)
	}
}

func TestTranslateHasExpressionDottedNumericPathMapsArrayIndex(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["nested array has"] = _.has(json.data, 'items.0.id');`,
		}},
	}}

	result := Translate(events)
	if !hasJSONPathAssert(result.Asserts.JSONPath, "$.data.items[0].id", "exists") {
		t.Fatalf("missing array-index exists assertion: %+v", result.Asserts.JSONPath)
	}
	if hasJSONPathAssert(result.Asserts.JSONPath, "$.data.items['0'].id", "exists") {
		t.Fatalf("unexpected object-key assertion for dotted numeric segment: %+v", result.Asserts.JSONPath)
	}
}

func TestTranslateConditionalHasExpressionDottedPathMapsNestedJSONPath(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (_.has(json.data, 'attributes.status')) {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`}`,
		}},
	}}

	result := Translate(events)
	if !hasJSONPathAssert(result.Asserts.JSONPath, "$.data.attributes.status", "exists") {
		t.Fatalf("missing nested guard assertion: %+v", result.Asserts.JSONPath)
	}
	if result.Captures == nil || len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one capture, got %+v", result.Captures)
	}
	if result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected capture path: %+v", result.Captures.JSONPath[0])
	}
}

func TestTranslateEnvironmentCaptureKeepsLastAssignmentAcrossTypes(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`postman.setEnvironmentVariable("x", responseCode.code);`,
			`postman.setEnvironmentVariable("x", responseHeaders['X-Request-Id']);`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
		}},
	}}

	result := Translate(events)
	if result.Captures == nil {
		t.Fatal("expected captures")
	}
	if len(result.Captures.Status) != 0 {
		t.Fatalf("expected status capture to be replaced, got %+v", result.Captures.Status)
	}
	if len(result.Captures.Headers) != 0 {
		t.Fatalf("expected header capture to be replaced, got %+v", result.Captures.Headers)
	}
	if len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one jsonpath capture, got %+v", result.Captures.JSONPath)
	}
	if result.Captures.JSONPath[0].Name != "x" || result.Captures.JSONPath[0].Path != "$.data.id" {
		t.Fatalf("unexpected jsonpath capture: %+v", result.Captures.JSONPath[0])
	}
}

func TestTranslateEnvironmentCaptureKeepsLatestSupportedValue(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`postman.setEnvironmentVariable("x", responseCode.code);`,
		}},
	}}

	result := Translate(events)
	if result.Captures == nil {
		t.Fatal("expected captures")
	}
	if len(result.Captures.JSONPath) != 0 {
		t.Fatalf("expected jsonpath capture to be replaced, got %+v", result.Captures.JSONPath)
	}
	if len(result.Captures.Status) != 1 {
		t.Fatalf("expected one status capture, got %+v", result.Captures.Status)
	}
	if result.Captures.Status[0].Name != "x" {
		t.Fatalf("unexpected status capture: %+v", result.Captures.Status[0])
	}
}

func TestTranslateEnvironmentCaptureUnsupportedReassignmentRemovesPriorCapture(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`postman.setEnvironmentVariable("x", json.data.status.toUpperCase());`,
		}},
	}}

	result := Translate(events)
	if result.Captures != nil {
		t.Fatalf("expected no captures after unsupported reassignment, got %+v", result.Captures)
	}
	if !hasIssue(result.Issues, report.CodeScriptJSONPathTranslationFailed) {
		t.Fatalf("missing translation failure issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateDecodesEscapedJSLiterals(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`tests["escaped newline"] = json.message === 'line\nvalue';`,
			`tests["escaped quote"] = json.title === "he said \"hi\"";`,
			`tests["escaped single quote"] = json.author === 'O\'Reilly';`,
			`postman.setEnvironmentVariable("capture_key", json["a\"b"]);`,
		}},
	}}

	result := Translate(events)

	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.message", "equals", "line\nvalue") {
		t.Fatalf("missing decoded newline assertion: %+v", result.Asserts.JSONPath)
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.title", "equals", `he said "hi"`) {
		t.Fatalf("missing decoded quote assertion: %+v", result.Asserts.JSONPath)
	}
	if !hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.author", "equals", "O'Reilly") {
		t.Fatalf("missing decoded single-quote assertion: %+v", result.Asserts.JSONPath)
	}

	if result.Captures == nil || len(result.Captures.JSONPath) != 1 {
		t.Fatalf("expected one capture, got %+v", result.Captures)
	}
	if result.Captures.JSONPath[0].Path != `$['a"b']` {
		t.Fatalf("capture path = %q, want %q", result.Captures.JSONPath[0].Path, `$['a"b']`)
	}
}

func TestTranslateUnsupportedConditionalExpression(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (json.data.attributes.status.toUpperCase() === 'READY') {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`}`,
		}},
	}}

	result := Translate(events)
	if result.UnmappedLines != 2 {
		t.Fatalf("expected two unmapped lines, got %d", result.UnmappedLines)
	}
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("missing unsupported expression issue: %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("missing aggregate unmapped issue: %+v", result.Issues)
	}
}

func TestTranslateUnsupportedOuterConditionSkipsInnerGuardTranslation(t *testing.T) {
	t.Parallel()

	events := []ast.Event{{
		Listen: "test",
		Script: ast.Script{Exec: []string{
			`if (json.data.attributes.status.toUpperCase() === 'READY') {`,
			`if (json.data.is_ready === true) {`,
			`postman.setEnvironmentVariable("x", json.data.id);`,
			`}`,
			`}`,
		}},
	}}

	result := Translate(events)
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("missing unsupported expression issue: %+v", result.Issues)
	}
	if hasJSONPathAssertWithValue(result.Asserts.JSONPath, "$.data.is_ready", "equals", true) {
		t.Fatalf("inner guard leaked from unsupported outer condition: %+v", result.Asserts.JSONPath)
	}
	if result.Captures != nil && len(result.Captures.JSONPath) > 0 {
		t.Fatalf("captures should not be translated inside unsupported condition: %+v", result.Captures)
	}
}

func hasIssue(issues []report.Issue, code report.IssueCode) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}

func findIssue(issues []report.Issue, code report.IssueCode) *report.Issue {
	for index := range issues {
		if issues[index].Code == code {
			return &issues[index]
		}
	}
	return nil
}

func hasJSONPathAssert(asserts []model.JSONPathAssert, path, op string) bool {
	for _, assert := range asserts {
		if assert.Path == path && assert.Predicate.Operation == op {
			return true
		}
	}
	return false
}

func hasJSONPathAssertWithValue(asserts []model.JSONPathAssert, path, op string, value any) bool {
	for _, assert := range asserts {
		if assert.Path == path && assert.Predicate.Operation == op && assert.Predicate.HasValue && assert.Predicate.Value == value {
			return true
		}
	}
	return false
}
