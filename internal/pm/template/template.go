package template

import (
	"regexp"
	"strings"
)

var (
	placeholderPattern = regexp.MustCompile(`\{\{\s*([^{}]+?)\s*\}\}`)
	simpleVariable     = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)
)

// Diagnostic reports placeholder patterns that cannot be safely normalized.
type Diagnostic struct {
	Placeholder string
	Inner       string
	Reason      string
	Start       int
	End         int
}

// Normalize rewrites source placeholders into rq-compatible template paths.
func Normalize(input string) string {
	normalized, _ := NormalizeDetailed(input)
	return normalized
}

// NormalizeDetailed rewrites placeholders and reports unsupported forms.
func NormalizeDetailed(input string) (string, []Diagnostic) {
	matches := placeholderPattern.FindAllStringSubmatchIndex(input, -1)
	if len(matches) == 0 {
		return input, nil
	}

	diagnostics := make([]Diagnostic, 0)
	var builder strings.Builder
	builder.Grow(len(input))

	last := 0
	for _, match := range matches {
		start, end := match[0], match[1]
		innerStart, innerEnd := match[2], match[3]

		builder.WriteString(input[last:start])

		inner := strings.TrimSpace(input[innerStart:innerEnd])
		normalized, reason := normalizeInner(inner)
		builder.WriteString(normalized)

		if reason != "" {
			diagnostics = append(diagnostics, Diagnostic{
				Placeholder: input[start:end],
				Inner:       inner,
				Reason:      reason,
				Start:       start,
				End:         end,
			})
		}

		last = end
	}

	builder.WriteString(input[last:])
	return builder.String(), diagnostics
}

func normalizeInner(inner string) (string, string) {
	if inner == "" {
		return "{{}}", "empty placeholder expression"
	}

	if strings.HasPrefix(inner, ".") {
		value := strings.TrimPrefix(inner, ".")
		if simpleVariable.MatchString(value) {
			return "{{." + value + "}}", ""
		}
		return "{{" + inner + "}}", "unsupported placeholder syntax"
	}

	if mapped, ok := normalizeDynamicVariable(inner); ok {
		return "{{" + mapped + "}}", ""
	}

	if simpleVariable.MatchString(inner) {
		return "{{." + inner + "}}", ""
	}

	if strings.HasPrefix(inner, "$") {
		return "{{" + inner + "}}", "unsupported dynamic placeholder"
	}

	return "{{" + inner + "}}", "unsupported placeholder syntax"
}

func normalizeDynamicVariable(inner string) (string, bool) {
	switch strings.ToLower(inner) {
	case "$timestamp":
		return "timestamp", true
	case "$guid":
		return "uuidv4", true
	default:
		return "", false
	}
}
