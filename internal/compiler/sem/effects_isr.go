package sem

// checkInterruptHandlers judges each handler, naked or not, from the shared OpSummary under its own resolved permissions.
func (r *Result) checkInterruptHandlers() {
	handlers := r.interruptHandlers()
	r.checkInterruptConfig(handlers)
	if len(handlers) == 0 {
		return
	}
	for _, sc := range r.isrScopes(handlers, r.effectNodes()) {
		if !sc.safe[sc.handler] {
			r.reportIsrPath(sc)
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

// isrFacts are the operations no configuration can permit; FactIO and FactSync pass only as permitted contracts.
var isrFacts = [...]OpFact{FactAlloc, FactRelease, FactBlock, FactSuspend, FactPanic, FactSpawn}

// summaryOK reads the converged shared OpSummary; Unresolved has no caller here, so it is unsafe.
func (sc *isrScope) summaryOK(s OpSummary) bool {
	if s.Unresolved || s.Untrusted {
		return false
	}
	for _, f := range isrFacts {
		if s.Facts[f].Maybe {
			return false
		}
	}
	for _, f := range [...]OpFact{FactIO, FactSync} {
		if st := s.Facts[f]; st.Maybe && (!st.Contract || st.Cause.Unknown) {
			return false
		}
	}
	return sc.permitsAll(s.Contracts)
}

func (sc *isrScope) permitsAll(contracts string) bool {
	for _, id := range splitContracts(contracts) {
		if !sc.permits[id] {
			return false
		}
	}
	return true
}

func (r *Result) inferIsrSafe(nodes []EntityID, sc *isrScope) map[EntityID]bool {
	safe := make(map[EntityID]bool, len(nodes))
	for _, id := range nodes {
		if fixed, ok := r.isrFixed(id, sc); ok {
			safe[id] = fixed
			continue
		}
		safe[id] = sc.summaryOK(r.opSummary(id))
	}
	return safe
}

// isrFixed reports fixpoint-free ISR safety: non-bodied entities are inert, a bodiless fn is judged by its fixed summary.
func (r *Result) isrFixed(id EntityID, sc *isrScope) (bool, bool) {
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
	s, _ := r.opFixedSummary(id)
	return sc.summaryOK(s), true
}

// isrTargetSafe is isrFixed/safe for a call's resolved target; 0 means unresolved.
func (r *Result) isrTargetSafe(target EntityID, sc *isrScope) bool {
	if target == 0 {
		return false
	}
	if v, ok := sc.safe[target]; ok {
		return v
	}
	if fixed, ok := r.isrFixed(target, sc); ok {
		return fixed
	}
	return false
}

// isrDropSafe mirrors computeDrop's shape for the ISR dimension; memo breaks self-referential cycles.
func (r *Result) isrDropSafe(t TypeID, sc *isrScope) bool {
	return r.isrDropSafeMemo(t, sc, map[TypeID]bool{})
}

func (r *Result) isrDropSafeMemo(t TypeID, sc *isrScope, memo map[TypeID]bool) bool {
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
			if !r.isrDropSafeMemo(r.Entities[cap.Local].Type, sc, memo) {
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
	if info.Form == FormResource && info.Drop != 0 && !r.isrTargetSafe(info.Drop, sc) {
		memo[t] = false
		return false
	}
	if r.isHeapCell(n.Ent) {
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
		if !r.isrDropSafeMemo(a, sc, memo) {
			memo[t] = false
			return false
		}
	}
	for _, f := range info.Fields {
		if !r.isrDropSafeMemo(tt.Subst(r.Entities[f].Type, subst), sc, memo) {
			memo[t] = false
			return false
		}
	}
	for _, v := range info.Variants {
		for _, p := range r.variant(v).Payload {
			if !r.isrDropSafeMemo(tt.Subst(p, subst), sc, memo) {
				memo[t] = false
				return false
			}
		}
	}
	return true
}
