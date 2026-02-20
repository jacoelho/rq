package naming

import (
	"path/filepath"
	"testing"
)

func TestSanitizeSegment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  string
	}{
		{input: "  My Request  ", want: "my-request"},
		{input: "A/B/C", want: "a-b-c"},
		{input: "***", want: "request"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()

			got := SanitizeSegment(tt.input)
			if got != tt.want {
				t.Fatalf("SanitizeSegment(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestPlannerNext(t *testing.T) {
	t.Parallel()

	planner := NewPlanner()

	first := planner.Next([]string{"Audit", "Entries"}, "Get Entry", "GET")
	second := planner.Next([]string{"Audit", "Entries"}, "Get Entry", "GET")
	third := planner.Next([]string{"Audit", "Entries"}, "Get Entry", "GET")
	fourth := planner.Next([]string{"Audit", "Entries"}, "Get Entry", "POST")

	if first != filepath.FromSlash("audit/entries/get-entry-get.yaml") {
		t.Fatalf("first path = %q", first)
	}
	if second != filepath.FromSlash("audit/entries/get-entry-get-1.yaml") {
		t.Fatalf("second path = %q", second)
	}
	if third != filepath.FromSlash("audit/entries/get-entry-get-2.yaml") {
		t.Fatalf("third path = %q", third)
	}
	if fourth != filepath.FromSlash("audit/entries/get-entry-post.yaml") {
		t.Fatalf("fourth path = %q", fourth)
	}
}
