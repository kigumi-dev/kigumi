package syntax

// blockCommentEnd returns the offset just past the nested block comment opening at pos, and false when it runs
// to end unclosed.
func blockCommentEnd(src []byte, pos, end int) (int, bool) {
	depth := 0
	for pos < end {
		switch {
		case pos+1 < end && src[pos] == '/' && src[pos+1] == '*':
			depth++
			pos += 2
		case pos+1 < end && src[pos] == '*' && src[pos+1] == '/':
			depth--
			pos += 2
			if depth == 0 {
				return pos, true
			}
		default:
			pos++
		}
	}
	return pos, false
}

// interpLineCommentEnd returns where a line comment inside `${...}` stops: at the newline, or at the brace that
// closes the interpolation (depth 1), which a comment cannot hide since the sub-lexer ends there too.
func interpLineCommentEnd(src []byte, pos, end, depth int) int {
	for pos < end && src[pos] != '\n' && (depth > 1 || src[pos] != '}') {
		pos++
	}
	return pos
}
