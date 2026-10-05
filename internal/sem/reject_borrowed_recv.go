package sem

import "kigumi/internal/syntax"

// A `mut self` call on a borrowed receiver, judged once every body is checked; witness marks a callee picked per instantiation.
type borrowRecvCall struct {
	callee  EntityID
	file    FileID
	node    syntax.NodeID
	witness bool
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
func (c *checker) noteWitnessRecv(base syntax.NodeID, recv TypeID, req EntityID) {
	tt := c.r.Types
	if c.r.Fn(req).Recv != RecvMut || tt.Kind(recv) != KRef || tt.Kind(tt.Node(recv).Elem) != KParam {
		return
	}
	c.r.borrowRecvCalls = append(c.r.borrowRecvCalls, borrowRecvCall{callee: req, file: c.f, node: base, witness: true})
}

func (c *checker) leasedBinding(base syntax.NodeID) bool {
	ent, ok := c.identPlaceEnt(base)
	return ok && c.r.Entities[ent].Kind == EntContextKey
}

// A borrow names no place of the caller's for a whole new value, so a queued callee that replaces self outright is rejected.
func (r *Result) checkBorrowedReceivers() {
	for _, call := range r.borrowRecvCalls {
		if call.witness {
			if w := r.witnessReplacingSelf(call.callee); w != 0 {
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
