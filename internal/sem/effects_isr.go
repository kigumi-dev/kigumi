package sem

// checkInterruptHandlers uses inferSummaries' own fixpoint shape over the edge graph, so mutual recursion settles without a cycle-aware DFS.
func (r *Result) checkInterruptHandlers() {
	handlers := r.interruptHandlers()
	if len(handlers) == 0 {
		return
	}
	safe := r.inferIsrSafe(r.effectNodes())
	for _, id := range handlers {
		if info := r.Fn(id); !info.Naked && !safe[id] {
			r.reportIsrPath(id, safe)
		}
	}
}

func (r *Result) interruptHandlers() []EntityID {
	var handlers []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntFn && e.Flags&EfPoison == 0 && r.Fn(EntityID(id)).Export == "interrupt" {
			handlers = append(handlers, EntityID(id))
		}
	}
	return handlers
}

// isrFacts are the operations every strict boundary rejects outright;
// FactSync is checked separately since a verified-safe primitive is allowed.
var isrFacts = [...]OpFact{FactAlloc, FactRelease, FactBlock, FactSuspend, FactPanic, FactIO, FactSpawn}

// isrSafeOp reads the already-converged shared OpSummary instead of walking
// the edge graph again; Unresolved has no caller here, so it is unsafe.
func (r *Result) isrSafeOp(s OpSummary) bool {
	if s.Unresolved {
		return false
	}
	for _, f := range isrFacts {
		if s.Facts[f].Maybe {
			return false
		}
	}
	sy := s.Facts[FactSync]
	return !sy.Maybe || !sy.InterruptUnsafe
}

func (r *Result) inferIsrSafe(nodes []EntityID) map[EntityID]bool {
	safe := make(map[EntityID]bool, len(nodes))
	for _, id := range nodes {
		if fixed, ok := r.isrFixed(id); ok {
			safe[id] = fixed
			continue
		}
		safe[id] = r.isrSafeOp(r.opSummary(id))
	}
	return safe
}

// isrFixed reports fixpoint-free ISR safety: non-bodied entities are inert, a bodiless fn only when declared noalloc.
func (r *Result) isrFixed(id EntityID) (bool, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return false, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return true, true
	}
	if r.Fn(id).Body != 0 {
		return false, false
	}
	return r.Fn(id).Declared&EffNoalloc != 0, true
}

// isrTargetSafe is isrFixed/safe for a call's resolved target; 0 means unresolved.
func (r *Result) isrTargetSafe(target EntityID, safe map[EntityID]bool) bool {
	if target == 0 {
		return false
	}
	if v, ok := safe[target]; ok {
		return v
	}
	if fixed, ok := r.isrFixed(target); ok {
		return fixed
	}
	return false
}

// isrDropSafe mirrors computeDrop's shape for the ISR dimension; memo breaks self-referential cycles.
func (r *Result) isrDropSafe(t TypeID, safe map[EntityID]bool) bool {
	return r.isrDropSafeMemo(t, safe, map[TypeID]bool{})
}

func (r *Result) isrDropSafeMemo(t TypeID, safe map[EntityID]bool, memo map[TypeID]bool) bool {
	if t == 0 {
		return true
	}
	if v, ok := memo[t]; ok {
		return v
	}
	memo[t] = true
	tt := r.Types
	n := tt.Node(t)
	switch n.Kind {
	case KIface:
		memo[t] = false
		return false
	case KClosure:
		if n.Ent == 0 {
			return true
		}
		for _, cap := range r.closure(n.Ent).Captures {
			if !r.isrDropSafeMemo(r.Entities[cap.Local].Type, safe, memo) {
				memo[t] = false
				return false
			}
		}
		return true
	// An unconstrained type parameter may be instantiated with anything, so this doesn't default to safe like computeDrop.
	case KParam:
		memo[t] = false
		return false
	case KNamed:
	default:
		return true
	}
	info := r.typeDecl(n.Ent)
	if info.Form == FormResource && info.Drop != 0 && !r.isrTargetSafe(info.Drop, safe) {
		memo[t] = false
		return false
	}
	if r.isSharedCell(n.Ent) {
		memo[t] = false
		return false
	}
	subst := map[EntityID]TypeID{}
	for i, p := range info.Params {
		if i < len(n.Args) {
			subst[p] = n.Args[i]
		}
	}
	for _, a := range n.Args {
		if !r.isrDropSafeMemo(a, safe, memo) {
			memo[t] = false
			return false
		}
	}
	for _, f := range info.Fields {
		if !r.isrDropSafeMemo(tt.Subst(r.Entities[f].Type, subst), safe, memo) {
			memo[t] = false
			return false
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if !r.isrDropSafeMemo(tt.Subst(p, subst), safe, memo) {
				memo[t] = false
				return false
			}
		}
	}
	return true
}
