package sanitizer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
)

// DumpRequestRedacted dumps an HTTP request with secrets redacted.
func DumpRequestRedacted(req *http.Request, redactValues []any, salt string) ([]byte, error) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return nil, fmt.Errorf("failed to dump request: %w", err)
	}

	return redactBody(dump, redactValues, salt), nil
}

// DumpResponseRedacted dumps an HTTP response with secrets redacted.
func DumpResponseRedacted(resp *http.Response, body []byte, redactValues []any, salt string) ([]byte, error) {
	clone := new(http.Response)
	*clone = *resp
	clone.Body = io.NopCloser(bytes.NewReader(body))

	dump, err := httputil.DumpResponse(clone, true)
	if err != nil {
		return nil, fmt.Errorf("failed to dump response: %w", err)
	}

	return redactBody(dump, redactValues, salt), nil
}

// redactBody replaces secret values in the given data with [S256:hash].
func redactBody(data []byte, redactValues []any, salt string) []byte {
	if len(redactValues) == 0 || len(data) == 0 {
		return data
	}

	var out []byte
	changed := false

	for _, v := range redactValues {
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		needle := []byte(s)

		if !bytes.Contains(data, needle) {
			continue
		}
		if !changed {
			out = make([]byte, len(data))
			copy(out, data)
			changed = true
		}

		redactedValue := hashToken(s, salt)
		out = bytes.ReplaceAll(out, needle, redactedValue)
	}
	if changed {
		return out
	}
	return data
}

func hashToken(secret, salt string) []byte {
	sum := sha256.Sum256([]byte(salt + secret))
	hex := hex.EncodeToString(sum[:8])
	return []byte("[S256:" + hex + "]")
}
