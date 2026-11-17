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

type crossing struct {
	site, producer EntityID
	strict         bool
}

func (r *Result) readReleased(rs ReadSite, x crossing) bool {
	h := r.HolderOf(rs)
	if x.strict || r.Entities[h].Kind != EntClosure || r.within(x.site, h) {
		return false
	}
	c := r.closure(rs.Closure)
	if c.Creator != h || c.InLoop || !r.soleReader(rs) || !r.reachesClosure(r.closure(h).Ret, rs.Closure) {
		return false
	}
	return r.frameOf(rs.Local) == h || h == x.producer
}

func (r *Result) soleReader(rs ReadSite) bool {
	st := &r.thread
	if !st.settled {
		return false
	}
	if st.readers == nil {
		st.readers = map[EntityID][]EntityID{}
		for id := 1; id < len(r.Entities); id++ {
			if r.Entities[id].Kind != EntClosure {
				continue
			}
			for _, cp := range r.closure(EntityID(id)).Captures {
				if roleOf(cp) == roleFrameRead {
					st.readers[cp.Local] = append(st.readers[cp.Local], EntityID(id))
				}
			}
		}
	}
	rd := st.readers[rs.Local]
	return len(rd) == 1 && rd[0] == rs.Closure
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

func (r *Result) leavesAlone(t TypeID, x crossing) bool {
	for _, rs := range r.ReadSites(t) {
		if !r.readReleased(rs, x) {
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
		if r.HolderOf(rs) != fn && !r.readReleased(rs, crossing{site: fn}) {
			return false
		}
	}
	return true
}

func (r *Result) ReadReleased(rs ReadSite, site EntityID) bool {
	return r.readReleased(rs, crossing{site: site})
}

func (r *Result) FrameFn(e EntityID) EntityID {
	for r.Entities[e].Kind == EntClosure && r.closure(e).Creator != 0 {
		e = r.closure(e).Creator
	}
	return e
}
