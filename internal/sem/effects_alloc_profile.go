package sem

import "strings"

// isTryName reports a name in the fallible family: try followed by an upper-case letter.
func isTryName(name string) bool {
	return len(name) > 3 && strings.HasPrefix(name, "try") && name[3] >= 'A' && name[3] <= 'Z'
}

// fatalConveniences keys, like stdKey, the std functions with a try sibling in
// the same package and owner (push beside tryPush); they end the program on refusal.
func (r *Result) fatalConveniences() map[string]bool {
	if r.fatalFamily != nil {
		return r.fatalFamily
	}
	family := map[string]bool{}
	for id := range r.Entities {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&EfStd == 0 || !isTryName(e.Name) {
			continue
		}
		plain := strings.ToLower(e.Name[3:4]) + e.Name[4:]
		key := r.stdKey(EntityID(id))
		family[strings.TrimSuffix(key, e.Name)+plain] = true
	}
	r.fatalFamily = family
	return family
}

// reservedTry lists the bodied std try functions inference cannot prove: a push after a reserve,
// a callback forwarded to a fallible primitive, a `?` that rebuilds the Err. A two-call fault sweep fixture covers each.
var reservedTry = map[string]bool{
	"array.Array.tryFill": true, "array.Array.tryPush": true, "array.Array.tryCloneWith": true,
	"array.Array.tryCopyRange": true, "array.Array.tryReverse": true, "array.Array.tryConcat": true,
	"array.Array.tryAppendAll": true, "map.Map.trySet": true, "array.tryJoin": true, "prelude.Bytes.tryFill": true,
	"prelude.String.tryTrim": true, "prelude.String.trySplit": true, "prelude.String.tryConcat": true,
	"prelude.String.tryToUpper": true, "prelude.String.tryToLower": true, "text.tryReplace": true,
	"text.tryLines": true, "text.tryGraphemes": true, "text.tryTrimPrefix": true, "text.tryTrimSuffix": true,
	"text.StringBuilder.tryPush": true, "text.StringBuilder.tryPushByte": true,
}

// isTrustedTry marks a bodied std fn on the reservedTry list; any other bodied try* is inferred like user code.
func (r *Result) isTrustedTry(id EntityID) bool {
	e := &r.Entities[id]
	return e.Flags&EfStd != 0 && isTryName(e.Name) && reservedTry[r.stdKey(id)]
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
			if ok := r.recoverableBodySafe(id, safe); ok != safe[id] {
				safe[id] = ok
				changed = true
			}
		}
	}
	return safe
}

func (r *Result) recoverableBodySafe(id EntityID, safe map[EntityID]bool) bool {
	if _, _, _, bad := r.badSiteOf(id, safe); bad {
		return false
	}
	for _, e := range r.edgesOf(id) {
		if !r.recoverableEdgeSafe(e, safe) {
			return false
		}
	}
	return true
}

// recoverableFixed mirrors isrFixed. A requirement keeps its declared contract; a bodiless fn is safe
// unless it is a fatal convenience or `mem.uninit`, which ends the program when its cell is refused.
func (r *Result) recoverableFixed(id EntityID) (bool, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return false, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return true, true
	}
	if noalloc, ok := r.boundContract(id); ok {
		return noalloc, true
	}
	if e.Flags&EfStd != 0 && (r.fatalConveniences()[r.fatalAllocKey(id)] || r.storageFn(id) == "Uninit") {
		return false, true
	}
	if r.Fn(id).Body != 0 {
		if r.isTrustedTry(id) {
			return true, true
		}
		return false, false
	}
	return true, true
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

// recoverableEdgeSafe: a bound call is judged at its witness site, and a callee with no body to read
// passes only when declared `noalloc`.
func (r *Result) recoverableEdgeSafe(e EffectEdge, safe map[EntityID]bool) bool {
	switch e.Kind {
	case EdgeAlloc:
		return r.profileDeferred(e)
	case EdgeCall, EdgeWitness, EdgeWitnessCall:
		return r.boundLocal(e) != 0 || r.recoverableTargetSafe(e.Target, safe)
	case EdgeCallback:
		if e.Target == 0 {
			return e.Why == whyWitnessCallback || e.Mods&EffNoalloc != 0
		}
		return r.recoverableTargetSafe(e.Target, safe)
	case EdgeParamCall:
		return false
	}
	return true
}

// checkRecoverableOnly enforces the recoverable-only profile at every
// root: the first edge or witness site reaching a fatal allocation path is reported.
func (r *Result) checkRecoverableOnly(nodes []EntityID) {
	roots := r.allocRoots()
	if len(roots) == 0 {
		return
	}
	r.markBoundSlots(nodes)
	safe := r.inferRecoverableSafe(nodes)
	r.recoverable.safe = safe
	for _, root := range roots {
		if !safe[root] {
			r.reportRecoverableRoot(root, safe)
		}
	}
}

// reportRecoverableRoot reports root's first failing check.
func (r *Result) reportRecoverableRoot(root EntityID, safe map[EntityID]bool) {
	f, e, found := r.firstFault(root, safe)
	if !found {
		return
	}
	if f != nil {
		r.reportSiteFault(f, "`"+r.Entities[f.site.fn].Name+"`", ", which calls", f.site.file, f.site.node)
		return
	}
	if r.reportStorageAlloc(e) || r.reportBoundCall(root, e, safe) {
		return
	}
	what, fix := "this allocation", "the fallible (`try`-prefixed) form"
	switch {
	case e.Kind == EdgeAlloc && e.Why == whyWitnessBind:
		what, fix = "binding the witnesses of `"+r.implName(e.Target)+"` for this call", "a type argument whose impl takes no witnesses (a non-generic type)"
	case (e.Kind == EdgeCall || e.Kind == EdgeCallback) && e.Target != 0:
		name := r.Entities[e.Target].Name
		what = "`" + name + "`"
		if r.Entities[e.Target].Flags&EfStd != 0 && r.fatalConveniences()[r.fatalAllocKey(e.Target)] {
			fix = "the fallible form `try" + strings.ToUpper(name[:1]) + name[1:] + "`"
		}
	}
	r.errAt(e.File, e.Node, cRecoverableOnlyFatalAlloc, what, fix)
}
