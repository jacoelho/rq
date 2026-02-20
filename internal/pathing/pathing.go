package pathing

import (
	"path/filepath"
	"strings"
)

// NormalizeInputPath trims path-like input from config/spec fields.
func NormalizeInputPath(path string) string {
	return strings.TrimSpace(path)
}

// IsAbsoluteLike reports whether the path should be treated as absolute
// regardless of host OS path semantics.
func IsAbsoluteLike(path string) bool {
	path = NormalizeInputPath(path)
	if path == "" {
		return false
	}
	if filepath.IsAbs(path) {
		return true
	}
	if strings.HasPrefix(path, `\\`) || strings.HasPrefix(path, `//`) {
		return true
	}
	if strings.HasPrefix(path, "/") {
		return true
	}
	if len(path) >= 3 && isASCIIAlpha(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
		return true
	}

	return false
}

// ShouldRebaseBodyFile reports whether a body_file path should be rebased from
// input file location to output file location.
func ShouldRebaseBodyFile(bodyFile string) bool {
	bodyFile = NormalizeInputPath(bodyFile)
	if bodyFile == "" || IsAbsoluteLike(bodyFile) {
		return false
	}
	if hasTemplateMarkers(bodyFile) && strings.HasPrefix(bodyFile, "{{") {
		return false
	}

	return true
}

// RebaseBodyFilePath rewrites relative body_file paths from the source
// collection location to the generated rq file location.
func RebaseBodyFilePath(bodyFile string, inputFile string, outputFile string) string {
	bodyFile = NormalizeInputPath(bodyFile)
	if !ShouldRebaseBodyFile(bodyFile) {
		return bodyFile
	}

	inputDir := filepath.Dir(inputFile)
	sourceAbsolute := filepath.Clean(filepath.Join(inputDir, bodyFile))
	outputDir := filepath.Dir(outputFile)

	relative, err := filepath.Rel(outputDir, sourceAbsolute)
	if err != nil || NormalizeInputPath(relative) == "" {
		return filepath.ToSlash(sourceAbsolute)
	}

	return filepath.ToSlash(relative)
}

// ResolveBodyFilePath resolves a possibly-relative body_file value using the
// step base directory while preserving absolute-like paths.
func ResolveBodyFilePath(filePath string, baseDir string) string {
	filePath = NormalizeInputPath(filePath)
	if filePath == "" {
		return ""
	}
	if IsAbsoluteLike(filePath) || NormalizeInputPath(baseDir) == "" {
		return filePath
	}

	return filepath.Join(baseDir, filePath)
}

func hasTemplateMarkers(path string) bool {
	return strings.Contains(path, "{{") || strings.Contains(path, "}}")
}

func isASCIIAlpha(char byte) bool {
	return (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z')
}
