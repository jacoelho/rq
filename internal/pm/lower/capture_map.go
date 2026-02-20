package lower

import (
	"strings"

	"github.com/jacoelho/rq/internal/pm/report"
	"github.com/jacoelho/rq/internal/rq/model"
)

type mapResult struct {
	mapped       bool
	requiresJSON bool
	issueCode    report.IssueCode
}

func mappedResult(requiresJSON bool) mapResult {
	return mapResult{
		mapped:       true,
		requiresJSON: requiresJSON,
	}
}

func issueResult(issueCode report.IssueCode) mapResult {
	return mapResult{
		issueCode: issueCode,
	}
}

func mapEnvironmentCapture(captures *model.Captures, line string) mapResult {
	matches := setEnvironmentPattern.FindStringSubmatch(line)
	if len(matches) != 3 {
		return mapResult{}
	}

	name := strings.TrimSpace(matches[1])
	if name == "" {
		return issueResult(report.CodeScriptJSONPathTranslationFailed)
	}

	// A later unsupported assignment must invalidate earlier mappings for this name.
	removeCaptureByName(captures, name)

	valueExpr := strings.TrimSpace(strings.TrimSuffix(matches[2], ";"))
	if isStatusExpression(valueExpr) {
		captures.Status = append(captures.Status, model.StatusCapture{Name: name})
		return mappedResult(false)
	}

	if headerName := parseHeaderExpression(valueExpr); headerName != "" {
		captures.Headers = append(captures.Headers, model.HeaderCapture{Name: name, HeaderName: headerName})
		return mappedResult(false)
	}

	path, ok := jsonExprToPath(valueExpr)
	if !ok {
		return issueResult(report.CodeScriptJSONPathTranslationFailed)
	}

	captures.JSONPath = append(captures.JSONPath, model.JSONPathCapture{
		Name: name,
		Path: path,
	})
	return mappedResult(true)
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
