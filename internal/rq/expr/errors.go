package expr

import (
	"errors"
	"fmt"
)

// ErrInvalidExpression indicates expression parsing or evaluation failures.
var ErrInvalidExpression = errors.New("invalid expression")

func expressionError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidExpression, fmt.Sprintf(format, args...))
}
