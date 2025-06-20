package template

import (
	"encoding/base64"
	"math/rand/v2"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/google/uuid"
)

// FuncMap returns a template.FuncMap with all available custom functions.
func FuncMap() template.FuncMap {
	return template.FuncMap{
		// UUID functions
		"uuidv4": generateUUIDv4,
		"uuid":   generateUUIDv4, // Alias for uuidv4

		// Time functions
		"now":       timeNow,
		"timestamp": timeUnix,
		"iso8601":   timeISO8601,
		"rfc3339":   timeRFC3339,

		// String functions
		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": titleCase,
		"trim":  strings.TrimSpace,

		// Random functions
		"randomInt":    randomInt,
		"randomString": randomString,

		// Encoding functions
		"base64": base64Encode,
	}
}

// generateUUIDv4 generates a new UUID v4 string.
func generateUUIDv4() string {
	return uuid.New().String()
}

// timeNow returns the current time formatted as RFC3339.
func timeNow() string {
	return time.Now().Format(time.RFC3339)
}

// timeUnix returns the current Unix timestamp as a string.
func timeUnix() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

// timeISO8601 returns the current time in ISO8601 format.
func timeISO8601() string {
	return time.Now().Format("2006-01-02T15:04:05Z07:00")
}

// timeRFC3339 returns the current time in RFC3339 format.
func timeRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

// titleCase converts a string to title case using proper Unicode word boundaries.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, word := range words {
		if len(word) > 0 {
			runes := []rune(word)
			runes[0] = unicode.ToUpper(runes[0])
			words[i] = string(runes)
		}
	}
	return strings.Join(words, " ")
}

// randomInt generates a random integer between min and max (inclusive).
// If min > max, the parameters are swapped.
func randomInt(min, max int) int {
	if min > max {
		min, max = max, min
	}

	if min == max {
		return min
	}

	return rand.IntN(max-min+1) + min
}

// randomString generates a random alphanumeric string of the specified length.
// Returns empty string for non-positive lengths.
func randomString(length int) string {
	if length <= 0 {
		return ""
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	buf := make([]byte, length)
	for i := range buf {
		buf[i] = charset[rand.IntN(len(charset))]
	}

	return string(buf)
}

// base64Encode encodes a string to base64.
func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

// NewTemplate creates a new template with custom functions.
// This is a convenience function that creates a template with all custom functions pre-loaded.
func NewTemplate(name string) *template.Template {
	return template.New(name).Funcs(FuncMap())
}

// MustParse creates a new template with custom functions and parses the given text.
// It panics if the template cannot be parsed.
func MustParse(name, text string) *template.Template {
	return template.Must(NewTemplate(name).Parse(text))
}

// Apply applies template substitution using the provided data and custom functions.
// Returns the processed template as a string or an error if processing fails.
func Apply(tmplStr string, data any) (string, error) {
	return ApplyWithName("", tmplStr, data)
}

// ApplyWithName applies template substitution with a custom template name.
// This can be useful for debugging template errors.
func ApplyWithName(name, tmplStr string, data any) (string, error) {
	if tmplStr == "" {
		return "", nil
	}

	tmpl, err := NewTemplate(name).Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
