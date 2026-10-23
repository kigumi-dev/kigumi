package sem

import "kigumi/internal/syntax"

// borrowedCallee unwraps a `&F` / `&mut F` callee to F when the borrow's kind can run it:
// a shared borrow needs `Fn`, an exclusive one `FnMut`.
func (c *checker) borrowedCallee(n syntax.NodeID, ft TypeID) (TypeID, bool) {
	tt := c.r.Types
	inner := c.vars.resolve(tt.Node(ft).Elem)
	sig, ok := c.calleeSig(inner)
	if !ok {
		return ft, true
	}
	kind := CFn
	if tt.RefMut(ft) {
		kind = CFnMut
	}
	if why := c.fnUnsatisfied(inner, Constraint{Kind: kind, Type: sig}); why != "" {
		if k, ok := c.paramCallKind(inner); ok && k > kind {
			why = "`" + c.r.TypeString(inner) + "` is only `" + callKindName[k] + "`"
		}
		c.errAt(n, cBorrowedCallKind, ft, why)
		return ft, false
	}
	return inner, true
}

var callKindName = map[ConstraintKind]string{CFn: "Fn", CFnMut: "FnMut", CFnOnce: "FnOnce"}

func (c *checker) paramCallKind(t TypeID) (ConstraintKind, bool) {
	if c.r.Types.Kind(t) != KParam {
		return 0, false
	}
	best, ok := ConstraintKind(0), false
	for _, con := range c.r.typeParam(c.r.Types.Node(t).Ent).Constraints {
		if con.Kind >= CFn && con.Kind <= CFnOnce && (!ok || con.Kind < best) {
			best, ok = con.Kind, true
		}
	}
	return best, ok
}

func (c *checker) calleeSig(t TypeID) (TypeID, bool) {
	tt := c.r.Types
	switch tt.Kind(t) {
	case KFn:
		return t, !tt.IsCFn(t)
	case KClosure:
		return c.vars.resolve(c.r.closure(tt.Node(t).Ent).Sig), true
	case KParam:
		for _, con := range c.r.typeParam(tt.Node(t).Ent).Constraints {
			if con.Kind == CFn || con.Kind == CFnMut || con.Kind == CFnOnce {
				return con.Type, true
			}
		}
	}
	return 0, false
}
