package jsonpath

import "errors"

var (
	// ErrSyntax indicates a JSONPath expression syntax error during compilation.
	ErrSyntax = errors.New("jsonpath: syntax error")

	// ErrNotSupported indicates a JSONPath feature is not supported in streaming mode.
	ErrNotSupported = errors.New("jsonpath: feature not supported in streaming mode")

	// ErrMalformed indicates the JSON structure is malformed or invalid.
	ErrMalformed = errors.New("jsonpath: malformed JSON structure")
)
