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

func FuncMap() template.FuncMap {
	return template.FuncMap{
		"uuidv4": generateUUIDv4,
		"uuid":   generateUUIDv4, // Alias for uuidv4

		"now":       timeNow,
		"timestamp": timeUnix,
		"iso8601":   timeISO8601,
		"rfc3339":   timeRFC3339,

		"upper": strings.ToUpper,
		"lower": strings.ToLower,
		"title": titleCase,
		"trim":  strings.TrimSpace,

		"randomInt":    randomInt,
		"randomString": randomString,

		"base64": base64Encode,
	}
}

func generateUUIDv4() string {
	return uuid.New().String()
}

func timeNow() string {
	return time.Now().Format(time.RFC3339)
}

func timeUnix() string {
	return strconv.FormatInt(time.Now().Unix(), 10)
}

func timeISO8601() string {
	return time.Now().Format("2006-01-02T15:04:05Z07:00")
}

func timeRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

// titleCase uses proper Unicode word boundaries.
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

// randomInt swaps parameters if min > max.
func randomInt(min, max int) int {
	if min > max {
		min, max = max, min
	}

	if min == max {
		return min
	}

	return rand.IntN(max-min+1) + min
}

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

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}

func NewTemplate(name string) *template.Template {
	return template.New(name).Option("missingkey=error").Funcs(FuncMap())
}

// MustParse panics if the template cannot be parsed.
func MustParse(name, text string) *template.Template {
	return template.Must(NewTemplate(name).Parse(text))
}

func Apply(tmplStr string, data any) (string, error) {
	return ApplyWithName("", tmplStr, data)
}

// ApplyWithName is useful for debugging template errors.
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
