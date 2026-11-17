package syntax

import "kigumi/internal/lang/token"

func (t *Tree) VisibilitySpan(id NodeID) token.Span {
	sp := t.Span(id)
	if t.nodes[id].Lhs == 0 {
		return sp
	}
	src := t.File.Src
	end := int(sp.End)
	for end < len(src) && src[end] != ')' && src[end] != '\n' {
		end++
	}
	if end < len(src) && src[end] == ')' {
		end++
	}
	return token.Span{Start: sp.Start, End: token.Pos(end)}
}
