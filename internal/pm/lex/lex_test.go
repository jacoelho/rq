package lex

import "testing"

func TestScript(t *testing.T) {
	t.Parallel()

	lines := []string{"a = 1;", "b = 2;"}
	tokens := Script(lines)

	if len(tokens) != 2 {
		t.Fatalf("len(tokens) = %d, want 2", len(tokens))
	}

	if tokens[0].Text != "a = 1;" || tokens[0].Line != 1 {
		t.Fatalf("token[0] = %+v", tokens[0])
	}
	if tokens[1].Text != "b = 2;" || tokens[1].Line != 2 {
		t.Fatalf("token[1] = %+v", tokens[1])
	}
}
