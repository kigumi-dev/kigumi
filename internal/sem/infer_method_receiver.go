package sem

import "kigumi/internal/syntax"

func (c *checker) takeReceiver(n, base syntax.NodeID, recv TypeID, viaRef bool) bool {
	b := c.t.Unparen(base)
	switch {
	case viaRef || c.lentRoot(b):
		c.errAt(n, cMethodValueBorrowed, recv)
		return false
	case c.t.Kind(n) == syntax.OptMemberExpr:
		c.errAt(n, cMethodValuePlace, recv)
		return false
	}
	switch c.t.Kind(b) {
	case syntax.Ident:
		if ent := c.info.Uses[b]; ent != 0 {
			if k := c.r.Entities[ent].Kind; k == EntLocal || k == EntParam {
				return true
			}
		}
		c.errAt(n, cMethodValuePlace, recv)
		return false
	case syntax.MemberExpr, syntax.BracketExpr, syntax.OptMemberExpr:
		c.errAt(n, cMethodValuePlace, recv)
		return false
	}
	return true
}

func (c *checker) lentRoot(n syntax.NodeID) bool {
	tt := c.r.Types
	for i := 0; n != 0 && i < 64; i++ {
		if tt.Kind(c.vars.resolve(c.info.Types[n])) == KRef {
			return true
		}
		switch c.t.Kind(n) {
		case syntax.Paren:
			n = c.t.View(n).AsParen().Inner().ID()
		case syntax.MemberExpr, syntax.BracketExpr, syntax.OptMemberExpr:
			n = c.t.View(n).Base().ID()
		case syntax.Ident:
			ent := c.info.Uses[n]
			if ent == 0 {
				return false
			}
			e := &c.r.Entities[ent]
			return e.Flags&EfSelf != 0 && e.Flags&EfMove == 0 || tt.Kind(e.Type) == KRef
		default:
			return false
		}
	}
	return false
}
