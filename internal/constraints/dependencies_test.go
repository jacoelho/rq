package constraints

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type goListPackage struct {
	ImportPath string
	Imports    []string
}

const modulePrefix = "github.com/jacoelho/rq/internal/"

func TestRQPackagesDoNotImportPMPackages(t *testing.T) {
	t.Parallel()

	packages := goList(t, "./internal/rq/...")

	var violations []string
	for _, pkg := range packages {
		for _, imp := range pkg.Imports {
			if strings.HasPrefix(imp, modulePrefix+"pm/") {
				violations = append(violations, pkg.ImportPath+" imports "+imp)
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("found forbidden rq->pm imports:\n%s", strings.Join(violations, "\n"))
	}
}

func TestPMPackagesOnlyImportAllowedRQPackages(t *testing.T) {
	t.Parallel()

	packages := goList(t, "./internal/pm/...")
	allowedRQImports := map[string]struct{}{
		modulePrefix + "rq/model": {},
		modulePrefix + "rq/yaml":  {},
	}

	var violations []string
	for _, pkg := range packages {
		for _, imp := range pkg.Imports {
			if !strings.HasPrefix(imp, modulePrefix+"rq/") {
				continue
			}
			if _, ok := allowedRQImports[imp]; ok {
				continue
			}
			violations = append(violations, pkg.ImportPath+" imports disallowed rq package "+imp)
		}
	}

	if len(violations) > 0 {
		t.Fatalf("found forbidden pm->rq imports:\n%s", strings.Join(violations, "\n"))
	}
}

func TestPurePackagesAvoidSideEffectImports(t *testing.T) {
	t.Parallel()

	purePackages := map[string]struct{}{
		modulePrefix + "rq/model":       {},
		modulePrefix + "rq/compile":     {},
		modulePrefix + "rq/predicate":   {},
		modulePrefix + "rq/expr":        {},
		modulePrefix + "rq/number":      {},
		modulePrefix + "rq/assert":      {},
		modulePrefix + "pm/normalize":   {},
		modulePrefix + "pm/lex":         {},
		modulePrefix + "pm/parse":       {},
		modulePrefix + "pm/lower":       {},
		modulePrefix + "pm/requestmap":  {},
		modulePrefix + "pm/diagnostics": {},
	}

	forbidden := map[string]struct{}{
		"os":           {},
		"net/http":     {},
		"math/rand":    {},
		"math/rand/v2": {},
	}

	packages := goList(t, "./internal/rq/...", "./internal/pm/...")

	var violations []string
	for _, pkg := range packages {
		if _, ok := purePackages[pkg.ImportPath]; !ok {
			continue
		}
		for _, imp := range pkg.Imports {
			if _, banned := forbidden[imp]; banned {
				violations = append(violations, pkg.ImportPath+" imports forbidden package "+imp)
			}
		}
	}

	if len(violations) > 0 {
		t.Fatalf("found forbidden imports in pure packages:\n%s", strings.Join(violations, "\n"))
	}
}

func goList(t *testing.T, patterns ...string) []goListPackage {
	t.Helper()

	args := append([]string{"list", "-json"}, patterns...)
	cmd := exec.Command("go", args...)
	cmd.Dir = repoRoot(t)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("go list failed: %v\nstderr:\n%s", err, stderr.String())
	}

	decoder := json.NewDecoder(bytes.NewReader(stdout.Bytes()))
	var packages []goListPackage
	for decoder.More() {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			t.Fatalf("decode go list json: %v", err)
		}
		packages = append(packages, pkg)
	}

	return packages
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}
