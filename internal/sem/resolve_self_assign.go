package sem

import "kigumi/internal/syntax"

func (c *checker) noteWholeSelfAssign(lhs syntax.NodeID) {
	ent, ok := c.identPlaceEnt(lhs)
	if !ok {
		return
	}
	e := c.r.Entities[ent]
	if e.Flags&EfSelf == 0 || e.Flags&EfMove != 0 || e.Parent == 0 {
		return
	}
	c.r.selfWholeAssign[e.Parent] = true
}

// `self.m()` inside a `mut self` body: m replacing its receiver makes owner replace its own.
type selfRecvCall struct {
	owner   EntityID
	callee  EntityID
	witness bool
	iface   TypeID
	pkg     PackageID
}

func (c *checker) noteSelfRecvCall(base syntax.NodeID, m EntityID, witness bool, iface TypeID) {
	if c.r.Fn(m).Recv != RecvMut {
		return
	}
	ent, ok := c.identPlaceEnt(base)
	if !ok {
		return
	}
	e := c.r.Entities[ent]
	if e.Flags&EfSelf == 0 || e.Flags&EfMove != 0 || e.Parent == 0 {
		return
	}
	c.r.selfRecvCalls = append(c.r.selfRecvCalls, selfRecvCall{owner: e.Parent, callee: m, witness: witness, iface: iface, pkg: c.pkg})
}

// Propagates "replaces its receiver" up the self-call chain, so a forwarding-only method still counts as replacing.
func (r *Result) closeSelfWholeAssign() {
	for changed := true; changed; {
		changed = false
		for _, call := range r.selfRecvCalls {
			if r.selfWholeAssign[call.owner] {
				continue
			}
			replacing := r.replacesSelf(call.callee)
			if call.witness {
				replacing = r.witnessReplacesSelf(call.callee, call.iface, call.pkg)
			}
			if replacing {
				r.selfWholeAssign[call.owner], changed = true, true
			}
		}
	}
}

// Whether the callee replaces self settles later, in the fixpoint.
func (c *checker) noteBorrowedRecv(base syntax.NodeID, recv TypeID, m EntityID) {
	if c.r.Fn(m).Recv != RecvMut {
		return
	}
	if ent, ok := c.refBinding(base); ok {
		c.queueRefRecv(refCellRecv{ent: ent, callee: m, node: base})
	} else if c.r.Types.Kind(recv) == KRef || c.leasedBinding(base) {
		c.queueRefRecv(refCellRecv{callee: m, node: base})
	}
}

// Like noteBorrowedRecv, but for a callee picked per instantiation; any implementation that replaces self decides it.
func (c *checker) noteWitnessRecv(base syntax.NodeID, recv TypeID, req EntityID, iface TypeID) {
	if c.r.Fn(req).Recv != RecvMut {
		return
	}
	if ent, ok := c.refBinding(base); ok {
		c.queueRefRecv(refCellRecv{ent: ent, callee: req, node: base, witness: true, iface: iface, pkg: c.pkg})
	} else if c.r.Types.Kind(recv) == KRef {
		c.queueRefRecv(refCellRecv{callee: req, node: base, witness: true, iface: iface, pkg: c.pkg})
	}
}

func (c *checker) queueRefRecv(q refCellRecv) {
	q.file = c.f
	c.r.refCellRecvs = append(c.r.refCellRecvs, q)
	if q.ent != 0 {
		c.noteRefCellSite(q.ent, q.node, false)
	}
}

// identPlaceEnt would reject a key outright, so the ident is resolved here.
func (c *checker) leasedBinding(base syntax.NodeID) bool {
	for c.t.Kind(base) == syntax.Paren {
		base = syntax.NodeID(c.t.Nodes[base].Lhs)
	}
	if c.t.Kind(base) != syntax.Ident {
		return false
	}
	ent := c.info.Uses[base]
	return ent != 0 && c.r.Entities[ent].Kind == EntContextKey
}

func (r *Result) witnessReplacingSelf(req EntityID) EntityID {
	for _, m := range r.witnessesOf(req) {
		if r.replacesSelf(m) {
			return m
		}
	}
	return 0
}

// Option.take replaces self but has no body to read, so it is special-cased here.
func (r *Result) replacesSelf(m EntityID) bool {
	if r.selfWholeAssign[m] {
		return true
	}
	e := r.Entities[m]
	return e.Name == "take" && r.Fn(m).Owner != 0 && r.Fn(m).Owner == r.Types.OptionEnt()
}
