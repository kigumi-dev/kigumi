package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// closureRunsDuring reports whether a closure write is lost to a sync `mut self` call: it may run between the read of the
// receiver's root and the store back (index operands, arguments) and replace storage on the receiver's path.
func (c *checker) closureRunsDuring(w mutWriteSite, call outsideMutCall) bool {
	e := &c.r.Entities[w.closure]
	if e.File != c.f || e.Node == 0 {
		return true
	}
	end := c.callEnd(call)
	start := c.t.Span(e.Node).Start
	inside := c.t.Span(call.base).Start <= start && start < end
	if c.directArg(e.Node, call) || c.r.closure(w.closure).Immediate && !inside {
		return false
	}
	exists := call.enclosing != 0 || start < end || c.sharesLoop(e.Node, call.base)
	window := append(c.receiverOperands(call.base, nil), call.args...)
	return exists && c.replacesReceiverPath(w, call.base) && c.argsRunCode(window)
}

// receiverOperands lists the index operands of a receiver place, which run after its root is read.
func (c *checker) receiverOperands(n syntax.NodeID, out []syntax.NodeID) []syntax.NodeID {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.Paren:
		return c.receiverOperands(v.AsParen().Inner().ID(), out)
	case syntax.MemberExpr:
		return c.receiverOperands(v.AsMember().Base().ID(), out)
	case syntax.BracketExpr:
		out = c.receiverOperands(v.AsBracket().Base().ID(), out)
		for a := range v.AsBracket().Args().IDs() {
			out = append(out, a)
		}
	}
	return out
}

// replacesReceiverPath reports whether the write may replace storage the receiver path runs through: an assignment or
// `&mut` borrow of a prefix of the path, or a `mut self` call on one that may replace its receiver or a field on it.
func (c *checker) replacesReceiverPath(w mutWriteSite, base syntax.NodeID) bool {
	recv, ok := c.pathOf(base)
	if !ok {
		return true
	}
	wrote, ok := c.pathOf(w.node)
	if !ok || wrote.root != recv.root {
		return true
	}
	if !wrote.prefixOf(recv) {
		return false
	}
	if !w.selfCall {
		return true
	}
	return w.method == 0 || len(wrote.segs) < len(recv.segs) || c.r.ReplacesSelf(w.method)
}

// placePath is a place as its root and the fields and indexes leading from it; a 0 field is an index.
type placePath struct {
	root EntityID
	segs []EntityID
}

func (p placePath) prefixOf(q placePath) bool {
	if len(p.segs) > len(q.segs) {
		return false
	}
	for i, s := range p.segs {
		if s != q.segs[i] {
			return false
		}
	}
	return true
}

func (c *checker) pathOf(n syntax.NodeID) (placePath, bool) {
	v := c.t.View(n)
	switch v.Kind() {
	case syntax.Ident:
		ent := c.info.Uses[n]
		return placePath{root: ent}, ent != 0
	case syntax.Paren:
		return c.pathOf(v.AsParen().Inner().ID())
	case syntax.MemberExpr:
		p, ok := c.pathOf(v.AsMember().Base().ID())
		fld := c.info.Uses[n]
		switch {
		case !ok || fld == 0:
			return placePath{}, false
		case c.r.Entities[fld].Kind == EntType:
			return p, true
		case c.r.Entities[fld].Kind != EntField:
			return placePath{}, false
		}
		return placePath{p.root, append(append([]EntityID(nil), p.segs...), fld)}, true
	case syntax.BracketExpr:
		p, ok := c.pathOf(v.AsBracket().Base().ID())
		return placePath{p.root, append(append([]EntityID(nil), p.segs...), 0)}, ok
	}
	return placePath{}, false
}

func (c *checker) callEnd(call outsideMutCall) token.Pos {
	end := c.t.Span(call.base).End
	for _, a := range call.args {
		end = max(end, c.t.Span(a).End)
	}
	return end
}

func (c *checker) directArg(n syntax.NodeID, call outsideMutCall) bool {
	for _, a := range call.args {
		if c.t.Kind(a) == syntax.NamedArg {
			a = c.t.View(a).AsNamedArg().Value().ID()
		}
		if c.t.Unparen(a) == n {
			return true
		}
	}
	return false
}
