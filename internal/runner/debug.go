package runner

import (
	"fmt"
	"net/http"

	"github.com/jacoelho/rq/internal/sanitizer"
)

// redactValues extracts all values that should be redacted from captures and static secrets.
func redactValues(captures map[string]CaptureValue, staticSecrets map[string]any) []any {
	var values []any

	for _, v := range staticSecrets {
		values = append(values, v)
	}

	for _, v := range captures {
		if v.Redact {
			values = append(values, v.Value)
		}
	}
	return values
}

// debugRequest outputs detailed request information when debug mode is enabled.
func (r *Runner) debugRequest(req *http.Request, redactValues []any) {
	reqDump, err := sanitizer.DumpRequestRedacted(req, redactValues, r.config.SecretSalt)
	if err != nil {
		fmt.Printf("Error dumping request: %v\n", err)
		return
	}

	if err := r.formatter.Debug("REQUEST", reqDump); err != nil {
		fmt.Printf("Error formatting debug request: %v\n", err)
	}
}

// debugResponse outputs detailed response information when debug mode is enabled.
func (r *Runner) debugResponse(resp *http.Response, body []byte, redactValues []any) {
	respDump, err := sanitizer.DumpResponseRedacted(resp, body, redactValues, r.config.SecretSalt)
	if err != nil {
		fmt.Printf("Error dumping response: %v\n", err)
		return
	}

	if err := r.formatter.Debug("RESPONSE", respDump); err != nil {
		fmt.Printf("Error formatting debug response: %v\n", err)
	}
}
