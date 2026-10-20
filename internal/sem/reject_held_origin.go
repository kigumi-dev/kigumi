package sem

import "kigumi/internal/syntax"

func (c *checker) heldOf(expr syntax.NodeID, h *heldResult, seen map[syntax.NodeID]bool) {
	expr = c.t.Unparen(expr)
	if c.t.Kind(expr) == syntax.NamedArg {
		expr = c.t.Unparen(c.t.View(expr).AsNamedArg().Value().ID())
	}
	if seen[expr] || h.conflict != 0 {
		return
	}
	seen[expr] = true
	switch c.t.Kind(expr) {
	case syntax.Spread:
		c.heldOf(c.t.View(expr).AsSpread().Value().ID(), h, seen)
		return
	case syntax.FieldInit:
		c.heldOf(c.t.View(expr).AsFieldInit().Value().ID(), h, seen)
		return
	}
	sub := h.sub()
	c.scanHeld(c.info.Types[expr], sub)
	if sub.conflict != 0 {
		h.merge(sub)
		return
	}
	if !sub.opaque {
		return
	}
	switch c.t.Kind(expr) {
	case syntax.Lambda:
		c.closureHolds(c.info.Defs[expr], h)
	case syntax.Ident:
		c.heldOfLocal(c.info.Uses[expr], expr, h, seen)
	case syntax.MemberExpr:
		c.heldOf(c.t.View(expr).AsMember().Base().ID(), h, seen)
	case syntax.BracketExpr:
		c.heldOf(c.t.View(expr).AsBracket().Base().ID(), h, seen)
	case syntax.TryExpr:
		c.heldOf(c.t.View(expr).AsTry().Operand().ID(), h, seen)
	case syntax.RecordLit:
		c.heldOfList(c.t.View(expr).AsRecordLit().Fields(), h, seen)
	case syntax.TupleLit:
		c.heldOfList(c.t.View(expr).AsTupleLit().ListView, h, seen)
	case syntax.CallExpr, syntax.WsCallExpr:
		c.heldOfCall(expr, h, seen)
	default:
		h.lose(expr, "this expression")
	}
}

// Parameters are trusted: the caller's own exclusive-call check rejects a closure over a lent root.
func (c *checker) heldOfLocal(ent EntityID, use syntax.NodeID, h *heldResult, seen map[syntax.NodeID]bool) {
	if init, ok := c.letInit[ent]; ok {
		c.heldOf(init, h, seen)
		return
	}
	if ent == 0 || c.r.Entities[ent].Kind != EntLocal {
		return
	}
	h.lose(use, "`"+c.t.View(use).Tok().Text()+"`")
}

func (c *checker) heldOfList(l syntax.ListView, h *heldResult, seen map[syntax.NodeID]bool) {
	for i := range l.Len() {
		c.heldOf(l.ItemID(i), h, seen)
	}
}

// A call can only return what its receiver and arguments hand it.
func (c *checker) heldOfCall(call syntax.NodeID, h *heldResult, seen map[syntax.NodeID]bool) {
	var callee syntax.NodeID
	var args syntax.ListView
	if c.t.Kind(call) == syntax.CallExpr {
		v := c.t.View(call).AsCall()
		callee, args = v.Callee().ID(), v.Args()
	} else {
		v := c.t.View(call).AsWsCall()
		callee, args = v.Callee().ID(), v.Args()
	}
	switch c.t.Kind(callee) {
	case syntax.MemberExpr:
		c.heldOf(c.t.View(callee).AsMember().Base().ID(), h, seen)
	case syntax.Ident:
		if ent := c.info.Uses[callee]; ent == 0 || c.r.Entities[ent].Kind != EntFn && c.r.Entities[ent].Kind != EntVariant {
			h.lose(call, "this call's result")
			return
		}
	default:
		h.lose(call, "this call's result")
		return
	}
	c.heldOfList(args, h, seen)
}
