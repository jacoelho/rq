package sanitizer

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"sort"
)

// DumpRequestRedacted dumps an HTTP request with secrets redacted.
func DumpRequestRedacted(req *http.Request, redactValues []any, salt string) ([]byte, error) {
	dump, err := httputil.DumpRequestOut(req, true)
	if err != nil {
		return nil, fmt.Errorf("failed to dump request: %w", err)
	}

	return redactOutput(dump, redactValues, salt), nil
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

	return redactOutput(dump, redactValues, salt), nil
}

// redactOutput replaces secret values in the given data with [S256:hash].
func redactOutput(data []byte, redactValues []any, salt string) []byte {
	if len(redactValues) == 0 || len(data) == 0 {
		return data
	}

	targets := buildRedactionTargets(redactValues, salt)
	if len(targets) == 0 {
		return data
	}

	var out []byte
	for index := 0; index < len(data); {
		target := matchRedactionTargetAt(data, index, targets)
		if target == nil {
			if out != nil {
				out = append(out, data[index])
			}
			index++
			continue
		}

		if out == nil {
			out = make([]byte, 0, len(data))
			out = append(out, data[:index]...)
		}

		out = append(out, target.Replacement...)
		index += len(target.Needle)
	}

	if out != nil {
		return out
	}

	return data
}

func matchRedactionTargetAt(data []byte, index int, targets []redactionTarget) *redactionTarget {
	remaining := data[index:]
	for targetIndex := range targets {
		target := &targets[targetIndex]
		if len(remaining) < len(target.Needle) {
			continue
		}
		if bytes.Equal(remaining[:len(target.Needle)], target.Needle) {
			return target
		}
	}

	return nil
}

func hashToken(secret, salt string) []byte {
	sum := sha256.Sum256([]byte(salt + secret))
	hex := hex.EncodeToString(sum[:8])
	return []byte("[S256:" + hex + "]")
}

type redactionTarget struct {
	Secret      string
	Needle      []byte
	Replacement []byte
}

func buildRedactionTargets(redactValues []any, salt string) []redactionTarget {
	unique := make(map[string]struct{}, len(redactValues))
	for _, value := range redactValues {
		secret, ok := value.(string)
		if !ok || secret == "" {
			continue
		}
		unique[secret] = struct{}{}
	}

	targets := make([]redactionTarget, 0, len(unique))
	for secret := range unique {
		targets = append(targets, redactionTarget{
			Secret:      secret,
			Needle:      []byte(secret),
			Replacement: hashToken(secret, salt),
		})
	}

	sort.Slice(targets, func(i, j int) bool {
		leftLen := len(targets[i].Needle)
		rightLen := len(targets[j].Needle)
		if leftLen != rightLen {
			return leftLen > rightLen
		}

		return targets[i].Secret < targets[j].Secret
	})

	return targets
}
