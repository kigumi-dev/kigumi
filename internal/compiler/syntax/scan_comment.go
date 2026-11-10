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

// interpLineCommentEnd returns where a line comment inside `${...}` stops. A line comment runs to the end of the
// physical line, so a brace inside it is comment text and cannot close the interpolation.
func interpLineCommentEnd(src []byte, pos, end int) int {
	for pos < end && src[pos] != '\n' {
		pos++
	}
	return pos
}
