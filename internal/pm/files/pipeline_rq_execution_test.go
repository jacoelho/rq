package files

import (
	"context"
	"testing"

	"github.com/jacoelho/rq/internal/rq/config"
	"github.com/jacoelho/rq/internal/rq/execute"
	"github.com/jacoelho/rq/internal/rq/output"
)

func assertGeneratedFileRunsInRQ(t *testing.T, generatedFile string) {
	t.Helper()

	rqConfig := &config.Config{
		TestFiles:      []string{generatedFile},
		Repeat:         0,
		OutputFormat:   output.FormatText,
		RequestTimeout: config.DefaultTimeout,
	}

	runner, exitResult := execute.New(rqConfig)
	if exitResult != nil {
		t.Fatalf("execute.New() failed: %s", exitResult.Message)
	}

	exitCode := runner.Run(context.Background())
	if exitCode != 0 {
		t.Fatalf("runner.Run() exit code = %d, want 0", exitCode)
	}
}
