package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

func (c *checker) warnBareInterpolation(n syntax.NodeID) {
	sp := c.t.Span(n)
	src := c.t.File.Src[sp.Start:sp.End]
	for i := 1; i+1 < len(src); i++ {
		switch {
		case src[i] == '\\':
			i++
		case src[i] == '$' && src[i+1] == '{':
			i = skipInterpolation(src, i+2)
		case src[i] == '$' && isIdentStart(src[i+1]):
			j := i + 2
			for j < len(src) && isIdentPart(src[j]) {
				j++
			}
			name := string(src[i+1 : j])
			if c.isLocalName(name) {
				at := token.Span{Start: sp.Start + token.Pos(i), End: sp.Start + token.Pos(j)}
				msg := "`$" + name + "` is plain text in a string"
				key := diagKey{c.f, at.Start, "bare-interpolation", msg}
				if _, dup := c.r.seen[key]; !dup {
					c.r.seen[key] = struct{}{}
					c.r.emitDiag(c.f, diag.Warnf(c.r.loc(c.f, at), msg).WithHelp("write `${"+name+"}` to insert the value of `"+name+"`"))
				}
			}
			i = j - 1
		}
	}
}

func (c *checker) isLocalName(name string) bool {
	b, _, ok := c.r.lookup(c.scope, name)
	if !ok || b.Ent == 0 {
		return false
	}
	k := c.r.Entities[b.Ent].Kind
	return k == EntLocal || k == EntParam
}

func skipInterpolation(src []byte, i int) int {
	for depth := 1; i < len(src); i++ {
		switch src[i] {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return i
			}
		}
	}
	return i
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentPart(c byte) bool { return isIdentStart(c) || c >= '0' && c <= '9' }
