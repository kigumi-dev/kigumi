package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// A closure from an earlier loop iteration is still alive, so a shared loop counts as written earlier.
func (c *checker) settleOutsideCall(call outsideMutCall) {
	callPos := c.t.Span(call.base).Start
	for _, root := range call.roots {
		field := call.lent[root]
		for _, w := range c.writeSites(root) {
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
	probe := c.newHeldResult(call.roots, call.lent)
	c.scanHeld(c.r.Entities[root].Type, probe)
	if !probe.opaque && probe.conflict == 0 {
		return 0
	}
	callPos := c.t.Span(call.base).Start
	for _, h := range c.holders(root) {
		cl := &c.r.Entities[h.closure]
		if h.immediate || h.mode == capCopy || cl.File != c.f || cl.Node == 0 {
			continue
		}
		if c.t.Span(cl.Node).Start >= callPos && !c.sharesLoop(cl.Node, call.base) {
			continue
		}
		if !c.closureAvoidsField(h.closure, root, field) {
			return h.closure
		}
	}
	return 0
}

// A closure that pulls a copy-shared provider in through a call writes it as far as this check can tell.
func (c *checker) writeSites(root EntityID) []syntax.NodeID {
	out := c.mutCaptures[root]
	for _, cl := range c.r.ctxImplicit[root] {
		e := &c.r.Entities[cl]
		for _, cap := range c.r.closure(cl).Captures {
			if cap.Local == root && cap.Mode == capCell && e.File == c.f && e.Node != 0 {
				out = append(slices.Clone(out), e.Node)
			}
		}
	}
	return out
}

// closureAvoidsField proves a closure only touches fields of root other than field.
func (c *checker) closureAvoidsField(closure, root, field EntityID) bool {
	cl := &c.r.Entities[closure]
	if field == 0 || cl.File != c.f || cl.Node == 0 {
		return false
	}
	ok := true
	touches := func(owner, bound EntityID) bool { return owner == root && (bound == 0 || bound == field) }
	var visit func(n syntax.NodeID)
	visit = func(n syntax.NodeID) {
		if !ok {
			return
		}
		for _, b := range c.info.CtxCall[n] {
			ok = ok && !touches(b.Local, b.Field)
		}
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.BracketExpr:
			if p, good := c.buildPlace(n); good && p.Root == root {
				ok = ok && len(p.Fields) > 0 && p.Fields[0] != field
				return
			}
		case syntax.Ident:
			use := c.info.CtxUse[n]
			ok = ok && c.info.Uses[n] != root && !touches(use.Local, use.Field)
			return
		}
		for ch := range c.t.View(n).Children() {
			visit(ch.ID())
		}
	}
	visit(cl.Node)
	return ok
}
