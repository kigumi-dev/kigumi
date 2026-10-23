package sem

// checkCtxRoots reports a real requirement reaching a program root, a drop or a foreign entry; a phantom-only key stays unbound.
func (r *Result) checkCtxRoots(nodes []EntityID, sets map[EntityID]map[EntityID]ctxNodeRef, real map[EntityID]map[EntityID]bool) {
	for _, id := range nodes {
		root := r.isCtxRoot(id)
		drop := r.isDropFn(id)
		entry := r.foreignEntryKind(id)
		if !root && !drop && entry == "" {
			continue
		}
		for _, key := range r.Fn(id).CtxRequires {
			if !real[id][key] {
				continue
			}
			ref := sets[id][key]
			switch {
			case root:
				r.errAt(ref.File, ref.Node, cCtxUnboundKey, key)
			case drop:
				r.errAt(ref.File, ref.Node, cDropHiddenArg, r.Entities[r.Fn(id).Owner].Name+".drop", "context key `"+r.Entities[key].Name+"`")
			default:
				r.errAt(ref.File, ref.Node, cCtxForeignEntry, entry+" `"+r.Entities[id].Name+"`", key, "entered from outside the program")
			}
		}
	}
}

// isDropFn: drop glue at scope ends, reassignments, container releases and unwinds carries no provider.
func (r *Result) isDropFn(id EntityID) bool {
	info := r.Fn(id)
	if r.Entities[id].Kind != EntFn || info.Owner == 0 || r.Entities[info.Owner].Kind != EntType {
		return false
	}
	return r.typeDecl(info.Owner).Drop == id
}

// foreignEntryKind names how id is entered without a call from main, or "".
func (r *Result) foreignEntryKind(id EntityID) string {
	e := &r.Entities[id]
	if e.Kind != EntFn {
		return ""
	}
	switch r.Fn(id).Export {
	case "interrupt":
		return "interrupt handler"
	case "":
	default:
		return "exported function"
	}
	if bp := r.mod.BuildProgram; bp != "" && e.Name == "configure" && e.Parent == 0 && r.Packages[e.Pkg].Path == bp {
		return "build program entry"
	}
	return ""
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
