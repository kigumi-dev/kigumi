package sem

import "slices"

// settleEnvCaptures drops the erasure boxes of lambdas needing no runtime type: a body needs one when it composes a
// type naming its parameters, calls such a body at an instantiation naming them, or builds a lambda that does.
func (r *Result) settleEnvCaptures(nodes []EntityID) {
	kids := map[EntityID][]EntityID{}
	for _, id := range nodes {
		if r.Entities[id].Kind == EntClosure {
			kids[r.closure(id).Creator] = append(kids[r.closure(id).Creator], id)
		}
	}
	r.envUse.lanes = r.settleInstanceUse(nodes, kids, r.lowersLanes)
	needs := r.settleInstanceUse(nodes, kids, r.composesTypes)
	for _, id := range nodes {
		edges := r.edgesPtr(id)
		*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool {
			return e.Why == whyEnvErase && !needs[r.Types.Node(e.From).Ent]
		})
	}
	r.envUse.needs = needs
}

// CapturesRuntimeTypes reports whether lambda cl, or the lambda it was copied from, reads a runtime type it must
// capture: what the backends' environment pass may add to it, and sem charged for.
func (r *Result) CapturesRuntimeTypes(cl EntityID) bool { return r.envUse.needs[r.ClosureOrigin(cl)] }

// DropReadsInstance reports whether a record of type decl ent carries its instantiation for its drop: the
// environment pass composes it at exactly these records, and sem charges each body building one for it.
func (r *Result) DropReadsInstance(ent EntityID) bool {
	return r.Entities[ent].Kind == EntType && r.dropReadsInstance(r.typeDecl(ent).Drop, r.envUse.needs)
}

// dropReadsInstance reports whether drop reads the instantiation of the value it drops: a const parameter, a SIMD
// lane shape, or a runtime type, given needs.
func (r *Result) dropReadsInstance(drop EntityID, needs map[EntityID]bool) bool {
	if drop == 0 || r.Fn(drop).Body == 0 {
		return false
	}
	return len(r.Fn(drop).ConstParams) > 0 || r.envUse.lanes[drop] || needs[drop]
}

// ownUse reports whether body id's own operations read its instantiation, given set, the bodies found so far.
type ownUse func(id EntityID, names func(TypeID) bool, set map[EntityID]bool) bool

// settleInstanceUse finds the bodies reading their instantiation: by their own operations, or by calling, making a
// value of, dropping or building a lambda of one that does at a type naming their parameters.
func (r *Result) settleInstanceUse(nodes []EntityID, kids map[EntityID][]EntityID, own ownUse) map[EntityID]bool {
	set := map[EntityID]bool{}
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			if !set[id] && r.readsInstance(id, set, kids[id], own) {
				set[id], changed = true, true
			}
		}
	}
	return set
}

func (r *Result) readsInstance(id EntityID, set map[EntityID]bool, kids []EntityID, own ownUse) bool {
	params := r.runtimeParams(id)
	if len(params) == 0 {
		return false
	}
	seen := map[EntityID]bool{}
	names := func(t TypeID) bool {
		clear(seen)
		return r.namesAny(t, params, seen)
	}
	if own(id, names, set) || slices.ContainsFunc(kids, func(k EntityID) bool { return set[k] }) {
		return true
	}
	for _, call := range r.envUse.calls[id] {
		if set[call.fn] && slices.ContainsFunc(call.args, names) {
			return true
		}
	}
	for _, e := range r.edgesOf(id) {
		switch {
		case e.Kind == EdgeDrop && names(e.Type) && set[r.dropOf(e.Type)]:
			return true
		case e.Kind == EdgeWitness && set[e.Target]:
			return true
		}
	}
	return false
}

// composesTypes: a type test, lent box, box or downcast of a type naming the parameters, or a record of one whose
// drop reads its instantiation.
func (r *Result) composesTypes(id EntityID, names func(TypeID) bool, needs map[EntityID]bool) bool {
	if slices.ContainsFunc(r.envUse.ops[id], names) {
		return true
	}
	for _, call := range r.envUse.calls[id] {
		if r.IsErrorAs(call.fn) && slices.ContainsFunc(call.args, names) {
			return true
		}
	}
	for _, t := range r.envUse.builds[id] {
		if names(t) && r.dropReadsInstance(r.dropOf(t), needs) {
			return true
		}
	}
	return slices.ContainsFunc(r.edgesOf(id), func(e EffectEdge) bool {
		return e.Kind == EdgeAlloc && e.From != 0 && e.Why != whyEnvErase && names(e.From)
	})
}

// lowersLanes: a std/simd call at a type naming the parameters, or a box or record of one whose method table or
// drop lowers such a call.
func (r *Result) lowersLanes(id EntityID, names func(TypeID) bool, lanes map[EntityID]bool) bool {
	for _, call := range r.envUse.calls[id] {
		if r.inStdSimd(call.fn) && slices.ContainsFunc(call.args, names) {
			return true
		}
	}
	for _, t := range r.envUse.builds[id] {
		if names(t) && lanes[r.dropOf(t)] {
			return true
		}
	}
	pkg := r.Entities[id].Pkg
	return slices.ContainsFunc(r.edgesOf(id), func(e EffectEdge) bool {
		if e.Kind != EdgeAlloc || e.From == 0 || r.Types.Kind(e.Type) != KIface || !names(e.From) {
			return false
		}
		return slices.ContainsFunc(r.Witnesses(e.From, e.Type, pkg), func(w EntityID) bool { return lanes[w] })
	})
}

// runtimeParams are the parameters body id knows only at runtime: a generic fn's own, or what a lambda named
// from outside it.
func (r *Result) runtimeParams(id EntityID) map[EntityID]bool {
	if r.Entities[id].Kind == EntClosure {
		return r.envUse.outer[id]
	}
	if r.Entities[id].Kind != EntFn {
		return nil
	}
	info := r.Fn(id)
	out := map[EntityID]bool{}
	for _, p := range append(append([]EntityID(nil), info.TypeParams...), info.RecvParams...) {
		if !r.typeParam(p).IsLifetime {
			out[p] = true
		}
	}
	return out
}

// IsErrorAs reports whether fn is std/error's `as`, whose instantiation the runtime reads back as a type.
func (r *Result) IsErrorAs(fn EntityID) bool { return fn != 0 && fn == r.langItem(0, 0, "as") }

// dropOf is the drop fn of t's declaration, 0 for none.
func (r *Result) dropOf(t TypeID) EntityID {
	if n := r.Types.Node(t); n.Kind == KNamed && r.Entities[n.Ent].Kind == EntType {
		return r.typeDecl(n.Ent).Drop
	}
	return 0
}
