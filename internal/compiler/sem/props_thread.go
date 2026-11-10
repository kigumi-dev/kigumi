package sem

// Send/Sync: raw pointers, erased fn values, existential boxes and executor handles
// never qualify; a resource or opaque type needs an explicit @unsafeGuarantee.

func (r *Result) isSend(t TypeID) bool { return r.threadProp(t, false) }
func (r *Result) isSync(t TypeID) bool { return r.threadProp(t, true) }

type threadKey struct {
	t    TypeID
	sync bool
}

// threadState: closure marks a computation that read a closure's captures, which a later write or a context binding
// can still extend until checkThreadBounds sets settled; until then its true lasts one top-level query (transient).
type threadState struct {
	open      map[threadKey]int
	low       int
	closure   bool
	settled   bool
	transient map[threadKey]bool
	reads     map[TypeID][]ReadSite
	checks    []pendingThread
}

// a type under computation counts as qualifying, so recursion resolves via the other parts;
// such a provisional true is not memoized, since the open type may still prove false.
func (r *Result) threadProp(t TypeID, sync bool) bool {
	if t == 0 {
		return true
	}
	p := &r.Types.props[t]
	memo := &p.send
	if sync {
		memo = &p.sync
	}
	switch *memo {
	case 1:
		return true
	case -1:
		return false
	}
	st := &r.thread
	key := threadKey{t, sync}
	if ok, seen := st.transient[key]; seen {
		return ok
	}
	if depth, open := st.open[key]; open {
		st.low = min(st.low, depth)
		return true
	}
	if st.open == nil {
		st.open = map[threadKey]int{}
	}
	depth := len(st.open)
	st.open[key] = depth
	outerLow, outerClosure := st.low, st.closure
	st.low, st.closure = depth, false
	ok := r.computeThread(t, sync)
	low, closure := st.low, st.closure
	delete(st.open, key)
	st.low, st.closure = min(outerLow, low), outerClosure || closure
	switch {
	case !ok:
		*memo = -1
	case low >= depth && (!closure || st.settled):
		*memo = 1
	case low >= depth:
		if st.transient == nil {
			st.transient = map[threadKey]bool{}
		}
		st.transient[key] = true
	}
	if len(st.open) == 0 {
		st.transient = nil
	}
	return ok
}

func (r *Result) computeThread(t TypeID, sync bool) bool {
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KPoison, KUntyped, KPrim:
		return true
	case KRef:
		// A borrow cannot be sent (v2.5); it is shareable when its target is.
		return sync && r.threadProp(n.Elem, true)
	case KParam:
		want := CSend
		if sync {
			want = CSync
		}
		for _, c := range r.typeParam(n.Ent).Constraints {
			if c.Kind == want {
				return true
			}
		}
		return false
	case KClosure:
		r.thread.closure = true
		info := r.closure(n.Ent)
		for _, cap := range info.Captures {
			if roleOf(cap) == roleCell || !r.threadProp(r.Entities[cap.Local].Type, sync) {
				return false
			}
		}
		return true
	case KNamed:
	default:
		return false
	}
	e := &r.Entities[n.Ent]
	info := r.typeDecl(n.Ent)
	// Future is never Send/Sync: its TypeID carries only the async fn's return type,
	// not the captured parameters, so there is nothing sound to check.
	if n.Ent == tt.futureEnt {
		return false
	}
	// A Task handle is a control block the scheduler already shares safely
	// across lanes, so it carries only its result's thread property.
	if n.Ent == r.taskEntity() && len(n.Args) == 1 {
		return r.threadProp(n.Args[0], sync)
	}
	// A predeclared opaque type (e.File == 0) has no declaration to attach
	// @unsafeGuarantee to, so it falls through to the structural check below.
	if info.Form == FormResource || (info.Form == FormOpaque && e.File != 0) {
		prop := ugSendProp
		if sync {
			prop = ugSyncProp
		}
		return r.evalGuarantee(info, n.Args, prop)
	}
	if info.Form == FormDefined {
		return r.threadProp(tt.Subst(info.Underlying, instSubst(info.Params, n.Args)), sync)
	}
	for _, a := range n.Args {
		if !r.threadProp(a, sync) {
			return false
		}
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, fld := range info.Fields {
		if !r.threadProp(tt.Subst(r.Entities[fld].Type, subst), sync) {
			return false
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if !r.threadProp(tt.Subst(p, subst), sync) {
				return false
			}
		}
	}
	return true
}
