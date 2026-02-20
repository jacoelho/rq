package lower

import (
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

func decodeJSStringLiteral(raw string, quote byte) (string, bool) {
	var out strings.Builder
	out.Grow(len(raw))

	for index := 0; index < len(raw); index++ {
		current := raw[index]
		if current != '\\' {
			out.WriteByte(current)
			continue
		}

		index++
		if index >= len(raw) {
			return "", false
		}

		escaped := raw[index]
		switch escaped {
		case '\\':
			out.WriteByte('\\')
		case '"':
			out.WriteByte('"')
		case '\'':
			out.WriteByte('\'')
		case 'b':
			out.WriteByte('\b')
		case 'f':
			out.WriteByte('\f')
		case 'n':
			out.WriteByte('\n')
		case 'r':
			out.WriteByte('\r')
		case 't':
			out.WriteByte('\t')
		case 'v':
			out.WriteByte('\v')
		case '0':
			out.WriteByte('\x00')
		case '\n':
			// JavaScript line continuation: backslash + newline is removed.
		case '\r':
			// JavaScript line continuation: swallow optional following newline.
			if index+1 < len(raw) && raw[index+1] == '\n' {
				index++
			}
		case 'x':
			if index+2 >= len(raw) {
				return "", false
			}
			value, err := strconv.ParseUint(raw[index+1:index+3], 16, 8)
			if err != nil {
				return "", false
			}
			out.WriteByte(byte(value))
			index += 2
		case 'u':
			if index+1 < len(raw) && raw[index+1] == '{' {
				end := strings.IndexByte(raw[index+2:], '}')
				if end < 0 {
					return "", false
				}
				hexValue := raw[index+2 : index+2+end]
				if hexValue == "" {
					return "", false
				}
				value, err := strconv.ParseUint(hexValue, 16, 32)
				if err != nil || value > utf8.MaxRune {
					return "", false
				}
				out.WriteRune(rune(value))
				index = index + 2 + end
				continue
			}

			if index+4 >= len(raw) {
				return "", false
			}
			firstValue, err := strconv.ParseUint(raw[index+1:index+5], 16, 16)
			if err != nil {
				return "", false
			}
			firstRune := rune(firstValue)

			if utf16.IsSurrogate(firstRune) && index+10 < len(raw) && raw[index+5] == '\\' && raw[index+6] == 'u' {
				secondValue, secondErr := strconv.ParseUint(raw[index+7:index+11], 16, 16)
				if secondErr == nil {
					secondRune := rune(secondValue)
					decoded := utf16.DecodeRune(firstRune, secondRune)
					if decoded != utf8.RuneError {
						out.WriteRune(decoded)
						index += 10
						continue
					}
				}
			}

			out.WriteRune(firstRune)
			index += 4
		default:
			// Preserve JS non-special escapes by returning the escaped character.
			if escaped == quote {
				out.WriteByte(quote)
				continue
			}
			out.WriteByte(escaped)
		}
	}

	return out.String(), true
}
