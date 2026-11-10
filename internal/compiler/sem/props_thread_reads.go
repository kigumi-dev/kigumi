package sem

import "slices"

// ReadSite is a local that Closure reads in place, the referent staying with the frame that holds it.
type ReadSite struct {
	Closure, Local EntityID
}

// ReadSites are the in-place reads of the closures inside t, through the closures they own and t's type arguments.
func (r *Result) ReadSites(t TypeID) []ReadSite {
	st := &r.thread
	if rs, ok := st.reads[t]; ok {
		return rs
	}
	var rs []ReadSite
	if t != 0 && r.Types.mentionsClosure(t) {
		rs = r.collectReadSites(t, nil)
	}
	if st.settled {
		if st.reads == nil {
			st.reads = map[TypeID][]ReadSite{}
		}
		st.reads[t] = rs
	}
	return rs
}

func (r *Result) collectReadSites(t TypeID, out []ReadSite) []ReadSite {
	add := func(rs []ReadSite) {
		for _, x := range rs {
			if !slices.Contains(out, x) {
				out = append(out, x)
			}
		}
	}
	n := r.Types.Node(t)
	if n.Kind != KClosure {
		if n.Elem != 0 {
			add(r.ReadSites(n.Elem))
		}
		for _, a := range n.Args {
			add(r.ReadSites(a))
		}
		return out
	}
	for _, cp := range r.closure(n.Ent).Captures {
		switch roleOf(cp) {
		case roleFrameRead:
			add([]ReadSite{{n.Ent, cp.Local}})
		case roleOwned:
			add(r.ReadSites(r.Entities[cp.Local].Type))
		}
	}
	return out
}

// HolderOf is the frame that keeps what rs reads: from the closure outward, the first frame that declares the local
// or took it by move; frames that only read it pass the read on.
func (r *Result) HolderOf(rs ReadSite) EntityID {
	decl := r.frameOf(rs.Local)
	for e := r.closure(rs.Closure).Creator; e != 0; e = r.closure(e).Creator {
		if e == decl || r.Entities[e].Kind != EntClosure {
			return e
		}
		for _, cp := range r.closure(e).Captures {
			if cp.Local == rs.Local && roleOf(cp) == roleOwned {
				return e
			}
		}
	}
	return decl
}

// within reports whether body is h or written inside it, so h is running whenever body is.
func (r *Result) within(body, h EntityID) bool {
	for e := body; e != 0; e = r.closure(e).Creator {
		if e == h {
			return true
		}
		if r.Entities[e].Kind != EntClosure {
			return false
		}
	}
	return false
}

// readReleased: a holder lambda that has returned the closure, and is not running at site, leaves it the read.
func (r *Result) readReleased(rs ReadSite, site EntityID) bool {
	h := r.HolderOf(rs)
	return r.Entities[h].Kind == EntClosure && !r.within(site, h) && r.reachesClosure(r.closure(h).Ret, rs.Closure)
}

// reachesClosure reports whether a value of type t holds closure cl, itself or through closures it owns.
func (r *Result) reachesClosure(t TypeID, cl EntityID) bool {
	if t == 0 || !r.Types.mentionsClosure(t) {
		return false
	}
	n := r.Types.Node(t)
	if n.Kind == KClosure {
		if n.Ent == cl {
			return true
		}
		for _, cp := range r.closure(n.Ent).Captures {
			if roleOf(cp) == roleOwned && r.reachesClosure(r.Entities[cp.Local].Type, cl) {
				return true
			}
		}
		return false
	}
	if r.reachesClosure(n.Elem, cl) {
		return true
	}
	return slices.ContainsFunc(n.Args, func(a TypeID) bool { return r.reachesClosure(a, cl) })
}

// leavesAlone: no read inside t is still held by a running frame when the value leaves site by itself.
func (r *Result) leavesAlone(t TypeID, site EntityID) bool {
	for _, rs := range r.ReadSites(t) {
		if !r.readReleased(rs, site) {
			return false
		}
	}
	return true
}

// IsSendInFrame is Send for a value that moves only with fn's frame: a read fn itself holds moves along.
func (r *Result) IsSendInFrame(t TypeID, fn EntityID) bool {
	if !r.isSend(t) {
		return false
	}
	for _, rs := range r.ReadSites(t) {
		if r.HolderOf(rs) != fn && !r.readReleased(rs, fn) {
			return false
		}
	}
	return true
}
