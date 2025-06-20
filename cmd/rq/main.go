package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/jacoelho/rq/internal/config"
	"github.com/jacoelho/rq/internal/runner"
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

	r, exitResult := runner.New(cfg)
	if exitResult != nil {
		exitResult.Print()
		return exitResult.ExitCode
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	return r.Run(ctx)
}
