package sem

import "kigumi/internal/syntax"

// The caller writes its receiver cell back when the call returns, which an async method's future outlives.
func (c *checker) rejectAsyncSelfAssign(lhs syntax.NodeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok {
		return
	}
	e := c.r.Entities[ent]
	if e.Flags&EfSelf == 0 || e.Flags&EfMove != 0 {
		return
	}
	owner := e.Parent
	if owner == 0 || c.r.Entities[owner].Kind != EntFn || c.r.Fn(owner).Declared&EffAsync == 0 {
		return
	}
	c.errAt(lhs, cAsyncSelfAssign, c.r.Entities[owner].Name)
}

// A call that may replace self from an async `mut self` body loses the write for the same reason.
func (r *Result) rejectAsyncSelfReplace() {
	for _, call := range r.selfRecvCalls {
		replacing := r.replacesSelf(call.callee)
		if call.witness {
			replacing = r.witnessReplacesSelf(call.callee, call.iface)
		}
		if replacing && r.asyncMutSelf(call.owner) {
			r.errAt(call.file, call.node, cAsyncSelfReplace, r.Entities[call.owner].Name, "`"+r.Entities[call.callee].Name+"`")
		}
	}
	for _, l := range r.selfLentArgs {
		if !r.asyncMutSelf(l.owner) || r.asyncArgReported(l.file, l.node) {
			continue
		}
		if l.param == 0 || r.refCellParams[l.param] {
			callee := "a function value"
			if l.param != 0 {
				callee = "`" + r.Entities[r.Entities[l.param].Parent].Name + "`"
			}
			r.errAt(l.file, l.node, cAsyncSelfReplace, r.Entities[l.owner].Name, callee)
		}
	}
}

// The async-argument cell diagnostic already names this site, so a second report would only repeat it.
func (r *Result) asyncArgReported(f FileID, n syntax.NodeID) bool {
	for _, s := range append(r.refCellSites, r.refCellArgs...) {
		if s.async && s.file == f && s.node == n && r.refCellParams[s.ent] {
			return true
		}
	}
	return false
}

func (r *Result) asyncMutSelf(fn EntityID) bool {
	return r.Entities[fn].Kind == EntFn && r.Fn(fn).Declared&EffAsync != 0 && r.Fn(fn).Recv == RecvMut
}
