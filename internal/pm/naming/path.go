package naming

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var nonAlnum = regexp.MustCompile(`[^a-z0-9]+`)

// Planner generates deterministic relative output paths.
type Planner struct {
	used map[string]int
}

// NewPlanner creates a path planner with collision tracking.
func NewPlanner() *Planner {
	return &Planner{used: make(map[string]int)}
}

// Next returns the next unique relative file path for the request.
func (p *Planner) Next(folderPath []string, requestName string, method string) string {
	sanitizedFolders := make([]string, 0, len(folderPath))
	for _, segment := range folderPath {
		sanitizedFolders = append(sanitizedFolders, SanitizeSegment(segment))
	}

	dir := filepath.Join(sanitizedFolders...)
	base := buildBaseName(requestName, method)
	key := dir + "\x00" + base

	p.used[key]++
	count := p.used[key]

	filename := base
	if count > 1 {
		filename = fmt.Sprintf("%s-%d", base, count-1)
	}
	filename += ".yaml"

	if dir == "" {
		return filename
	}

	return filepath.Join(dir, filename)
}

// SanitizeSegment converts arbitrary names into deterministic file-safe slugs.
func SanitizeSegment(input string) string {
	slug := strings.ToLower(strings.TrimSpace(input))
	slug = nonAlnum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if slug == "" {
		return "request"
	}
	return slug
}

func buildBaseName(requestName, method string) string {
	namePart := SanitizeSegment(requestName)
	methodPart := SanitizeSegment(method)
	if methodPart == "" {
		methodPart = "request"
	}
	return fmt.Sprintf("%s-%s", namePart, methodPart)
}
