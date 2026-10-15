package sem

import "slices"

// closureCalledParam returns a captured fn-typed parameter the closure invokes whose
// declared type does not promise mod: the closure summary leaves such a call to the callers.
func (r *Result) closureCalledParam(closure EntityID, mod Effects) EntityID {
	for _, e := range r.edgesOf(closure) {
		if e.Kind != EdgeParamCall || e.Target == 0 || r.Entities[e.Target].Kind != EntParam || !r.calledThrough(e.Param) {
			continue
		}
		shape := r.fnShapeOf(r.Entities[e.Target].Type)
		if shape != 0 && Effects(r.Types.Node(shape).Flags)&mod == 0 {
			return e.Target
		}
	}
	return 0
}

// calledThrough reports whether the slot a captured parameter is passed into is invoked by its owner;
// slot 0 is a direct call or a callee without a body.
func (r *Result) calledThrough(slot EntityID) bool {
	if slot == 0 {
		return true
	}
	owner := r.Entities[slot].Parent
	return r.Entities[owner].Kind == EntFn && slices.Contains(r.Fn(owner).CalledParams, slot)
}

func (r *Result) closureParamViolation(target EntityID, mod Effects) (string, bool) {
	if r.Entities[target].Kind != EntClosure {
		return "", false
	}
	p := r.closureCalledParam(target, mod)
	if p == 0 {
		return "", false
	}
	return "calls `" + r.Entities[p].Name + "`, which is not declared `" + modName(mod) + "`", true
}
