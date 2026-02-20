package files

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/jacoelho/rq/internal/pm/config"
	"github.com/jacoelho/rq/internal/pm/report"
)

func TestRunMatchesGoldenOutput(t *testing.T) {
	t.Parallel()

	fixtures := []string{
		"basic",
	}

	for _, fixture := range fixtures {
		fixture := fixture
		t.Run(fixture, func(t *testing.T) {
			t.Parallel()

			fixtureDir := filepath.Join("testdata", "golden", fixture)
			outputDir := filepath.Join(t.TempDir(), "out")

			summary, err := Run(config.Config{
				InputFile:    filepath.Join(fixtureDir, "collection.json"),
				OutputDir:    outputDir,
				Overwrite:    false,
				DryRun:       false,
				ReportFormat: report.FormatJSON,
			})
			if err != nil {
				t.Fatalf("Run() error = %v", err)
			}
			if summary.HasErrors() {
				t.Fatalf("Run() produced fatal diagnostics for golden fixture %q: %+v", fixture, summary)
			}

			got := readYAMLTree(t, outputDir)
			want := readYAMLTree(t, filepath.Join(fixtureDir, "expected"))
			assertYAMLTreesEqual(t, got, want)
		})
	}
}

func TestRunProducesByteStableOutput(t *testing.T) {
	t.Parallel()

	fixtureDir := filepath.Join("testdata", "golden", "basic")
	runOnce := func(t *testing.T, outputDir string) map[string]string {
		t.Helper()

		summary, err := Run(config.Config{
			InputFile:    filepath.Join(fixtureDir, "collection.json"),
			OutputDir:    outputDir,
			Overwrite:    false,
			DryRun:       false,
			ReportFormat: report.FormatJSON,
		})
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
		if summary.HasErrors() {
			t.Fatalf("Run() produced fatal diagnostics: %+v", summary)
		}

		return readYAMLTree(t, outputDir)
	}

	first := runOnce(t, filepath.Join(t.TempDir(), "run-1"))
	second := runOnce(t, filepath.Join(t.TempDir(), "run-2"))
	assertYAMLTreesEqual(t, first, second)
}

func readYAMLTree(t *testing.T, root string) map[string]string {
	t.Helper()

	tree := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if filepath.Ext(entry.Name()) != ".yaml" {
			return nil
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		payload, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		tree[filepath.ToSlash(rel)] = string(payload)
		return nil
	})
	if err != nil {
		t.Fatalf("readYAMLTree(%q): %v", root, err)
	}

	return tree
}

func assertYAMLTreesEqual(t *testing.T, got map[string]string, want map[string]string) {
	t.Helper()

	var diffs []string
	for path, wantContent := range want {
		gotContent, ok := got[path]
		if !ok {
			diffs = append(diffs, fmt.Sprintf("missing file: %s", path))
			continue
		}
		if gotContent != wantContent {
			diffs = append(diffs, fmt.Sprintf("content mismatch for %s", path))
		}
	}

	for path := range got {
		if _, ok := want[path]; ok {
			continue
		}
		diffs = append(diffs, fmt.Sprintf("unexpected file: %s", path))
	}

	if len(diffs) == 0 {
		return
	}

	sort.Strings(diffs)
	t.Fatalf("YAML output tree mismatch:\n%s", strings.Join(diffs, "\n"))
}
