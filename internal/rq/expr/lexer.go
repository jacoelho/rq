package expr

import (
	"strconv"
	"strings"
	"unicode"
)

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenIdentifier
	tokenNumber
	tokenString
	tokenTrue
	tokenFalse
	tokenNull
	tokenEqual
	tokenNotEqual
	tokenAnd
	tokenOr
	tokenNot
	tokenLParen
	tokenRParen
)

type token struct {
	typ     tokenType
	literal string
	pos     int
}

func lex(input string) ([]token, error) {
	tokens := make([]token, 0, len(input)/2)
	pos := 0

	for pos < len(input) {
		r := rune(input[pos])
		if unicode.IsSpace(r) {
			pos++
			continue
		}

		if isIdentifierStart(r) {
			start := pos
			pos++
			for pos < len(input) && isIdentifierPart(rune(input[pos])) {
				pos++
			}
			literal := input[start:pos]
			switch literal {
			case "true":
				tokens = append(tokens, token{typ: tokenTrue, pos: start})
			case "false":
				tokens = append(tokens, token{typ: tokenFalse, pos: start})
			case "null":
				tokens = append(tokens, token{typ: tokenNull, pos: start})
			default:
				tokens = append(tokens, token{typ: tokenIdentifier, literal: literal, pos: start})
			}
			continue
		}

		if isNumberStart(input, pos) {
			numberToken, nextPos, err := lexNumber(input, pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, numberToken)
			pos = nextPos
			continue
		}

		if input[pos] == '\'' || input[pos] == '"' {
			literal, nextPos, err := lexString(input, pos)
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, token{typ: tokenString, literal: literal, pos: pos})
			pos = nextPos
			continue
		}

		switch input[pos] {
		case '=':
			if pos+1 < len(input) && input[pos+1] == '=' {
				tokens = append(tokens, token{typ: tokenEqual, pos: pos})
				pos += 2
				continue
			}
			return nil, expressionError("unexpected '=' at position %d", pos)
		case '!':
			if pos+1 < len(input) && input[pos+1] == '=' {
				tokens = append(tokens, token{typ: tokenNotEqual, pos: pos})
				pos += 2
				continue
			}
			tokens = append(tokens, token{typ: tokenNot, pos: pos})
			pos++
			continue
		case '&':
			if pos+1 < len(input) && input[pos+1] == '&' {
				tokens = append(tokens, token{typ: tokenAnd, pos: pos})
				pos += 2
				continue
			}
			return nil, expressionError("unexpected '&' at position %d", pos)
		case '|':
			if pos+1 < len(input) && input[pos+1] == '|' {
				tokens = append(tokens, token{typ: tokenOr, pos: pos})
				pos += 2
				continue
			}
			return nil, expressionError("unexpected '|' at position %d", pos)
		case '(':
			tokens = append(tokens, token{typ: tokenLParen, pos: pos})
			pos++
			continue
		case ')':
			tokens = append(tokens, token{typ: tokenRParen, pos: pos})
			pos++
			continue
		default:
			return nil, expressionError("unexpected character %q at position %d", input[pos], pos)
		}
	}

	tokens = append(tokens, token{typ: tokenEOF, pos: len(input)})
	return tokens, nil
}

func isIdentifierStart(r rune) bool {
	return r == '_' || unicode.IsLetter(r)
}

func isIdentifierPart(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

func isNumberStart(input string, pos int) bool {
	if pos >= len(input) {
		return false
	}
	if input[pos] >= '0' && input[pos] <= '9' {
		return true
	}
	if input[pos] == '-' {
		return pos+1 < len(input) && input[pos+1] >= '0' && input[pos+1] <= '9'
	}
	return false
}

func lexNumber(input string, start int) (token, int, error) {
	pos := start
	if input[pos] == '-' {
		pos++
	}

	digitStart := pos
	for pos < len(input) && input[pos] >= '0' && input[pos] <= '9' {
		pos++
	}
	if pos == digitStart {
		return token{}, 0, expressionError("invalid number at position %d", start)
	}

	if pos < len(input) && input[pos] == '.' {
		pos++
		fracStart := pos
		for pos < len(input) && input[pos] >= '0' && input[pos] <= '9' {
			pos++
		}
		if pos == fracStart {
			return token{}, 0, expressionError("invalid decimal number at position %d", start)
		}
	}

	literal := input[start:pos]
	if _, err := strconv.ParseFloat(literal, 64); err != nil {
		return token{}, 0, expressionError("invalid number %q at position %d", literal, start)
	}

	return token{typ: tokenNumber, literal: literal, pos: start}, pos, nil
}

func lexString(input string, start int) (string, int, error) {
	quote := input[start]
	var b strings.Builder

	for pos := start + 1; pos < len(input); pos++ {
		ch := input[pos]
		if ch == quote {
			return b.String(), pos + 1, nil
		}

		if ch == '\\' {
			pos++
			if pos >= len(input) {
				return "", 0, expressionError("unterminated escape sequence at position %d", start)
			}
			escaped := input[pos]
			switch escaped {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			case '\'', '"':
				b.WriteByte(escaped)
			default:
				b.WriteByte(escaped)
			}
			continue
		}

		if ch == '\n' || ch == '\r' {
			return "", 0, expressionError("unterminated string at position %d", start)
		}

		b.WriteByte(ch)
	}

	return "", 0, expressionError("unterminated string at position %d", start)
}
