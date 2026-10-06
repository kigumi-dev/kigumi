package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// A `mut self` call on a borrowed receiver, judged once every body is checked; witness marks a callee picked per instantiation.
type borrowRecvCall struct {
	callee  EntityID
	file    FileID
	node    syntax.NodeID
	witness bool
	iface   TypeID
	pkg     PackageID
}

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

// noteBorrowedRecv queues a `mut self` call whose receiver this body only borrows: a reference, or a context key's lease.
func (c *checker) noteBorrowedRecv(base syntax.NodeID, recv TypeID, m EntityID) {
	if c.r.Fn(m).Recv != RecvMut {
		return
	}
	if c.r.Types.Kind(recv) != KRef && !c.leasedBinding(base) {
		return
	}
	c.r.borrowRecvCalls = append(c.r.borrowRecvCalls, borrowRecvCall{callee: m, file: c.f, node: base})
}

// An existential receiver keeps its box, so only a borrowed type parameter is at risk here.
func (c *checker) noteWitnessRecv(base syntax.NodeID, recv TypeID, req EntityID, iface TypeID) {
	tt := c.r.Types
	if c.r.Fn(req).Recv != RecvMut || tt.Kind(recv) != KRef || tt.Kind(tt.Node(recv).Elem) != KParam {
		return
	}
	c.r.borrowRecvCalls = append(c.r.borrowRecvCalls, borrowRecvCall{callee: req, file: c.f, node: base, witness: true, iface: iface, pkg: c.pkg})
}

func (c *checker) leasedBinding(base syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(base)
	return ok && c.r.Entities[ent].Kind == EntContextKey
}

// A borrow names no place of the caller's for a whole new value, so a queued callee that replaces self outright is rejected.
func (r *Result) checkBorrowedReceivers() {
	for _, call := range r.borrowRecvCalls {
		if call.witness {
			w, resolved := r.witnessReplacingSelfConforming(call.callee, call.iface, call.pkg)
			if !resolved {
				w = r.witnessReplacingSelf(call.callee)
			}
			if w != 0 {
				r.errAt(call.file, call.node, cWitnessRecvReplace, r.Entities[call.callee].Name, r.memberPath(w))
			}
			continue
		}
		if !r.replacesSelf(call.callee) {
			continue
		}
		r.errAt(call.file, call.node, cBorrowedRecvReplace, r.Entities[call.callee].Name)
	}
}

// Conformance is seen from pkg, so a witness private to another package looks unresolved;
// such a candidate falls back to the name/arity approximation instead of being cleared.
func (r *Result) witnessReplacingSelfConforming(req EntityID, iface TypeID, pkg PackageID) (EntityID, bool) {
	if iface == 0 || iface == TyPoison || r.Types.Kind(iface) != KIface {
		return 0, false
	}
	reqs := r.iface(r.Types.Node(iface).Ent).Reqs
	idx := slices.Index(reqs, req)
	if idx < 0 {
		return 0, false
	}
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		if r.Entities[id].Kind != EntType {
			continue
		}
		t := r.ownerInstance(id)
		res := r.conforms(t, iface, pkg)
		if res.ok {
			if r.replacesSelf(res.witnesses[idx]) {
				return res.witnesses[idx], true
			}
			continue
		}
		if w := r.mutSelfReplacerOwnedBy(id, req); w != 0 && !r.visibleFrom(r.Entities[w].Vis, pkg) {
			return w, true
		}
	}
	return 0, true
}

// Matches by name, receiver and arity only, over-approximating rather than risk missing a method that replaces self.
func (r *Result) witnessReplacingSelf(req EntityID) EntityID {
	name, arity := r.Entities[req].Name, len(r.Fn(req).Params)
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		e := &r.Entities[id]
		if id == req || e.Kind != EntFn || e.Name != name {
			continue
		}
		fi := r.Fn(id)
		if fi.Recv != RecvMut || len(fi.Params) != arity || fi.Owner == 0 || r.Entities[fi.Owner].Kind != EntType {
			continue
		}
		if r.replacesSelf(id) {
			return id
		}
	}
	return 0
}

func (r *Result) mutSelfReplacerOwnedBy(owner, req EntityID) EntityID {
	name, arity := r.Entities[req].Name, len(r.Fn(req).Params)
	for id := EntityID(1); id < EntityID(len(r.Entities)); id++ {
		e := &r.Entities[id]
		if id == req || e.Kind != EntFn || e.Name != name {
			continue
		}
		fi := r.Fn(id)
		if fi.Recv != RecvMut || len(fi.Params) != arity || fi.Owner != owner {
			continue
		}
		if r.replacesSelf(id) {
			return id
		}
	}
	return 0
}

func (r *Result) memberPath(m EntityID) string {
	return r.entityName(r.Fn(m).Owner) + "." + r.Entities[m].Name
}

// Option.take replaces self but has no body to read, so it is special-cased here.
func (r *Result) replacesSelf(m EntityID) bool {
	if r.selfWholeAssign[m] {
		return true
	}
	e := r.Entities[m]
	return e.Name == "take" && r.Fn(m).Owner != 0 && r.Fn(m).Owner == r.Types.OptionEnt()
}
