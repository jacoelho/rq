package lower

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
