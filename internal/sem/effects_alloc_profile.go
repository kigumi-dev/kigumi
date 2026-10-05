package sem

// fatalAllocPrimitives lists bodiless std fns with no fallible sibling
// whose native/VM body terminates on refused allocation, keyed like mir.StdKey.
var fatalAllocPrimitives = map[string]bool{
	"array.Array.push":         true,
	"prelude.Bytes.zeros":      true,
	"prelude.Bytes.fill":       true,
	"prelude.Bytes.fromArray":  true,
	"prelude.Bytes.concat":     true,
	"prelude.String.fromBytes": true,
	"alloc.Shared.new":         true,
	"mem.secureErase":          true,
}

// trustedFallibleWrappers lists bodied std fns proven safe by a preceding
// reservation — a manual stand-in for edge-graph path-sensitivity.
var trustedFallibleWrappers = map[string]bool{
	"array.Array.tryPush":        true,
	"array.Array.tryReserve":     true,
	"array.Array.tryAppendAll":   true,
	"array.Array.tryFill":        true,
	"map.Map.trySet":             true,
	"text.StringBuilder.tryPush": true,
}

func (r *Result) fatalAllocKey(id EntityID) string { return r.stdKey(id) }

// inferRecoverableSafe is checkInterruptHandlers' fixpoint shape over a
// different predicate: true means no reachable OOM non-return termination.
func (r *Result) inferRecoverableSafe(nodes []EntityID) map[EntityID]bool {
	safe := make(map[EntityID]bool, len(nodes))
	var inferred []EntityID
	for _, id := range nodes {
		if fixed, ok := r.recoverableFixed(id); ok {
			safe[id] = fixed
			continue
		}
		inferred = append(inferred, id)
		safe[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, id := range inferred {
			ok := true
			for _, e := range r.edgesOf(id) {
				if !r.recoverableEdgeSafe(e, safe) {
					ok = false
					break
				}
			}
			if ok != safe[id] {
				safe[id] = ok
				changed = true
			}
		}
	}
	return safe
}

// recoverableFixed mirrors isrFixed: non-bodied entities are inert, and a
// bodiless fn is safe unless the registry marks it a fatal convenience.
func (r *Result) recoverableFixed(id EntityID) (bool, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return false, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return true, true
	}
	if r.Fn(id).Body != 0 {
		if trustedFallibleWrappers[r.fatalAllocKey(id)] {
			return true, true
		}
		return false, false
	}
	return !fatalAllocPrimitives[r.fatalAllocKey(id)], true
}

func (r *Result) recoverableTargetSafe(target EntityID, safe map[EntityID]bool) bool {
	if target == 0 {
		return false
	}
	if v, ok := safe[target]; ok {
		return v
	}
	if fixed, ok := r.recoverableFixed(target); ok {
		return fixed
	}
	return false
}

// recoverableEdgeSafe treats EdgeAlloc as always fatal (no fallible path
// yet) and leaves EdgePanic alone; an unproven dynamic callee counts unsafe.
func (r *Result) recoverableEdgeSafe(e EffectEdge, safe map[EntityID]bool) bool {
	switch e.Kind {
	case EdgeAlloc:
		return false
	case EdgeCall:
		return r.recoverableTargetSafe(e.Target, safe)
	case EdgeCallback:
		return e.Target != 0 && r.recoverableTargetSafe(e.Target, safe)
	case EdgeWitness, EdgeParamCall, EdgeWitnessCall:
		return false
	}
	return true
}

// checkRecoverableOnly enforces the recoverable-only profile at the
// entry: the first edge reaching a fatal allocation path is reported.
func (r *Result) checkRecoverableOnly(nodes []EntityID) {
	entry := r.MainFn()
	if entry == 0 {
		return
	}
	safe := r.inferRecoverableSafe(nodes)
	if safe[entry] {
		return
	}
	for _, e := range r.edgesOf(entry) {
		if r.recoverableEdgeSafe(e, safe) {
			continue
		}
		what := "this allocation"
		if (e.Kind == EdgeCall || e.Kind == EdgeCallback) && e.Target != 0 {
			what = "`" + r.Entities[e.Target].Name + "`"
		}
		r.errAt(e.File, e.Node, cRecoverableOnlyFatalAlloc, what)
		return
	}
}
