package extractor

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/theory/jsonpath"
)

// ExtractJSONPath extracts data from JSON using JSONPath expressions.
// Supports standard JSONPath syntax for navigating JSON structures (e.g., "$.user.name", "$..items[0]").
// Returns the first matching result, or ErrNotFound if no matches are found.
// Returns ErrExtraction for invalid JSON data or malformed JSONPath expressions.
// Returns ErrInvalidInput if body is empty or pathExpr is empty.
func ExtractJSONPath(body []byte, pathExpr string) (any, error) {
	if len(body) == 0 {
		return nil, fmt.Errorf("%w: body is empty", ErrInvalidInput)
	}

	if pathExpr == "" {
		return nil, fmt.Errorf("%w: JSONPath expression is empty", ErrInvalidInput)
	}

	// Parse the JSONPath expression
	path, err := jsonpath.Parse(pathExpr)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid JSONPath %s: %v", ErrExtraction, pathExpr, err)
	}

	// Parse the JSON data
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("%w: failed to parse JSON data: %v", ErrExtraction, err)
	}

	// Execute the JSONPath query
	results := path.Select(data)

	// Return the first result if available
	if len(results) > 0 {
		return results[0], nil
	}

	return nil, ErrNotFound
}

// ExtractJSONPathString extracts a JSONPath value and converts it to a string.
// Convenience function that combines ExtractJSONPath with string conversion.
// Non-string values are converted using fmt.Sprintf("%v", value).
// Returns the same errors as ExtractJSONPath.
func ExtractJSONPathString(body []byte, path string) (string, error) {
	result, err := ExtractJSONPath(body, path)
	if err != nil {
		return "", err
	}

	if str, ok := result.(string); ok {
		return str, nil
	}

	// Try to convert to string
	return fmt.Sprintf("%v", result), nil
}

// ExtractRegex extracts data from content using regular expressions.
// Returns the specified capture group from the first match found.
// Group 0 returns the entire match, group 1+ return numbered capture groups.
// Returns ErrNotFound if no matches are found.
// Returns ErrInvalidInput for empty patterns, negative groups, or invalid regex.
// Returns ErrExtraction if the specified group doesn't exist in the match.
func ExtractRegex(body []byte, pattern string, group int) (any, error) {
	if pattern == "" {
		return nil, fmt.Errorf("%w: regex pattern is empty", ErrInvalidInput)
	}

	if group < 0 {
		return nil, fmt.Errorf("%w: capture group must be >= 0, got: %d", ErrInvalidInput, group)
	}

	// Compile the regex pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid regex pattern %s: %v", ErrInvalidInput, pattern, err)
	}

	// Find all submatches
	matches := re.FindSubmatch(body)
	if matches == nil {
		return nil, ErrNotFound
	}

	// Check if the requested group exists
	if group >= len(matches) {
		return nil, fmt.Errorf("%w: invalid capture group %d for pattern (found %d groups)",
			ErrExtraction, group, len(matches)-1)
	}

	// Extract the specified group
	value := string(matches[group])
	return value, nil
}

// ExtractAllRegex finds all regex matches and extracts the specified capture group from each.
// Similar to ExtractRegex but returns all matches instead of just the first one.
// Useful for extracting multiple occurrences of a pattern (e.g., all email addresses).
// Returns ErrNotFound if no matches are found.
// Returns the same validation errors as ExtractRegex.
func ExtractAllRegex(body []byte, pattern string, group int) ([]string, error) {
	if pattern == "" {
		return nil, fmt.Errorf("%w: regex pattern is empty", ErrInvalidInput)
	}

	if group < 0 {
		return nil, fmt.Errorf("%w: capture group must be >= 0, got: %d", ErrInvalidInput, group)
	}

	// Compile the regex pattern
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

// ExtractRegexString extracts a regex match and converts it to a string.
// Convenience function that combines ExtractRegex with string conversion.
// Since regex extraction already returns strings, this mainly provides type safety.
// Returns the same errors as ExtractRegex.
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
