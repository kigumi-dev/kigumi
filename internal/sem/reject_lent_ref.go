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
}

// Whether the callee replaces the referent as a whole settles later, in the fixpoint.
func (c *checker) noteLentRefArg(a syntax.NodeID, param EntityID, async bool) {
	// mir's peelNoOp sees through the same layer, and a coerced one stops both.
	for c.t.Kind(a) == syntax.Paren && len(c.info.Coerce[a].Steps) == 0 {
		a = syntax.NodeID(c.t.Nodes[a].Lhs)
	}
	// An async call is already reported against the parameter by noteRefPlaceArg.
	if async || c.t.Kind(a) == syntax.BorrowExpr {
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
		if v.param == 0 || r.refCellParams[v.param] && !r.lentInstanceKeepsValue(v) {
			r.errAt(v.file, v.node, cRefCellLentValue, v.what)
		}
	}
}
