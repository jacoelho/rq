package exit

import (
	"bytes"
	"os"
	"testing"
)

func TestSuccess(t *testing.T) {
	message := "Operation completed successfully"
	result := Success(message)

	if result.ExitCode != 0 {
		t.Errorf("Success() ExitCode = %d, want 0", result.ExitCode)
	}

	if result.Message != message {
		t.Errorf("Success() Message = %q, want %q", result.Message, message)
	}

	if result.Output != os.Stdout {
		t.Error("Success() expected output to stdout")
	}
}

func TestError(t *testing.T) {
	message := "Operation failed"
	result := Error(message)

	if result.ExitCode != 1 {
		t.Errorf("Error() ExitCode = %d, want 1", result.ExitCode)
	}

	if result.Message != message {
		t.Errorf("Error() Message = %q, want %q", result.Message, message)
	}

	if result.Output != os.Stderr {
		t.Error("Error() expected output to stderr")
	}
}

func TestResult(t *testing.T) {
	// Test that Result struct holds values correctly
	result := &Result{
		Output:   os.Stdout,
		ExitCode: 42,
		Message:  "test message",
	}

	if result.ExitCode != 42 {
		t.Errorf("Result ExitCode = %d, want 42", result.ExitCode)
	}

	if result.Message != "test message" {
		t.Errorf("Result Message = %q, want %q", result.Message, "test message")
	}

	if result.Output != os.Stdout {
		t.Error("Result Output should be os.Stdout")
	}
}

func TestPrint(t *testing.T) {
	// Test Print method with custom output
	var buf bytes.Buffer
	result := &Result{
		Output:   &buf,
		ExitCode: 0,
		Message:  "test output",
	}

	result.Print()

	if buf.String() != "test output" {
		t.Errorf("Print() output = %q, want %q", buf.String(), "test output")
	}
}

func TestErrorf(t *testing.T) {
	result := Errorf("Operation failed: %s (code: %d)", "timeout", 500)

	if result.ExitCode != 1 {
		t.Errorf("Errorf() ExitCode = %d, want 1", result.ExitCode)
	}

	expectedMessage := "Operation failed: timeout (code: 500)"
	if result.Message != expectedMessage {
		t.Errorf("Errorf() Message = %q, want %q", result.Message, expectedMessage)
	}

	if result.Output != os.Stderr {
		t.Error("Errorf() expected output to stderr")
	}
}
