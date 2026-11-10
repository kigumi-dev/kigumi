package sem

// ownsHeap reports whether a value of t may point at heap storage its drop
// frees: every non-Copy value owns its own runtime storage, and a Copy
// String/Bytes (or a Copy aggregate over one) owns the buffer it copies.
// Interrupt terms call that storage a release, so a store that replaces such
// a value touches the allocator.
func (r *Result) ownsHeap(t TypeID) bool {
	return r.ownsHeapMemo(t, map[TypeID]bool{})
}

func (r *Result) ownsHeapMemo(t TypeID, memo map[TypeID]bool) bool {
	if t == 0 {
		return false
	}
	if v, ok := memo[t]; ok {
		return v
	}
	memo[t] = false
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KRef:
		return false
	case KIface, KParam, KClosure:
		// A closure is Copy only when it captures nothing, so its release frees
		// nothing; a capture-carrying one is not Copy and owns its captures.
		return !r.isCopy(t)
	case KPrim:
		return t == TyString || t == TyBytes
	case KNamed:
	default:
		return !r.isCopy(t)
	}
	if !r.isCopy(t) {
		memo[t] = true
		return true
	}
	info := r.typeDecl(n.Ent)
	if info.Form == FormResource || info.Form == FormOpaque {
		memo[t] = true
		return true
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	if info.Form == FormDefined {
		owns := r.ownsHeapMemo(tt.Subst(info.Underlying, subst), memo)
		memo[t] = owns
		return owns
	}
	for _, a := range n.Args {
		if r.ownsHeapMemo(a, memo) {
			memo[t] = true
			return true
		}
	}
	for _, f := range info.Fields {
		if r.ownsHeapMemo(tt.Subst(r.Entities[f].Type, subst), memo) {
			memo[t] = true
			return true
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if r.ownsHeapMemo(tt.Subst(p, subst), memo) {
				memo[t] = true
				return true
			}
		}
	}
	return false
}
