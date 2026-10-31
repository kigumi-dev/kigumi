package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) rejectSharedMutRef(n syntax.NodeID, t TypeID) {
	if c.r.Types.RefMut(c.vars.resolve(t)) && c.behindShared(n) {
		c.errAt(n, cSharedMutRef, c.placeName(n))
	}
}

func (c *checker) behindShared(n syntax.NodeID) bool {
	for i := 0; n != 0 && i < 64; i++ {
		switch c.t.Kind(n) {
		case syntax.Paren:
			n = c.t.View(n).AsParen().Inner().ID()
			continue
		case syntax.MemberExpr, syntax.BracketExpr:
			n = c.t.View(n).Base().ID()
		case syntax.Ident:
			e := c.info.Uses[n]
			return e != 0 && plainSelfReceiver(&c.r.Entities[e])
		default:
			return false
		}
		if t := c.vars.resolve(c.info.Types[c.t.Unparen(n)]); c.r.Types.Kind(t) == KRef && !c.r.Types.RefMut(t) {
			return true
		}
	}
	return false
}

// Iterating as `&` keeps the elements readable but not writable.
func (c *checker) sharedIter(head syntax.NodeID, iter TypeID) TypeID {
	if c.r.Types.RefMut(iter) && c.behindShared(head) {
		return c.r.Types.Ref(c.r.Types.Node(iter).Elem, false)
	}
	return iter
}
