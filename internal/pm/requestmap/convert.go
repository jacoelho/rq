package requestmap

import (
	"encoding/json"
	"fmt"
	"net/textproto"
	"net/url"
	"strings"

	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/diagnostics"
	"github.com/jacoelho/rq/internal/pm/lower"
	"github.com/jacoelho/rq/internal/pm/normalize"
	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/pm/template"
	"github.com/jacoelho/rq/internal/rq/model"
)

// Result contains the conversion output for one source request.
type Result struct {
	Step      model.Step
	Converted bool
	Issues    []report.Issue
}

func requestIssue(code report.IssueCode, message string) report.Issue {
	definition := diagnostics.DefinitionFor(code)
	return report.Issue{
		Code:     code,
		Stage:    definition.DefaultStage,
		Severity: definition.DefaultSeverity,
		Message:  message,
	}
}

// Request converts a source request node into one rq step.
func Request(node normalize.RequestNode) Result {
	result := Result{}
	method := strings.ToUpper(strings.TrimSpace(node.Request.Method))
	if method == "" {
		result.Issues = append(result.Issues, requestIssue(report.CodeInvalidRequestShape, "missing HTTP method"))
		return result
	}
	if !model.IsSupportedMethod(method) {
		result.Issues = append(result.Issues, requestIssue(report.CodeInvalidRequestShape, fmt.Sprintf("unsupported HTTP method: %s", method)))
		return result
	}

	urlValue, query, urlIssues := convertURL(node)
	result.Issues = append(result.Issues, urlIssues...)
	if strings.TrimSpace(urlValue) == "" {
		result.Issues = append(result.Issues, requestIssue(report.CodeInvalidRequestShape, "missing request URL"))
		return result
	}

	headers, headerIssues := convertHeaders(node)
	result.Issues = append(result.Issues, headerIssues...)

	body, bodyFile, bodyHeaders, bodyIssues := convertBody(node)
	result.Issues = append(result.Issues, bodyIssues...)
	if len(bodyHeaders) > 0 {
		for _, header := range bodyHeaders {
			if !hasHeader(headers, header.Key) {
				headers = append(headers, header)
			}
		}
	}

	if hasAuth(node) && !hasHeader(headers, "Authorization") {
		result.Issues = append(result.Issues, requestIssue(report.CodeAuthNotMapped, "auth configuration was not mapped; define equivalent headers/variables manually"))
	}

	scriptResult := lower.Translate(node.Events)
	result.Issues = append(result.Issues, scriptResult.Issues...)

	step := model.Step{
		Method:   method,
		URL:      urlValue,
		Headers:  nil,
		Query:    nil,
		Body:     body,
		BodyFile: bodyFile,
		Asserts:  scriptResult.Asserts,
	}
	step.Captures = scriptResult.Captures

	if len(headers) > 0 {
		step.Headers = headers
	}
	if len(query) > 0 {
		step.Query = query
	}

	result.Step = step
	result.Converted = !report.HasErrors(result.Issues)
	return result
}

func convertURL(node normalize.RequestNode) (string, model.KeyValues, []report.Issue) {
	resolved := node.Request.EffectiveURL()
	raw := strings.TrimSpace(resolved.Raw)
	if raw == "" && (len(resolved.Host) > 0 || len(resolved.Path) > 0) {
		host := strings.Join(resolved.Host, "")
		if host != "" && !strings.Contains(host, "://") && len(resolved.Host) > 1 {
			host = strings.Join(resolved.Host, ".")
		}

		if host != "" {
			if !strings.Contains(host, "://") {
				host = normalizeURLScheme(resolved.Protocol) + "://" + strings.TrimLeft(host, "/")
			}
			host = appendURLPort(host, resolved.Port)

			pathValue := strings.Join(resolved.Path, "/")
			if pathValue != "" {
				host = strings.TrimRight(host, "/") + "/" + strings.TrimLeft(pathValue, "/")
			}
			raw = host
		}
	}

	query, queryIssues := convertQuery(resolved.Query)
	issues := make([]report.Issue, 0, len(queryIssues))
	issues = append(issues, queryIssues...)

	if len(resolved.Query) > 0 {
		raw = stripRawQuery(raw)
	}

	normalized, normalizeIssues := normalizeWithIssues(raw, "url")
	issues = append(issues, normalizeIssues...)
	return normalized, query, issues
}

func normalizeURLScheme(protocol string) string {
	trimmed := strings.TrimSpace(protocol)
	trimmed = strings.TrimSuffix(trimmed, "://")
	trimmed = strings.TrimSuffix(trimmed, ":")
	if trimmed == "" {
		return "https"
	}
	return trimmed
}

func appendURLPort(host string, port string) string {
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return host
	}

	prefix := ""
	authority := host
	if index := strings.Index(authority, "://"); index >= 0 {
		prefix = authority[:index+len("://")]
		authority = authority[index+len("://"):]
	}

	authority = strings.TrimSpace(authority)
	if authority == "" {
		return host
	}

	authorityOnly, suffix := splitAuthoritySuffix(authority)
	authorityOnly = strings.TrimSpace(strings.TrimRight(authorityOnly, "/"))
	if authorityOnly == "" {
		return host
	}
	if hasExplicitURLPort(authorityOnly) {
		return prefix + authorityOnly + suffix
	}

	return prefix + authorityOnly + ":" + port + suffix
}

func splitAuthoritySuffix(authority string) (string, string) {
	index := strings.IndexAny(authority, "/?#")
	if index < 0 {
		return authority, ""
	}
	return authority[:index], authority[index:]
}

func hasExplicitURLPort(authority string) bool {
	if strings.HasPrefix(authority, "[") {
		closing := strings.Index(authority, "]")
		if closing < 0 {
			return true
		}
		return len(authority) > closing+1 && authority[closing+1] == ':'
	}

	colonCount := strings.Count(authority, ":")
	if colonCount > 1 {
		return true
	}

	return colonCount == 1
}

func convertHeaders(node normalize.RequestNode) (model.KeyValues, []report.Issue) {
	return convertNormalizedKeyValues(
		node.Request.Header,
		func(header ast.Header) bool { return header.Disabled },
		func(header ast.Header) string { return textproto.CanonicalMIMEHeaderKey(header.Key) },
		func(header ast.Header) string { return header.Value },
		"header",
	)
}

func convertQuery(params []ast.QueryParam) (model.KeyValues, []report.Issue) {
	return convertNormalizedKeyValues(
		params,
		func(param ast.QueryParam) bool { return param.Disabled },
		func(param ast.QueryParam) string { return param.Key },
		func(param ast.QueryParam) string { return param.Value },
		"query",
	)
}

func convertNormalizedKeyValues[T any](
	entries []T,
	isDisabled func(T) bool,
	getKey func(T) string,
	getValue func(T) string,
	fieldName string,
) (model.KeyValues, []report.Issue) {
	var values model.KeyValues
	var issues []report.Issue

	for _, entry := range entries {
		if isDisabled(entry) {
			continue
		}

		key := strings.TrimSpace(getKey(entry))
		if key == "" {
			continue
		}

		normalized, normalizedIssues := normalizeWithIssues(getValue(entry), fmt.Sprintf("%s[%s]", fieldName, key))
		issues = append(issues, normalizedIssues...)
		values = append(values, model.KeyValue{
			Key:   key,
			Value: normalized,
		})
	}

	if len(values) == 0 {
		return nil, issues
	}

	return values, issues
}

func convertBody(node normalize.RequestNode) (string, string, model.KeyValues, []report.Issue) {
	if node.Request.Body == nil {
		return "", "", nil, nil
	}

	mode := strings.ToLower(strings.TrimSpace(node.Request.Body.Mode))
	switch mode {
	case "", "none":
		return "", "", nil, nil
	case "raw":
		normalized, issues := normalizeWithIssues(node.Request.Body.Raw, "body")
		return normalized, "", nil, issues
	case "file":
		if node.Request.Body.File == nil {
			return "", "", nil, nil
		}
		sourcePath, issues := normalizeWithIssues(strings.TrimSpace(node.Request.Body.File.Src), "body_file")
		return "", sourcePath, nil, issues
	case "urlencoded":
		encoded, issues := encodeKeyValues(node.Request.Body.URLEncoded)
		if encoded == "" {
			return "", "", nil, issues
		}
		return encoded, "", model.KeyValues{
			{Key: "Content-Type", Value: "application/x-www-form-urlencoded"},
		}, issues
	case "formdata":
		for _, entry := range node.Request.Body.FormData {
			if entry.Disabled {
				continue
			}
			if strings.EqualFold(strings.TrimSpace(entry.Type), "file") {
				return "", "", nil, []report.Issue{
					requestIssue(report.CodeBodyNotSupported, "form-data file entries are not supported"),
				}
			}
		}

		encoded, issues := encodeKeyValues(node.Request.Body.FormData)
		if encoded == "" {
			return "", "", nil, issues
		}
		return encoded, "", model.KeyValues{
			{Key: "Content-Type", Value: "application/x-www-form-urlencoded"},
		}, issues
	default:
		return "", "", nil, []report.Issue{
			requestIssue(report.CodeBodyNotSupported, fmt.Sprintf("body mode is not supported: %s", mode)),
		}
	}
}

func encodeKeyValues(values []ast.BodyKV) (string, []report.Issue) {
	parts := make([]string, 0, len(values))
	var issues []report.Issue

	for _, entry := range values {
		if entry.Disabled {
			continue
		}
		key := strings.TrimSpace(entry.Key)
		if key == "" {
			continue
		}

		normalizedKey, keyIssues := normalizeWithIssues(key, fmt.Sprintf("body key[%s]", key))
		issues = append(issues, keyIssues...)

		normalizedValue, valueIssues := normalizeWithIssues(entry.Value, fmt.Sprintf("body value[%s]", key))
		issues = append(issues, valueIssues...)

		encodedKey := encodeFormComponentPreserveTemplates(normalizedKey)
		encodedValue := encodeFormComponentPreserveTemplates(normalizedValue)
		parts = append(parts, encodedKey+"="+encodedValue)
	}

	if len(parts) == 0 {
		return "", issues
	}

	return strings.Join(parts, "&"), issues
}

func encodeFormComponentPreserveTemplates(input string) string {
	if input == "" {
		return ""
	}

	var builder strings.Builder
	remaining := input
	for len(remaining) > 0 {
		start := strings.Index(remaining, "{{")
		if start < 0 {
			builder.WriteString(url.QueryEscape(remaining))
			break
		}

		builder.WriteString(url.QueryEscape(remaining[:start]))
		remaining = remaining[start:]

		end := strings.Index(remaining, "}}")
		if end < 0 {
			builder.WriteString(url.QueryEscape(remaining))
			break
		}

		end += len("}}")
		builder.WriteString(remaining[:end])
		remaining = remaining[end:]
	}

	return builder.String()
}

func stripRawQuery(raw string) string {
	queryIndex := strings.IndexByte(raw, '?')
	if queryIndex < 0 {
		return raw
	}

	fragmentIndex := strings.IndexByte(raw, '#')
	if fragmentIndex >= 0 && queryIndex > fragmentIndex {
		return raw
	}
	if fragmentIndex < 0 {
		return raw[:queryIndex]
	}
	return raw[:queryIndex] + raw[fragmentIndex:]
}

func normalizeWithIssues(value string, field string) (string, []report.Issue) {
	normalized, diagnostics := template.NormalizeDetailed(value)
	return normalized, templateDiagnosticsToIssues(field, diagnostics)
}

func templateDiagnosticsToIssues(field string, tmplDiagnostics []template.Diagnostic) []report.Issue {
	if len(tmplDiagnostics) == 0 {
		return nil
	}

	issues := make([]report.Issue, 0, len(tmplDiagnostics))
	definition := diagnostics.DefinitionFor(report.CodeTemplatePlaceholderUnsupported)
	for _, diagnostic := range tmplDiagnostics {
		issues = append(issues, report.Issue{
			Code:     report.CodeTemplatePlaceholderUnsupported,
			Stage:    definition.DefaultStage,
			Severity: definition.DefaultSeverity,
			Message:  fmt.Sprintf("unsupported template placeholder in %s: %s (%s)", field, diagnostic.Placeholder, diagnostic.Reason),
		})
	}

	return issues
}

func hasAuth(node normalize.RequestNode) bool {
	raw := strings.TrimSpace(string(node.Request.Auth))
	if raw == "" || raw == "null" || raw == "{}" {
		return false
	}

	var auth struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(node.Request.Auth, &auth); err == nil {
		if strings.EqualFold(strings.TrimSpace(auth.Type), "noauth") {
			return false
		}
	}

	return true
}

func hasHeader(headers model.KeyValues, expected string) bool {
	for _, header := range headers {
		if strings.EqualFold(header.Key, expected) {
			return true
		}
	}
	return false
}
