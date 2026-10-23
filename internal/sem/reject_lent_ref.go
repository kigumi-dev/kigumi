package sem

import "kigumi/internal/syntax"

// Neither `&mut <place>` nor a binding of this body's: no storage backs a whole new value.
type refCellValue struct {
	param EntityID
	what  string
	file  FileID
	node  syntax.NodeID
	// referent is the lent value's type, the instantiation a witness call in the callee resolves against.
	referent TypeID
	key      EntityID
}

// Whether the callee replaces the referent as a whole settles later, in the fixpoint.
func (c *checker) noteLentRefArg(a syntax.NodeID, param EntityID, async bool) {
	// mir's peelNoOp sees through the same layer, and a coerced one stops both.
	for c.t.Kind(a) == syntax.Paren && len(c.info.Coerce[a].Steps) == 0 {
		a = c.t.View(a).AsParen().Inner().ID()
	}
	// An async call is already reported against the parameter by noteRefPlaceArg.
	if async {
		return
	}
	if c.t.Kind(a) == syntax.BorrowExpr {
		if key := c.lentCtxKey(a); key != 0 {
			c.r.refCellValues = append(c.r.refCellValues, refCellValue{param: param, file: c.f, node: a, referent: c.r.Entities[key].Type, key: key})
		}
		return
	}
	if _, ok := c.refBinding(a); ok {
		return
	}
	t := c.vars.resolve(c.info.Types[a])
	if !c.r.wholeRefBinding(t) {
		return
	}
	referent, _ := c.r.MutRefElem(t)
	c.r.refCellValues = append(c.r.refCellValues, refCellValue{param: param, what: lentRefShape(c.t, a), file: c.f, node: a, referent: referent})
}

func lentRefShape(t *syntax.Tree, a syntax.NodeID) string {
	switch t.Kind(a) {
	case syntax.MemberExpr, syntax.OptMemberExpr:
		return "a `&mut` held in a field"
	case syntax.CallExpr, syntax.WsCallExpr:
		return "a `&mut` a call returned"
	case syntax.BracketExpr:
		return "a `&mut` read out of an element"
	}
	return "a `&mut` with no place behind it"
}

// param 0 means the callee is unknown through a value call, so the argument is lent as a cell whatever its body does.
func (r *Result) reportLentRefArgs() {
	for _, v := range r.refCellValues {
		switch {
		case v.param != 0 && (!r.refCellParams[v.param] || r.lentInstanceKeepsValue(v)):
		case v.key != 0:
			r.errAt(v.file, v.node, cCtxKeyAssign, v.key, "lent with `&mut` to a callee that may replace it as a whole", ctxKeyWholeWhy)
		default:
			r.errAt(v.file, v.node, cRefCellLentValue, v.what)
		}
	}
}
