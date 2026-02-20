package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/jacoelho/rq/internal/pm/config"
	"github.com/jacoelho/rq/internal/pm/files"
)

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	cfg, err := config.Parse(args)
	if err != nil {
		if errors.Is(err, config.ErrHelp) {
			fmt.Fprintln(os.Stdout, config.Usage())
			return 0
		}

		fmt.Fprintf(os.Stderr, "Error: %v\n\n%s\n", err, config.Usage())
		return 1
	}

	summary, err := files.Run(*cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}

	if err := summary.Write(os.Stdout, cfg.ReportFormat); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to write report: %v\n", err)
		return 1
	}

	if summary.HasErrors() {
		return 1
	}

	return 0
}
