package lex

// Token is a single script line token with source line metadata.
type Token struct {
	Text string
	Line int
}

// Script converts source lines into line tokens, preserving source order.
func Script(lines []string) []Token {
	if len(lines) == 0 {
		return nil
	}

	tokens := make([]Token, 0, len(lines))
	for index, line := range lines {
		tokens = append(tokens, Token{
			Text: line,
			Line: index + 1,
		})
	}

	return tokens
}
