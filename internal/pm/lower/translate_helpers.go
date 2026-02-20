package lower

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/jacoelho/rq/internal/pm/diagnostics"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/rq/model"
)

func addStatusAssert(asserts *model.Asserts, seen map[int]struct{}, code int) {
	if _, exists := seen[code]; exists {
		return
	}
	seen[code] = struct{}{}

	asserts.Status = append(asserts.Status, model.StatusAssert{
		Predicate: model.Predicate{
			Operation: "equals",
			Value:     int64(code),
			HasValue:  true,
		},
	})
}

func addJSONPathAssert(asserts *model.Asserts, seen map[string]struct{}, path string, op string, value any, hasValue bool) {
	key := assertKey(path, op, value, hasValue)
	if _, exists := seen[key]; exists {
		return
	}
	seen[key] = struct{}{}

	assert := model.JSONPathAssert{
		Path: path,
		Predicate: model.Predicate{
			Operation: op,
			HasValue:  hasValue,
		},
	}
	if hasValue {
		assert.Predicate.Value = value
	}

	asserts.JSONPath = append(asserts.JSONPath, assert)
}

func assertKey(path string, op string, value any, hasValue bool) string {
	if !hasValue {
		return fmt.Sprintf("%s|%s", path, op)
	}
	return fmt.Sprintf("%s|%s|%T|%v", path, op, value, value)
}

func mapHasAssertion(asserts *model.Asserts, seen map[string]struct{}, line string) (bool, bool) {
	expression := extractTestExpression(line)
	if expression == "" {
		return false, false
	}

	path, ok := parseHasExpression(expression)
	if !ok {
		return false, false
	}

	addJSONPathAssert(asserts, seen, path, "exists", nil, false)
	return true, true
}

func mapJSONComparison(asserts *model.Asserts, seen map[string]struct{}, line string) (bool, bool) {
	expression := extractTestExpression(line)
	if expression == "" {
		return false, false
	}

	path, op, value, hasValue, ok := parseJSONComparisonExpression(expression)
	if !ok {
		return false, false
	}

	addJSONPathAssert(asserts, seen, path, op, value, hasValue)
	return true, true
}

func mapArrayTypeAssertion(asserts *model.Asserts, seen map[string]struct{}, line string) (bool, bool) {
	expression := extractTestExpression(line)
	if expression == "" {
		return false, false
	}

	path, ok := parseArrayIsArrayExpression(expression)
	if !ok {
		return false, false
	}

	addJSONPathAssert(asserts, seen, path, "type_is", "array", true)
	return true, true
}

func mapEnvironmentCapture(captures *model.Captures, line string) (bool, bool, string) {
	matches := setEnvironmentPattern.FindStringSubmatch(line)
	if len(matches) != 3 {
		return false, false, ""
	}

	name := strings.TrimSpace(matches[1])
	if name == "" {
		return false, false, "capture name is empty"
	}

	// A later unsupported assignment must invalidate earlier mappings for this name.
	removeCaptureByName(captures, name)

	valueExpr := strings.TrimSpace(strings.TrimSuffix(matches[2], ";"))
	if isStatusExpression(valueExpr) {
		captures.Status = append(captures.Status, model.StatusCapture{Name: name})
		return true, false, ""
	}

	if headerName := parseHeaderExpression(valueExpr); headerName != "" {
		captures.Headers = append(captures.Headers, model.HeaderCapture{Name: name, HeaderName: headerName})
		return true, false, ""
	}

	path, ok := jsonExprToPath(valueExpr)
	if !ok {
		return false, false, "unsupported capture expression"
	}

	captures.JSONPath = append(captures.JSONPath, model.JSONPathCapture{
		Name: name,
		Path: path,
	})
	return true, true, ""
}

func hasAnyCaptures(captures model.Captures) bool {
	return len(captures.Status) > 0 || len(captures.Headers) > 0 || len(captures.Certificate) > 0 || len(captures.JSONPath) > 0 || len(captures.Regex) > 0 || len(captures.Body) > 0
}

func removeCaptureByName(captures *model.Captures, name string) {
	captures.Status = removeStatusCapturesByName(captures.Status, name)
	captures.Headers = removeHeaderCapturesByName(captures.Headers, name)
	captures.JSONPath = removeJSONPathCapturesByName(captures.JSONPath, name)
}

func removeStatusCapturesByName(captures []model.StatusCapture, name string) []model.StatusCapture {
	filtered := captures[:0]
	for _, capture := range captures {
		if capture.Name == name {
			continue
		}
		filtered = append(filtered, capture)
	}
	return filtered
}

func removeHeaderCapturesByName(captures []model.HeaderCapture, name string) []model.HeaderCapture {
	filtered := captures[:0]
	for _, capture := range captures {
		if capture.Name == name {
			continue
		}
		filtered = append(filtered, capture)
	}
	return filtered
}

func removeJSONPathCapturesByName(captures []model.JSONPathCapture, name string) []model.JSONPathCapture {
	filtered := captures[:0]
	for _, capture := range captures {
		if capture.Name == name {
			continue
		}
		filtered = append(filtered, capture)
	}
	return filtered
}

func extractTestExpression(line string) string {
	matches := testExpressionPattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(matches) != 2 {
		return ""
	}

	return strings.TrimSpace(matches[1])
}

func extractStatusAssertionCode(line string) (int, bool) {
	trimmed := strings.TrimSpace(line)
	if expression := extractTestExpression(trimmed); expression != "" {
		return extractStatusCodeFromPatterns(expression, statusTestExpressionPatterns)
	}

	return extractStatusCodeFromPatterns(trimmed, statusDirectAssertionPatterns)
}

func extractStatusCodeFromPatterns(input string, patterns []*regexp.Regexp) (int, bool) {
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(input)
		if len(matches) < 2 {
			continue
		}

		code, err := strconv.Atoi(matches[1])
		if err != nil {
			continue
		}

		return code, true
	}

	return 0, false
}

func parseConditionalExpression(conditionExpression string) (conditionalGuard, bool) {
	conditionExpression = strings.TrimSpace(conditionExpression)
	if path, op, value, hasValue, ok := parseJSONComparisonExpression(conditionExpression); ok {
		return conditionalGuard{
			path:         path,
			op:           op,
			value:        value,
			hasValue:     hasValue,
			requiresJSON: true,
		}, true
	}

	if path, ok := parseHasExpression(conditionExpression); ok {
		return conditionalGuard{
			path:         path,
			op:           "exists",
			requiresJSON: true,
		}, true
	}

	if path, ok := parseArrayIsArrayExpression(conditionExpression); ok {
		return conditionalGuard{
			path:         path,
			op:           "type_is",
			value:        "array",
			hasValue:     true,
			requiresJSON: true,
		}, true
	}

	return conditionalGuard{}, false
}

func parseHasExpression(expression string) (string, bool) {
	matches := hasPattern.FindStringSubmatch(strings.TrimSpace(expression))
	if len(matches) != 3 {
		return "", false
	}

	baseExpr := strings.TrimSpace(matches[1])
	field := strings.TrimSpace(matches[2])
	if field == "" {
		return "", false
	}

	basePath, ok := jsonExprToPath(baseExpr)
	if !ok {
		return "", false
	}

	segments, ok := parseHasPathSegments(field)
	if !ok {
		return "", false
	}

	path := basePath
	for _, segment := range segments {
		if segment.isIndex {
			path = fmt.Sprintf("%s[%s]", path, segment.index)
			continue
		}
		path = appendJSONPathSegment(path, segment.key)
	}

	return path, true
}

func parseHasPathSegments(field string) ([]hasPathSegment, bool) {
	remaining := strings.TrimSpace(field)
	if remaining == "" {
		return nil, false
	}

	segments := make([]hasPathSegment, 0)
	expectingSegment := true

	for len(remaining) > 0 {
		if strings.HasPrefix(remaining, ".") {
			if expectingSegment {
				return nil, false
			}
			remaining = strings.TrimSpace(remaining[1:])
			expectingSegment = true
			continue
		}

		if strings.HasPrefix(remaining, "[") {
			end := strings.IndexByte(remaining, ']')
			if end <= 1 {
				return nil, false
			}
			content := strings.TrimSpace(remaining[1:end])
			if !isDigits(content) {
				return nil, false
			}
			segments = append(segments, hasPathSegment{
				index:   content,
				isIndex: true,
			})
			remaining = strings.TrimSpace(remaining[end+1:])
			expectingSegment = false
			continue
		}

		boundary := len(remaining)
		if dot := strings.IndexByte(remaining, '.'); dot >= 0 && dot < boundary {
			boundary = dot
		}
		if bracket := strings.IndexByte(remaining, '['); bracket >= 0 && bracket < boundary {
			boundary = bracket
		}

		token := strings.TrimSpace(remaining[:boundary])
		if token == "" {
			return nil, false
		}

		if isDigits(token) {
			segments = append(segments, hasPathSegment{
				index:   token,
				isIndex: true,
			})
		} else {
			segments = append(segments, hasPathSegment{key: token})
		}
		remaining = strings.TrimSpace(remaining[boundary:])
		expectingSegment = false
	}

	if expectingSegment {
		return nil, false
	}

	return segments, true
}

func parseJSONComparisonExpression(expression string) (string, string, any, bool, bool) {
	matches := jsonComparisonPattern.FindStringSubmatch(strings.TrimSpace(expression))
	if len(matches) != 4 {
		return "", "", nil, false, false
	}

	path, ok := jsonExprToPath(matches[1])
	if !ok {
		return "", "", nil, false, false
	}

	value, ok := parseLiteral(matches[3])
	if !ok {
		return "", "", nil, false, false
	}

	op := "equals"
	if matches[2] == "!==" {
		op = "not_equals"
	}

	return path, op, value, true, true
}

func parseArrayIsArrayExpression(expression string) (string, bool) {
	matches := arrayIsArrayPattern.FindStringSubmatch(strings.TrimSpace(expression))
	if len(matches) != 2 {
		return "", false
	}

	path, ok := jsonExprToPath(matches[1])
	if !ok {
		return "", false
	}

	return path, true
}

func jsonExprToPath(expr string) (string, bool) {
	expr = strings.TrimSpace(strings.TrimSuffix(expr, ";"))
	if expr == "json" {
		return "$", true
	}
	if !strings.HasPrefix(expr, "json") {
		return "", false
	}

	rest := expr[len("json"):]
	path := "$"

	for len(rest) > 0 {
		switch rest[0] {
		case '.':
			rest = rest[1:]
			identifier, consumed := consumeIdentifier(rest)
			if consumed == 0 {
				return "", false
			}
			path = path + "." + identifier
			rest = rest[consumed:]
		case '[':
			end := strings.IndexByte(rest, ']')
			if end <= 1 {
				return "", false
			}

			content := strings.TrimSpace(rest[1:end])
			rest = rest[end+1:]

			if isDigits(content) {
				path = fmt.Sprintf("%s[%s]", path, content)
				continue
			}

			key, ok := parseQuoted(content)
			if !ok {
				return "", false
			}

			path = appendJSONPathSegment(path, key)
		default:
			return "", false
		}
	}

	return path, true
}

func appendJSONPathSegment(path string, segment string) string {
	if isIdentifier(segment) {
		if path == "$" {
			return "$." + segment
		}
		return path + "." + segment
	}

	escaped := strings.ReplaceAll(segment, "'", "\\'")
	return fmt.Sprintf("%s['%s']", path, escaped)
}

func consumeIdentifier(input string) (string, int) {
	if input == "" {
		return "", 0
	}

	for i, r := range input {
		if i == 0 {
			if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')) {
				return "", 0
			}
			continue
		}

		if !(r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')) {
			return input[:i], i
		}
	}

	return input, len(input)
}

func isIdentifier(input string) bool {
	_, consumed := consumeIdentifier(input)
	return consumed == len(input)
}

func isDigits(input string) bool {
	if input == "" {
		return false
	}
	for _, r := range input {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func parseQuoted(input string) (string, bool) {
	if len(input) < 2 {
		return "", false
	}

	quote := input[0]
	if (quote == '\'' || quote == '"') && input[len(input)-1] == quote {
		decoded, ok := decodeJSStringLiteral(input[1:len(input)-1], quote)
		if !ok {
			return "", false
		}
		return decoded, true
	}

	return "", false
}

func decodeJSStringLiteral(raw string, quote byte) (string, bool) {
	var out strings.Builder
	out.Grow(len(raw))

	for index := 0; index < len(raw); index++ {
		current := raw[index]
		if current != '\\' {
			out.WriteByte(current)
			continue
		}

		index++
		if index >= len(raw) {
			return "", false
		}

		escaped := raw[index]
		switch escaped {
		case '\\':
			out.WriteByte('\\')
		case '"':
			out.WriteByte('"')
		case '\'':
			out.WriteByte('\'')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		case '0':
			out.WriteByte('\x00')
		case '\n':
			// JavaScript line continuation: backslash + newline is removed.
		case '\r':
			// JavaScript line continuation: swallow optional following newline.
			if index+1 < len(raw) && raw[index+1] == '\n' {
				index++
			}
		case 'x':
			if index+2 >= len(raw) {
				return "", false
			}
			value, err := strconv.ParseUint(raw[index+1:index+3], 16, 8)
			if err != nil {
				return "", false
			}
			out.WriteByte(byte(value))
			index += 2
		case 'u':
			if index+1 < len(raw) && raw[index+1] == '{' {
				end := strings.IndexByte(raw[index+2:], '}')
				if end < 0 {
					return "", false
				}
				hexValue := raw[index+2 : index+2+end]
				if hexValue == "" {
					return "", false
				}
				value, err := strconv.ParseUint(hexValue, 16, 32)
				if err != nil || value > utf8.MaxRune {
					return "", false
				}
				out.WriteRune(rune(value))
				index = index + 2 + end
				continue
			}

			if index+4 >= len(raw) {
				return "", false
			}
			firstValue, err := strconv.ParseUint(raw[index+1:index+5], 16, 16)
			if err != nil {
				return "", false
			}
			firstRune := rune(firstValue)

			if utf16.IsSurrogate(firstRune) && index+10 < len(raw) && raw[index+5] == '\\' && raw[index+6] == 'u' {
				secondValue, secondErr := strconv.ParseUint(raw[index+7:index+11], 16, 16)
				if secondErr == nil {
					secondRune := rune(secondValue)
					decoded := utf16.DecodeRune(firstRune, secondRune)
					if decoded != utf8.RuneError {
						out.WriteRune(decoded)
						index += 10
						continue
					}
				}
			}

			out.WriteRune(firstRune)
			index += 4
		default:
			// Preserve JS non-special escapes by returning the escaped character.
			if escaped == quote {
				out.WriteByte(quote)
				continue
			}
			out.WriteByte(escaped)
		}
	}

	return out.String(), true
}

func parseLiteral(input string) (any, bool) {
	value := strings.TrimSpace(strings.TrimSuffix(input, ";"))
	if value == "" {
		return nil, false
	}

	if unquoted, ok := parseQuoted(value); ok {
		return unquoted, true
	}

	switch strings.ToLower(value) {
	case "true":
		return true, true
	case "false":
		return false, true
	case "null":
		return nil, true
	}

	if intValue, err := strconv.ParseInt(value, 10, 64); err == nil {
		return intValue, true
	}

	if floatValue, err := strconv.ParseFloat(value, 64); err == nil {
		return floatValue, true
	}

	return nil, false
}

func isStatusExpression(input string) bool {
	trimmed := strings.TrimSpace(input)
	return trimmed == "responseCode.code" || trimmed == "pm.response.code"
}

func parseHeaderExpression(input string) string {
	trimmed := strings.TrimSpace(input)
	if matches := headerCapturePattern.FindStringSubmatch(trimmed); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	if matches := pmHeaderCaptureRegex.FindStringSubmatch(trimmed); len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func isJSONParseLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "var json = JSON.parse(responseBody)") || strings.HasPrefix(line, "let json = JSON.parse(responseBody)") || strings.HasPrefix(line, "const json = JSON.parse(responseBody)")
}

func isJSONValidityLine(line string) bool {
	lower := strings.ToLower(strings.TrimSpace(line))
	return strings.Contains(lower, "response is valid json") && strings.Contains(lower, "= true")
}

func hasUnsupportedCondition(stack []conditionFrame) bool {
	for _, frame := range stack {
		if !frame.supported {
			return true
		}
	}
	return false
}

func buildUnmappedIssues(counts map[report.IssueCode]int, firstLine map[report.IssueCode]int, total int) []report.Issue {
	if total == 0 {
		return nil
	}

	codes := make([]string, 0, len(counts))
	for code := range counts {
		codes = append(codes, string(code))
	}
	sort.Strings(codes)

	issues := make([]report.Issue, 0, len(codes)+1)
	for _, code := range codes {
		issueCode := report.IssueCode(code)
		issue := report.Issue{
			Code:     issueCode,
			Stage:    diagnostics.StageLower,
			Severity: diagnostics.SeverityError,
			Message:  fmt.Sprintf("%d script lines were not mapped (%s)", counts[issueCode], issueCode),
		}
		if line := firstLine[issueCode]; line > 0 {
			issue.Span = &diagnostics.Span{Line: line}
		}
		issues = append(issues, report.Issue{
			Code:     issue.Code,
			Stage:    issue.Stage,
			Severity: issue.Severity,
			Message:  issue.Message,
			Span:     issue.Span,
		})
	}

	issues = append(issues, report.Issue{
		Code:     report.CodeTestNotMapped,
		Stage:    diagnostics.StageLower,
		Severity: diagnostics.SeverityError,
		Message:  fmt.Sprintf("%d test script lines were not mapped", total),
	})

	return issues
}
