package formatter

import (
	"github.com/jacoelho/rq/internal/results"
)

// Formatter defines the interface for different output formats.
// Implementations are responsible for determining the output device (stdout, file, etc.).
type Formatter interface {
	// Format automatically determines whether to format as single or aggregated results
	// based on the number of summaries provided. The formatter decides where to output.
	Format(summaries ...*results.Summary) error
}
