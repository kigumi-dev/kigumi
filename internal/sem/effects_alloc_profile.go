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

// recoverableFixed mirrors isrFixed: non-bodied entities are inert, and a bodiless
// fn is safe unless it is a fatal convenience or `mem.uninit`, which ends the program when its cell is refused.
func (r *Result) recoverableFixed(id EntityID) (bool, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return false, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return true, true
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

// recoverableEdgeSafe treats EdgeAlloc as always fatal (no fallible path
// yet) and leaves EdgePanic alone; an unproven dynamic callee counts unsafe.
func (r *Result) recoverableEdgeSafe(e EffectEdge, safe map[EntityID]bool) bool {
	switch e.Kind {
	case EdgeAlloc:
		return r.profileDeferred(e)
	case EdgeCall:
		return r.recoverableTargetSafe(e.Target, safe)
	case EdgeCallback:
		return e.Target != 0 && r.recoverableTargetSafe(e.Target, safe)
	case EdgeWitness, EdgeParamCall, EdgeWitnessCall:
		return false
	}
	return true
}

// checkRecoverableOnly enforces the recoverable-only profile at every
// root: the first edge reaching a fatal allocation path is reported.
func (r *Result) checkRecoverableOnly(nodes []EntityID) {
	roots := r.allocRoots()
	if len(roots) == 0 {
		return
	}
	safe := r.inferRecoverableSafe(nodes)
	r.recoverableSafe = safe
	for _, root := range roots {
		if safe[root] {
			continue
		}
		for _, e := range r.edgesOf(root) {
			if r.recoverableEdgeSafe(e, safe) {
				continue
			}
			if r.reportStorageAlloc(e) {
				break
			}
			what, fix := "this allocation", "the fallible (`try`-prefixed) form"
			if (e.Kind == EdgeCall || e.Kind == EdgeCallback) && e.Target != 0 {
				name := r.Entities[e.Target].Name
				what = "`" + name + "`"
				if r.Entities[e.Target].Flags&EfStd != 0 && r.fatalConveniences()[r.fatalAllocKey(e.Target)] {
					fix = "the fallible form `try" + strings.ToUpper(name[:1]) + name[1:] + "`"
				}
			}
			r.errAt(e.File, e.Node, cRecoverableOnlyFatalAlloc, what, fix)
			break
		}
	}
}
