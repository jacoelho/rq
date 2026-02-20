package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jacoelho/rq/internal/rq/config"
	"github.com/jacoelho/rq/internal/rq/execute"
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	cfg, exitResult := config.Parse(os.Args)
	if exitResult != nil {
		exitResult.Print()
		return exitResult.ExitCode
	}

	r, exitResult := execute.New(cfg)
	if exitResult != nil {
		exitResult.Print()
		return exitResult.ExitCode
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return r.Run(ctx)
}
