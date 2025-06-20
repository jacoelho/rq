package exit

import (
	"fmt"
	"io"
	"os"
)

// Result holds the output destination and exit code for program termination.
type Result struct {
	Output   io.Writer
	ExitCode int
	Message  string
}

// Print writes the result message to the configured output destination.
func (r *Result) Print() {
	fmt.Fprint(r.Output, r.Message)
}

// Success creates a successful exit result that outputs to stdout with exit code 0.
func Success(message string) *Result {
	return &Result{
		Output:   os.Stdout,
		ExitCode: 0,
		Message:  message,
	}
}

// Error creates an error exit result that outputs to stderr with exit code 1.
func Error(message string) *Result {
	return &Result{
		Output:   os.Stderr,
		ExitCode: 1,
		Message:  message,
	}
}

// Errorf creates an error exit result with formatted message.
func Errorf(format string, a ...any) *Result {
	return Error(fmt.Sprintf(format, a...))
}
