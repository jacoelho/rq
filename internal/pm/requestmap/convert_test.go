package requestmap

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/pm/ast"
	"github.com/jacoelho/rq/internal/pm/normalize"
	"github.com/jacoelho/rq/internal/pm/report"
)

func TestRequestBasicMapping(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name:       "Get user",
		FolderPath: []string{"Users"},
		Request: ast.Request{
			Method: "get",
			URL:    ast.URLValue{Raw: "https://api.example.com/users?x=1"},
			URLObject: &ast.URLObject{
				Query: []ast.QueryParam{{Key: "user_id", Value: "{{user_id}}"}},
			},
			Header: []ast.Header{{Key: "Authorization", Value: "Bearer {{token}}"}},
			Body:   &ast.Body{Mode: "raw", Raw: `{"id":"{{user_id}}"}`},
		},
		Events: []ast.Event{{
			Listen: "test",
			Script: ast.Script{Exec: []string{
				`tests["response code is 200"] = responseCode.code === 200;`,
				`var json = JSON.parse(responseBody);`,
			}},
		}},
	}

	result := Request(node)
	if result.Converted {
		t.Fatal("expected request conversion to fail on unsupported script lines")
	}

	if result.Step.Method != "GET" {
		t.Fatalf("method = %s", result.Step.Method)
	}
	if result.Step.URL != "https://api.example.com/users" {
		t.Fatalf("url = %s", result.Step.URL)
	}
	if got, _ := result.Step.Query.Get("user_id"); got != "{{.user_id}}" {
		t.Fatalf("query user_id = %q", got)
	}
	if got, _ := result.Step.Headers.Get("Authorization"); got != "Bearer {{.token}}" {
		t.Fatalf("authorization header = %q", got)
	}
	if result.Step.Body != `{"id":"{{.user_id}}"}` {
		t.Fatalf("body = %q", result.Step.Body)
	}
	if len(result.Step.Asserts.Status) != 1 {
		t.Fatalf("status asserts len = %d", len(result.Step.Asserts.Status))
	}
	if len(result.Step.Asserts.JSONPath) != 0 {
		t.Fatalf("expected no jsonpath asserts, got %+v", result.Step.Asserts.JSONPath)
	}
	if !hasIssue(result.Issues, report.CodeScriptExpressionNotSupported) {
		t.Fatalf("expected unsupported script expression issue, got %+v", result.Issues)
	}
	if !hasIssue(result.Issues, report.CodeTestNotMapped) {
		t.Fatalf("expected aggregate not-mapped issue, got %+v", result.Issues)
	}
}

func TestRequestBuildsURLWithProtocolFromHostPath(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Users",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Protocol: "http",
				Host:     []string{"api", "example", "com"},
				Path:     []string{"users"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "http://api.example.com/users" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestBuildsURLWithPortFromHostPath(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Health",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Protocol: "http",
				Host:     []string{"localhost"},
				Port:     "8080",
				Path:     []string{"health"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "http://localhost:8080/health" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestBuildsURLWithPortFromURLObjectFallback(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Health",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{},
			URLObject: &ast.URLObject{
				Protocol: "http",
				Host:     []string{"localhost"},
				Port:     "8080",
				Path:     []string{"health"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "http://localhost:8080/health" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestBuildsURLWithPortBeforeHostEmbeddedPath(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Users",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Host: []string{"api.example.com/v1"},
				Port: "8080",
				Path: []string{"users"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com:8080/v1/users" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestKeepsExistingPortWhenHostHasPath(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Users",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Host: []string{"https://api.example.com:9090/v1"},
				Port: "8080",
				Path: []string{"users"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com:9090/v1/users" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestBuildsURLWithDefaultSchemeFromHostPath(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Users",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Host: []string{"api", "example", "com"},
				Path: []string{"users"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com/users" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestUnsupportedFormDataFile(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Upload",
		Request: ast.Request{
			Method: "POST",
			URL:    ast.URLValue{Raw: "https://api.example.com/upload"},
			Body: &ast.Body{
				Mode: "formdata",
				FormData: []ast.BodyKV{{
					Key:  "file",
					Type: "file",
				}},
			},
		},
	}

	result := Request(node)
	if result.Converted {
		t.Fatal("expected request conversion to fail on unsupported form-data file mode")
	}
	if result.Step.Body != "" {
		t.Fatalf("expected empty body, got %q", result.Step.Body)
	}
	if !hasIssue(result.Issues, report.CodeBodyNotSupported) {
		t.Fatalf("expected body unsupported issue, got %+v", result.Issues)
	}
}

func TestRequestFileBodyMapping(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Upload",
		Request: ast.Request{
			Method: "PUT",
			URL:    ast.URLValue{Raw: "https://api.example.com/upload"},
			Body: &ast.Body{
				Mode: "file",
				File: &ast.BodyFile{Src: "{{upload_path}}"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.Body != "" {
		t.Fatalf("expected empty inline body, got %q", result.Step.Body)
	}
	if result.Step.BodyFile != "{{.upload_path}}" {
		t.Fatalf("body_file = %q", result.Step.BodyFile)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
}

func TestRequestFileBodyEmptySource(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Upload",
		Request: ast.Request{
			Method: "PUT",
			URL:    ast.URLValue{Raw: "https://api.example.com/upload"},
			Body: &ast.Body{
				Mode: "file",
				File: &ast.BodyFile{Src: ""},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.Body != "" {
		t.Fatalf("expected empty body, got %q", result.Step.Body)
	}
	if result.Step.BodyFile != "" {
		t.Fatalf("expected empty body_file, got %q", result.Step.BodyFile)
	}
	if len(result.Issues) != 0 {
		t.Fatalf("expected no issues, got %+v", result.Issues)
	}
}

func TestRequestInvalidMethod(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Bad",
		Request: ast.Request{
			Method: "TRACE",
			URL:    ast.URLValue{Raw: "https://api.example.com"},
		},
	}

	result := Request(node)
	if result.Converted {
		t.Fatal("expected conversion to fail")
	}
	if !hasIssue(result.Issues, report.CodeInvalidRequestShape) {
		t.Fatalf("expected invalid request issue, got %+v", result.Issues)
	}
}

func TestRequestAuthIssue(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Auth",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{Raw: "https://api.example.com"},
			Auth:   json.RawMessage(`{"type":"basic"}`),
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if !hasIssue(result.Issues, report.CodeAuthNotMapped) {
		t.Fatalf("expected auth issue, got %+v", result.Issues)
	}
}

func TestRequestNoAuthDoesNotCreateAuthIssue(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "No auth",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{Raw: "https://api.example.com"},
			Auth:   json.RawMessage(`{"type":"noauth"}`),
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if hasIssue(result.Issues, report.CodeAuthNotMapped) {
		t.Fatalf("did not expect auth issue, got %+v", result.Issues)
	}
}

func TestRequestPreservesURLTemplatePathWhenStrippingQuery(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Raw: "https://api.example.com/users/{{user_id}}?x=1",
			},
			URLObject: &ast.URLObject{
				Query: []ast.QueryParam{
					{Key: "expand", Value: "{{token}}"},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com/users/{{.user_id}}" {
		t.Fatalf("url = %s", result.Step.URL)
	}
	if got, _ := result.Step.Query.Get("expand"); got != "{{.token}}" {
		t.Fatalf("query expand = %q", got)
	}
}

func TestRequestStripRawQueryKeepsFragment(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Raw: "https://api.example.com/users/{{user_id}}?x=1#section",
			},
			URLObject: &ast.URLObject{
				Query: []ast.QueryParam{
					{Key: "expand", Value: "true"},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com/users/{{.user_id}}#section" {
		t.Fatalf("url = %s", result.Step.URL)
	}
}

func TestRequestStripRawQueryWhenStructuredQueryAllDisabled(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Raw: "https://api.example.com/users/{{user_id}}?x=1#section",
			},
			URLObject: &ast.URLObject{
				Query: []ast.QueryParam{
					{Key: "x", Value: "1", Disabled: true},
					{Key: "y", Value: "2", Disabled: true},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com/users/{{.user_id}}#section" {
		t.Fatalf("url = %s", result.Step.URL)
	}
	if len(result.Step.Query) != 0 {
		t.Fatalf("expected empty query map, got %+v", result.Step.Query)
	}
}

func TestRequestKeepsRawQueryWithoutStructuredQuery(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Raw: "https://api.example.com/users?x=1&y=2",
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.URL != "https://api.example.com/users?x=1&y=2" {
		t.Fatalf("url = %s", result.Step.URL)
	}
	if len(result.Step.Query) != 0 {
		t.Fatalf("expected empty query map, got %+v", result.Step.Query)
	}
}

func TestRequestURLencodedBodyPreservesTemplates(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Submit",
		Request: ast.Request{
			Method: "POST",
			URL:    ast.URLValue{Raw: "https://api.example.com/submit"},
			Body: &ast.Body{
				Mode: "urlencoded",
				URLEncoded: []ast.BodyKV{
					{Key: "user_id", Value: "{{user_id}}"},
					{Key: "name", Value: "John Doe"},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.Body != "user_id={{.user_id}}&name=John+Doe" {
		t.Fatalf("body = %q", result.Step.Body)
	}
	if strings.Contains(result.Step.Body, "%7B%7B") {
		t.Fatalf("body should preserve templates, got %q", result.Step.Body)
	}
}

func TestRequestFormBodyPreservesTemplates(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Submit",
		Request: ast.Request{
			Method: "POST",
			URL:    ast.URLValue{Raw: "https://api.example.com/submit"},
			Body: &ast.Body{
				Mode: "formdata",
				FormData: []ast.BodyKV{
					{Key: "note", Value: "hello {{name}}/x", Type: "text"},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if result.Step.Body != "note=hello+{{.name}}%2Fx" {
		t.Fatalf("body = %q", result.Step.Body)
	}
	if strings.Contains(result.Step.Body, "%7B%7B") {
		t.Fatalf("body should preserve templates, got %q", result.Step.Body)
	}
}

func TestRequestReportsUnsupportedTemplatePlaceholder(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{Raw: "https://api.example.com/users"},
			Header: []ast.Header{
				{Key: "Authorization", Value: "Bearer {{base-url}}"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if !hasIssue(result.Issues, report.CodeTemplatePlaceholderUnsupported) {
		t.Fatalf("expected unsupported template placeholder issue, got %+v", result.Issues)
	}
	if got, _ := result.Step.Headers.Get("Authorization"); got != "Bearer {{base-url}}" {
		t.Fatalf("authorization header = %q", got)
	}
}

func TestRequestMapsKnownDynamicPlaceholders(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Get user",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{Raw: "https://api.example.com/users"},
			Header: []ast.Header{
				{Key: "X-Timestamp", Value: "{{$timestamp}}"},
				{Key: "X-Trace", Value: "{{$guid}}"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if got, _ := result.Step.Headers.Get("X-Timestamp"); got != "{{timestamp}}" {
		t.Fatalf("X-Timestamp header = %q", got)
	}
	if got, _ := result.Step.Headers.Get("X-Trace"); got != "{{uuidv4}}" {
		t.Fatalf("X-Trace header = %q", got)
	}
	if hasIssue(result.Issues, report.CodeTemplatePlaceholderUnsupported) {
		t.Fatalf("did not expect unsupported template placeholder issue: %+v", result.Issues)
	}
}

func TestRequestCanonicalizesAndPreservesRepeatedHeaderKeys(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Headers",
		Request: ast.Request{
			Method: "GET",
			URL:    ast.URLValue{Raw: "https://api.example.com/users"},
			Header: []ast.Header{
				{Key: "x-token", Value: "first"},
				{Key: "X-Token", Value: "second"},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if len(result.Step.Headers) != 2 {
		t.Fatalf("headers len = %d, want 2 (headers=%+v)", len(result.Step.Headers), result.Step.Headers)
	}
	for _, header := range result.Step.Headers {
		if header.Key != "X-Token" {
			t.Fatalf("header key = %q, want canonical X-Token", header.Key)
		}
	}
	if result.Step.Headers[0].Value != "first" {
		t.Fatalf("headers[0].Value = %q, want first", result.Step.Headers[0].Value)
	}
	if result.Step.Headers[1].Value != "second" {
		t.Fatalf("headers[1].Value = %q, want second", result.Step.Headers[1].Value)
	}
}

func TestRequestPreservesRepeatedQueryKeys(t *testing.T) {
	t.Parallel()

	node := normalize.RequestNode{
		Name: "Users",
		Request: ast.Request{
			Method: "GET",
			URL: ast.URLValue{
				Raw: "https://api.example.com/users",
			},
			URLObject: &ast.URLObject{
				Query: []ast.QueryParam{
					{Key: "tag", Value: "alpha"},
					{Key: "tag", Value: "beta"},
				},
			},
		},
	}

	result := Request(node)
	if !result.Converted {
		t.Fatal("expected request to be converted")
	}
	if len(result.Step.Query) != 2 {
		t.Fatalf("query len = %d, want 2 (query=%+v)", len(result.Step.Query), result.Step.Query)
	}
	if result.Step.Query[0].Key != "tag" || result.Step.Query[0].Value != "alpha" {
		t.Fatalf("query[0] = %+v, want tag=alpha", result.Step.Query[0])
	}
	if result.Step.Query[1].Key != "tag" || result.Step.Query[1].Value != "beta" {
		t.Fatalf("query[1] = %+v, want tag=beta", result.Step.Query[1])
	}
}

func hasIssue(issues []report.Issue, code report.IssueCode) bool {
	for _, issue := range issues {
		if issue.Code == code {
			return true
		}
	}
	return false
}
