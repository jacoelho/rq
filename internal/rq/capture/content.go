package capture

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/theory/jsonpath"
)

// ParseJSONBody decodes a JSON response payload once so multiple selectors can reuse it.
func ParseJSONBody(body []byte) (any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: body is empty", ErrInvalidInput)
	}

	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("%w: failed to parse JSON data: %v", ErrExtraction, err)
	}

	return data, nil
}

// ExtractJSONPathFromData selects the first value matching pathExpr from decoded JSON data.
func ExtractJSONPathFromData(data any, pathExpr string) (any, error) {
	if pathExpr == "" {
		return nil, fmt.Errorf("%w: JSONPath expression is empty", ErrInvalidInput)
	}

	path, err := jsonpath.Parse(pathExpr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JSONPath %s: %v", ErrExtraction, pathExpr, err)
	}

	results := path.Select(data)
	if len(results) > 0 {
		return results[0], nil
	}

	return nil, ErrNotFound
}

// ExtractJSONPathFromDataString converts non-string values using fmt.Sprintf.
func ExtractJSONPathFromDataString(data any, pathExpr string) (string, error) {
	result, err := ExtractJSONPathFromData(data, pathExpr)
	if err != nil {
		return "", err
	}

	if str, ok := result.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", result), nil
}

// ExtractJSONPath supports standard JSONPath syntax (e.g., "$.user.name", "$..items[0]").
func ExtractJSONPath(body []byte, pathExpr string) (any, error) {
	data, err := ParseJSONBody(body)
	if err != nil {
		return nil, err
	}

	return ExtractJSONPathFromData(data, pathExpr)
}

// ExtractJSONPathString converts non-string values using fmt.Sprintf.
func ExtractJSONPathString(body []byte, path string) (string, error) {
	data, err := ParseJSONBody(body)
	if err != nil {
		return "", err
	}

	return ExtractJSONPathFromDataString(data, path)
}

// ExtractRegex uses capture groups: 0 = entire match, 1+ = numbered groups.
func ExtractRegex(body []byte, pattern string, group int) (any, error) {
	if pattern == "" {
		return nil, fmt.Errorf("%w: regex pattern is empty", ErrInvalidInput)
	}

	if group < 0 {
		return nil, fmt.Errorf("%w: capture group must be >= 0, got: %d", ErrInvalidInput, group)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex pattern %s: %v", ErrInvalidInput, pattern, err)
	}

	matches := re.FindSubmatch(body)
	if matches == nil {
		return nil, ErrNotFound
	}

	if group >= len(matches) {
		return nil, fmt.Errorf("%w: invalid capture group %d for pattern (found %d groups)",
			ErrExtraction, group, len(matches)-1)
	}

	value := string(matches[group])
	return value, nil
}

// ExtractAllRegex extracts multiple occurrences (e.g., all email addresses).
func ExtractAllRegex(body []byte, pattern string, group int) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("%w: regex pattern is empty", ErrInvalidInput)
	}

	if group < 0 {
		return nil, fmt.Errorf("%w: capture group must be >= 0, got: %d", ErrInvalidInput, group)
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex pattern %s: %v", ErrInvalidInput, pattern, err)
	}

	allMatches := re.FindAllSubmatch(body, -1)
	if len(allMatches) == 0 {
		return nil, ErrNotFound
	}

	results := make([]string, 0, len(allMatches))
	for _, matches := range allMatches {
		if group >= len(matches) {
			return nil, fmt.Errorf("%w: invalid capture group %d for pattern (found %d groups)",
				ErrExtraction, group, len(matches)-1)
		}

		value := string(matches[group])
		results = append(results, value)
	}

	return results, nil
}

func ExtractRegexString(body []byte, pattern string, group int) (string, error) {
	result, err := ExtractRegex(body, pattern, group)
	if err != nil {
		return "", err
	}

	if str, ok := result.(string); ok {
		return str, nil
	}

	return fmt.Sprintf("%v", result), nil
}
