package extractor

import (
	"errors"
)

// Common sentinel errors for extraction operations.
// These errors support error wrapping and can be checked using errors.Is().
var (
	// ErrExtraction indicates a failure during the extraction process itself.
	// This includes JSON parsing errors, regex compilation failures, etc.
	ErrExtraction = errors.New("extraction error")

	// ErrInvalidInput indicates invalid parameters were provided to an extraction function.
	// This includes nil responses, empty patterns, negative indices, etc.
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotFound indicates the requested data was not found in the source.
	// This is returned when JSONPath queries return no results, headers don't exist,
	// regex patterns don't match, etc. Use IsNotFound() for convenient checking.
	ErrNotFound = errors.New("not found")
)

// IsNotFound checks if an error indicates that requested data was not found.
// Returns true for ErrNotFound and any error that wraps it.
// This is the recommended way to check for "not found" conditions instead of
// direct error comparison, as it supports error wrapping.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
