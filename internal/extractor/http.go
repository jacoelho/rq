package extractor

import (
	"fmt"
	"net/http"
	"net/url"
)

func ExtractStatusCode(resp *http.Response) (int, error) {
	if resp == nil {
		return 0, fmt.Errorf("%w: response is nil", ErrInvalidInput)
	}
	return resp.StatusCode, nil
}

// ExtractHeader matching is case-insensitive per HTTP specifications.
func ExtractHeader(resp *http.Response, headerName string) (string, error) {
	if resp == nil {
		return "", fmt.Errorf("%w: response is nil", ErrInvalidInput)
	}

	if headerName == "" {
		return "", fmt.Errorf("%w: header name cannot be empty", ErrInvalidInput)
	}

	headerValue := resp.Header.Get(headerName)
	if headerValue == "" {
		return "", ErrNotFound
	}

	return headerValue, nil
}

// ExtractAllHeaders handles multi-value headers and returns a defensive copy.
func ExtractAllHeaders(resp *http.Response) (map[string][]string, error) {
	if resp == nil {
		return nil, fmt.Errorf("%w: response is nil", ErrInvalidInput)
	}

	if resp.Header == nil {
		return make(map[string][]string), nil
	}

	// Create a copy to avoid returning the original map
	headers := make(map[string][]string, len(resp.Header))
	for k, v := range resp.Header {
		headers[k] = append([]string{}, v...)
	}

	return headers, nil
}

// ExtractBody is suitable for text content; use ExtractBodyBytes for binary data.
func ExtractBody(body []byte) (string, error) {
	if body == nil {
		return "", fmt.Errorf("%w: body is nil", ErrInvalidInput)
	}
	return string(body), nil
}

// ExtractBodyBytes preserves exact byte representation for binary data.
func ExtractBodyBytes(body []byte) ([]byte, error) {
	if body == nil {
		return nil, fmt.Errorf("%w: body is nil", ErrInvalidInput)
	}
	return body, nil
}

// ParseFormData handles multiple values per field; use Get() for single values.
func ParseFormData(body []byte) (url.Values, error) {
	if len(body) == 0 {
		return url.Values{}, nil
	}

	return url.ParseQuery(string(body))
}
