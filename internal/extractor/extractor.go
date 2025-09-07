package extractor

import (
	"errors"
)

var (
	// ErrExtraction indicates failure during extraction (JSON parsing, regex compilation, etc.).
	ErrExtraction = errors.New("extraction error")

	// ErrInvalidInput indicates invalid parameters (nil responses, empty patterns, etc.).
	ErrInvalidInput = errors.New("invalid input")

	// ErrNotFound indicates requested data not found.
	ErrNotFound = errors.New("not found")
)

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}
