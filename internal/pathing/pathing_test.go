package pathing

import (
	"path/filepath"
	"testing"
)

func TestIsAbsoluteLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "empty",
			path: "",
			want: false,
		},
		{
			name: "relative",
			path: "payload.bin",
			want: false,
		},
		{
			name: "posix absolute",
			path: "/tmp/payload.bin",
			want: true,
		},
		{
			name: "windows drive backslash",
			path: `C:\tmp\payload.bin`,
			want: true,
		},
		{
			name: "windows drive slash",
			path: `C:/tmp/payload.bin`,
			want: true,
		},
		{
			name: "unc backslash",
			path: `\\server\share\payload.bin`,
			want: true,
		},
		{
			name: "unc slash",
			path: `//server/share/payload.bin`,
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAbsoluteLike(tt.path); got != tt.want {
				t.Fatalf("IsAbsoluteLike(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestShouldRebaseBodyFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		bodyFile string
		want     bool
	}{
		{
			name:     "empty",
			bodyFile: "",
			want:     false,
		},
		{
			name:     "relative",
			bodyFile: "payload.bin",
			want:     true,
		},
		{
			name:     "template prefix",
			bodyFile: "{{.payload_dir}}/payload.bin",
			want:     false,
		},
		{
			name:     "absolute",
			bodyFile: "/tmp/payload.bin",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldRebaseBodyFile(tt.bodyFile); got != tt.want {
				t.Fatalf("ShouldRebaseBodyFile(%q) = %v, want %v", tt.bodyFile, got, tt.want)
			}
		})
	}
}

func TestRebaseBodyFilePath(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	inputFile := filepath.Join(tempDir, "in", "collection.json")
	outputFile := filepath.Join(tempDir, "out", "request.yaml")
	bodyFile := "payloads/{{.name}}.bin"

	got := RebaseBodyFilePath(bodyFile, inputFile, outputFile)

	sourceAbsolute := filepath.Clean(filepath.Join(filepath.Dir(inputFile), bodyFile))
	want, err := filepath.Rel(filepath.Dir(outputFile), sourceAbsolute)
	if err != nil {
		t.Fatalf("filepath.Rel() error = %v", err)
	}
	want = filepath.ToSlash(want)
	if got != want {
		t.Fatalf("RebaseBodyFilePath(%q) = %q, want %q", bodyFile, got, want)
	}
}

func TestRebaseBodyFilePathPreservesAbsoluteLikeAndTemplatePrefix(t *testing.T) {
	t.Parallel()

	unchanged := []string{
		"/tmp/payload.bin",
		`C:\tmp\payload.bin`,
		`C:/tmp/payload.bin`,
		`\\server\share\payload.bin`,
		"{{.payload_dir}}/payload.bin",
	}

	for _, bodyFile := range unchanged {
		bodyFile := bodyFile
		t.Run(bodyFile, func(t *testing.T) {
			t.Parallel()

			got := RebaseBodyFilePath(bodyFile, "/input/collection.json", "/out/request.yaml")
			if got != bodyFile {
				t.Fatalf("RebaseBodyFilePath(%q) = %q, want unchanged", bodyFile, got)
			}
		})
	}
}

func TestResolveBodyFilePath(t *testing.T) {
	t.Parallel()

	baseDir := "/spec"
	tests := []struct {
		name    string
		path    string
		baseDir string
		want    string
	}{
		{
			name:    "empty",
			path:    "",
			baseDir: baseDir,
			want:    "",
		},
		{
			name:    "relative with base",
			path:    "payload.bin",
			baseDir: baseDir,
			want:    filepath.Join(baseDir, "payload.bin"),
		},
		{
			name:    "relative without base",
			path:    "payload.bin",
			baseDir: "",
			want:    "payload.bin",
		},
		{
			name:    "posix absolute",
			path:    "/tmp/payload.bin",
			baseDir: baseDir,
			want:    "/tmp/payload.bin",
		},
		{
			name:    "windows drive absolute",
			path:    `C:/tmp/payload.bin`,
			baseDir: baseDir,
			want:    `C:/tmp/payload.bin`,
		},
		{
			name:    "unc absolute",
			path:    `\\server\share\payload.bin`,
			baseDir: baseDir,
			want:    `\\server\share\payload.bin`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ResolveBodyFilePath(tt.path, tt.baseDir); got != tt.want {
				t.Fatalf("ResolveBodyFilePath(%q, %q) = %q, want %q", tt.path, tt.baseDir, got, tt.want)
			}
		})
	}
}
