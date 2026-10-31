package sem

import "kigumi/internal/compiler/syntax"

// eraseRule is how much of a capture's later sharing the end-of-body re-check judges for one erasure.
type eraseRule uint8

const (
	// ruleFull: the plain fn may run anywhere later (stored, returned, passed to an `escaping` parameter).
	ruleFull eraseRule = iota
	// ruleInline: the plain fn lives only for the call it is lent to; sem judges a write inside the closure, MIR one by another
	// argument of that call.
	ruleInline
	// ruleLocal: bound to a local in the body that built the closure; MIR's liveness check judges a direct write, sem a write
	// inside a closure.
	ruleLocal
)

// erasure is a closure that passed closureErase; ret is the return type its help text was judged against.
type erasure struct {
	node    syntax.NodeID
	closure EntityID
	want    TypeID
	ret     TypeID
	rule    eraseRule
}

// eraseCaptures reports the first capture a plain fn cannot hold; a cell is allowed only to an inline reader.
func (c *checker) eraseCaptures(n syntax.NodeID, closure EntityID, want TypeID, rule eraseRule) bool {
	info := c.r.closure(closure)
	inline := info.Immediate || rule != ruleFull
	for _, cap := range info.Captures {
		name := c.capturedName(n, info, cap.Local)
		switch cap.Mode {
		case capCell:
			if inline && !lentReceiver(c.r.Entity(cap.Local)) && !c.closureWrites(closure, cap.Local) && (rule != ruleLocal || !c.writtenInClosure(cap.Local)) {
				continue
			}
			c.errAtHelp(n, c.cellFnHelp(want, closure, cap.Local), cClosureNotPlainFn, name, c.cellCaptureWhy(closure, cap.Local))
			return false
		case capMove, capRetain, capRead:
			c.errAtHelp(n, c.plainFnHelp(want, true), cClosureNotPlainFn, name, "move-only")
			return false
		}
		t := c.r.Entities[cap.Local].Type
		if c.callsFnMut(t, map[EntityID]bool{closure: true}) && (!inline || rule == ruleLocal && c.reachesClosureWrite(t, map[EntityID]bool{closure: true})) {
			c.errAtHelp(n, c.fnMutCallableHelp(want, name), cClosureNotPlainFn, name, "a callable that is `FnMut`")
			return false
		}
	}
	return true
}

func (c *checker) noteErasure(n syntax.NodeID, closure EntityID, want TypeID) {
	rule := ruleFull
	switch {
	case c.r.closure(closure).Immediate || c.eraseSites[n] == siteLent:
		rule = ruleInline
	case c.eraseSites[n] == siteLocal && c.createdHere(closure):
		rule = ruleLocal
	}
	c.erased = append(c.erased, erasure{node: n, closure: closure, want: want, ret: c.retType, rule: rule})
}

// recheckErasures reports, at the erasure, a capture that turned into a cell after it, as far as the erasure's rule judges.
func (c *checker) recheckErasures() {
	saved := c.retType
	for _, er := range c.erased {
		c.retType = er.ret
		c.eraseCaptures(er.node, er.closure, er.want, er.rule)
	}
	c.retType, c.erased = saved, nil
}
