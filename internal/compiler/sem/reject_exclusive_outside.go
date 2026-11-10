package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// An exclusive access conflicts with a write of the same root that can run while the access holds it.
func (c *checker) settleOutsideCall(call outsideMutCall) {
	callPos := c.t.Span(call.base).Start
	for _, root := range call.roots {
		field := call.lent[root]
		for _, w := range c.writeSites(root) {
			if !c.writeReachesCall(w, call, callPos) || c.writeAvoidsField(w.node, field) {
				continue
			}
			c.errCapturedMutConflict(call, root, w.node)
			return
		}
		// A call inside a closure was never checked against stored captures before; keep that boundary.
		if call.enclosing == 0 {
			if cl := c.storedReadCapture(call, root, field); cl != 0 {
				c.errStoredCapture(call, root, cl)
				return
			}
		}
	}
}

// writeReachesCall decides whether a write of this root conflicts with the call. A `&mut` argument and an async call,
// whose future holds the receiver past the call, are judged by where the write sits, not by when its closure can run.
func (c *checker) writeReachesCall(w mutWriteSite, call outsideMutCall, callPos token.Pos) bool {
	switch {
	case w.closure == 0:
		return call.enclosing == 0 && c.wroteEarlier(w.node, call, callPos)
	case c.insideClosureBody(w.closure, call.base):
		return false
	case !call.byRef && !call.async:
		return c.closureRunsDuring(w, call)
	case c.argumentClosure(w.closure):
		return false
	case call.byRef || !w.selfCall && call.enclosing == 0:
		return c.wroteEarlier(w.node, call, callPos)
	}
	return true
}

func (c *checker) wroteEarlier(w syntax.NodeID, call outsideMutCall, callPos token.Pos) bool {
	return c.t.Span(w).Start < callPos || c.sharesLoop(w, call.base)
}

// argumentClosure reports whether closure is a call argument the callee cannot keep, so the argument rule judges it.
func (c *checker) argumentClosure(closure EntityID) bool {
	return c.r.closure(closure).Immediate || c.inlineArgClosure(closure)
}

// inlineArgClosure reports whether closure is written inside one of an exclusive call's argument expressions.
func (c *checker) inlineArgClosure(closure EntityID) bool {
	cl := &c.r.Entities[closure]
	if cl.File != c.f || cl.Node == 0 {
		return false
	}
	start := c.t.Span(cl.Node).Start
	for _, a := range c.exclusiveArgs {
		if arg := c.t.Span(a); arg.Start <= start && start < arg.End {
			return true
		}
	}
	return false
}

// insideClosureBody reports whether n sits lexically in the body of closure.
func (c *checker) insideClosureBody(closure EntityID, n syntax.NodeID) bool {
	e := &c.r.Entities[closure]
	if e.File != c.f || e.Node == 0 {
		return false
	}
	body, pos := c.t.Span(e.Node), c.t.Span(c.t.Unparen(n)).Start
	return body.Start <= pos && pos < body.End
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
func (c *checker) writeSites(root EntityID) []mutWriteSite {
	out := c.mutCaptures[root]
	for _, cl := range c.r.ctxImplicit[root] {
		e := &c.r.Entities[cl]
		for _, cap := range c.r.closure(cl).Captures {
			if cap.Local == root && cap.Mode == capCell && e.File == c.f && e.Node != 0 {
				out = append(slices.Clone(out), mutWriteSite{node: e.Node, closure: cl})
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
