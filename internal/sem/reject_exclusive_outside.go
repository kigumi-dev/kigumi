package sem

import "kigumi/internal/syntax"

// A closure from an earlier loop iteration is still alive, so a shared loop counts as written earlier.
func (c *checker) settleOutsideCall(call outsideMutCall) {
	callPos := c.t.Span(call.base).Start
	for _, root := range call.roots {
		field := call.lent[root]
		for _, w := range c.mutCaptures[root] {
			if (c.t.Span(w).Start < callPos || c.sharesLoop(w, call.base)) && !c.writeAvoidsField(w, field) {
				c.errCapturedMutConflict(call, root, w)
				return
			}
		}
		if cl := c.storedReadCapture(call, root, field); cl != 0 {
			c.errStoredCapture(call, root, cl)
			return
		}
	}
}

func (c *checker) writeAvoidsField(w syntax.NodeID, field EntityID) bool {
	if field == 0 {
		return false
	}
	p, ok := c.buildPlace(w)
	return ok && len(p.Fields) > 0 && p.Fields[0] != field
}

// A closure that borrows or moves root can be stored in root's own storage when its type can hold a callable.
func (c *checker) storedReadCapture(call outsideMutCall, root, field EntityID) EntityID {
	probe := newHeldResult(call.roots, call.lent)
	c.scanHeld(c.r.Entities[root].Type, probe)
	if !probe.opaque && probe.conflict == 0 {
		return 0
	}
	callPos := c.t.Span(call.base).Start
	for _, site := range c.captureSites[root] {
		ctx := site.ctx
		cl := &c.r.Entities[ctx.ent]
		if ctx.immediate || ctx.list[site.idx].Mode == capCopy || cl.File != c.f || cl.Node == 0 {
			continue
		}
		if c.t.Span(cl.Node).Start >= callPos && !c.sharesLoop(cl.Node, call.base) {
			continue
		}
		if !c.closureAvoidsField(ctx.ent, root, field) {
			return ctx.ent
		}
	}
	return 0
}

// closureAvoidsField proves a closure only touches fields of root other than field.
func (c *checker) closureAvoidsField(closure, root, field EntityID) bool {
	cl := &c.r.Entities[closure]
	if field == 0 || cl.File != c.f || cl.Node == 0 {
		return false
	}
	ok := true
	var visit func(n syntax.NodeID)
	visit = func(n syntax.NodeID) {
		if !ok {
			return
		}
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.BracketExpr:
			if p, good := c.buildPlace(n); good && p.Root == root {
				ok = ok && len(p.Fields) > 0 && p.Fields[0] != field
				return
			}
		case syntax.Ident:
			if c.info.Uses[n] == root {
				ok = false
				return
			}
		}
		for ch := range c.t.View(n).Children() {
			visit(ch.ID())
		}
	}
	visit(cl.Node)
	return ok
}
