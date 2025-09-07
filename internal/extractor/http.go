package extractor

import (
	"fmt"
	"net/http"
	"net/url"
)

// ExtractStatusCode extracts the HTTP status code from a response.
// Returns the numeric status code (e.g., 200, 404, 500).
// Returns ErrInvalidInput if the response is nil.
func ExtractStatusCode(resp *http.Response) (int, error) {
	if resp == nil {
		return 0, fmt.Errorf("%w: response is nil", ErrInvalidInput)
	}
	return resp.StatusCode, nil
}

// ExtractHeader extracts a specific header value from an HTTP response.
// Returns the first header value for the given name, or ErrNotFound if the header doesn't exist.
// Header name matching is case-insensitive as per HTTP specifications.
// Returns ErrInvalidInput if response is nil or headerName is empty.
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

// ExtractAllHeaders extracts all HTTP headers from a response.
// Returns a map where keys are header names and values are slices of header values.
// This handles multi-value headers correctly (e.g., Set-Cookie, Cache-Control).
// Returns an empty map if no headers are present, never returns nil.
// The returned map is a copy to prevent modification of the original headers.
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

// ExtractBody extracts the response body as a UTF-8 string.
// Suitable for text-based content like JSON, XML, HTML, or plain text.
// For binary data, use ExtractBodyBytes instead.
// Returns ErrInvalidInput if body is nil.
func ExtractBody(body []byte) (string, error) {
	if body == nil {
		return "", fmt.Errorf("%w: body is nil", ErrInvalidInput)
	}
	return string(body), nil
}

// ExtractBodyBytes returns the raw response body as bytes.
// Useful for binary data, images, or when you need the exact byte representation.
// For text content, consider using ExtractBody for UTF-8 string conversion.
// Returns ErrInvalidInput if body is nil.
func ExtractBodyBytes(body []byte) ([]byte, error) {
	if body == nil {
		return nil, fmt.Errorf("%w: body is nil", ErrInvalidInput)
	}
	return body, nil
}

// ParseFormData parses application/x-www-form-urlencoded data from raw bytes.
// Returns url.Values where keys are form field names and values are slices of field values.
// This handles multiple values for the same field name correctly.
// Users can extract individual fields using values.Get("field") for single values
// or values["field"] for all values as a slice.
// Returns an empty url.Values for empty input, never returns nil.
func ParseFormData(body []byte) (url.Values, error) {
	if len(body) == 0 {
		return url.Values{}, nil
	}

	return url.ParseQuery(string(body))
}
