package lower

import (
	"fmt"
	"strconv"
	"strings"
)

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
