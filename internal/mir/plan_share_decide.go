package mir

func (w *shareFacts) writable(v LocalID) bool { return w.p.R.WritableInside(w.f.Locals[v].Type) }

// WritableInside is false for a T without Copy, yet its instance may be a writable record.
func (w *shareFacts) sealed(v LocalID) bool {
	t := w.f.Locals[v].Type
	m, ok := w.sum.sealed[t]
	if !ok {
		m = !w.p.R.WritableInside(t) && len(w.p.R.Types.Params(t)) == 0
		w.sum.sealed[t] = m
	}
	return m
}

// shared does not ask whether anything writes: the place a value is stored can be written anywhere later.
func (w *shareFacts) shared(v LocalID, at Point) bool {
	if !w.writable(v) {
		return false
	}
	rs := w.rootsOf(v)
	if rs.other {
		return true
	}
	for _, r := range rs.refs {
		if w.p.lentLocal(w.f, r.l) || w.observed(r.l, at) {
			return true
		}
	}
	return false
}

func (w *shareFacts) observed(l LocalID, at Point) bool {
	lv := w.liveness()
	if lv.after(l, at.Block, at.Inst) {
		return true
	}
	for _, v := range w.viewersOf(l) {
		if lv.after(v, at.Block, at.Inst) {
			return true
		}
	}
	return false
}

func (w *shareFacts) decideBindings() {
	for _, sites := range w.binds {
		for _, at := range sites {
			if w.inst(at).Op == OpCopy && w.copyBinding(at, w.inst(at)) {
				w.copyAt[at] = true
			}
		}
	}
	w.decided, w.memo, w.viewers = true, nil, nil
}

func (w *shareFacts) copyBinding(at Point, in Inst) bool {
	b := in.Dst
	rs := w.rootsOf(in.Args[0])
	if len(rs.refs) == 0 && !rs.other || !w.writable(b) {
		return false
	}
	if rs.other || w.reachable(at, w.own[b]) || w.reachable(at, w.inner[b]) {
		return true
	}
	lv := w.liveness()
	for _, r := range rs.refs {
		if r.own && lv.after(r.l, at.Block, at.Inst) {
			return true
		}
		if r.inner && w.interferes(r.l, b, at, in.Args[0]) {
			return true
		}
	}
	return false
}

func (w *shareFacts) interferes(r, b LocalID, at Point, src LocalID) bool {
	lv := w.liveness()
	s, exact := w.exactRoot(src, r, at)
	for _, x := range w.inner[r] {
		if w.reachable(at, []Point{x}) && lv.after(b, x.Block, x.Inst) && (!exact || w.touches(r, x, s)) {
			return true
		}
	}
	for _, x := range w.blind {
		if w.reachable(at, []Point{x}) && lv.after(b, x.Block, x.Inst) {
			return true
		}
	}
	return false
}

// A lent parameter's storage is left to the callers when they read this body's summary.
func (w *shareFacts) returnCopies(a LocalID) bool {
	if w.aliasOfNamed(a) || !w.writable(a) {
		return false
	}
	rs := w.rootsOf(a)
	if rs.other {
		return true
	}
	for _, r := range rs.refs {
		if w.p.lentLocal(w.f, r.l) && !w.sum.retView[w.f] {
			return true
		}
	}
	return false
}
