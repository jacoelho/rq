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

func (r *Result) Print() {
	fmt.Fprint(r.Output, r.Message)
}

func Success(message string) *Result {
	return &Result{
		Output:   os.Stdout,
		ExitCode: 0,
		Message:  message,
	}
}

func Error(message string) *Result {
	return &Result{
		Output:   os.Stderr,
		ExitCode: 1,
		Message:  message,
	}
}

func Errorf(format string, a ...any) *Result {
	return Error(fmt.Sprintf(format, a...))
}
