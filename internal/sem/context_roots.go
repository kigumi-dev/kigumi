package sem

// checkCtxRoots reports a requirement that reaches a program root with
// nothing left to supply it; a phantom-only key stays unbound instead.
func (r *Result) checkCtxRoots(nodes []EntityID, sets map[EntityID]map[EntityID]ctxNodeRef, real map[EntityID]map[EntityID]bool) {
	for _, id := range nodes {
		if !r.isCtxRoot(id) {
			continue
		}
		for _, key := range r.Fn(id).CtxRequires {
			if !real[id][key] {
				continue
			}
			ref := sets[id][key]
			r.errAt(ref.File, ref.Node, cCtxUnboundKey, key)
		}
	}
}

// checkCtxValueRefs reports a bare fn-value reference whose target has a
// real requirement: the plain fn type it erases to has no hidden argument slot.
func (r *Result) checkCtxValueRefs() {
	for _, v := range r.ctxValueRefs {
		info := r.Fn(v.target)
		for _, key := range info.CtxRequires {
			if info.CtxReal[key] {
				r.errAt(v.ref.File, v.ref.Node, cCtxValueReal, v.target, key)
				break
			}
		}
	}
}

func (r *Result) isCtxRoot(id EntityID) bool {
	e := &r.Entities[id]
	if e.Kind == EntImplicitMain || e.Kind == EntTest {
		return true
	}
	if e.Kind != EntFn || e.Name != "main" || e.Parent != 0 {
		return false
	}
	p := &r.Packages[e.Pkg]
	return !p.Std && p.Module == ""
}
