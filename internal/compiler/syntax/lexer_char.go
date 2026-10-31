package syntax

import "unicode/utf8"

// A quote that starts a lifetime or nothing valid is not a char literal; the length is 0.
func charLiteralLen(src []byte, pos, end int) int {
	if pos+1 >= end {
		return 0
	}
	j := pos + 1
	switch c := src[j]; {
	case c == '\\':
		j++
		if j >= end {
			return 0
		}
		if src[j] == 'u' && j+1 < end && src[j+1] == '{' {
			for j < end && src[j] != '}' {
				j++
			}
		}
		j++
	case c == '\'' || c == '\n':
		return 0
	default:
		_, size := utf8.DecodeRune(src[j:end])
		j += size
	}
	if j < end && src[j] == '\'' {
		return j + 1 - pos
	}
	return 0
}
