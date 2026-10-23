package sem

import "slices"

// EffectSummary is what a body may do: Pure and Noalloc are the
// contracts, and Default means every allocation in its dynamic extent sits
// inside an `allocator` block.
type EffectSummary struct {
	Pure    bool
	Noalloc bool
	Default bool
	Known   bool
}

// inferDefault settles Default for every body: it starts true and drops to
// false when an unscoped edge needs the ambient allocator.
func (r *Result) inferDefault(nodes []EntityID) {
	for _, id := range nodes {
		r.summaryPtr(id).Default = true
	}
	for changed := true; changed; {
		changed = false
		for _, id := range nodes {
			s := r.summaryPtr(id)
			if !s.Default {
				continue
			}
			for _, e := range r.edgesOf(id) {
				if !r.edgeAllowsDefault(e) {
					s.Default, changed = false, true
					break
				}
			}
		}
	}
}

// defaultOf is a callee's Default: a bodiless function needs the ambient
// allocator unless it is noalloc.
func (r *Result) defaultOf(id EntityID) bool {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return r.closure(id).Summary.Default
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return true
	}
	if r.Fn(id).Body == 0 {
		return r.summary(id).Noalloc
	}
	return r.Fn(id).Summary.Default
}

func (r *Result) edgeAllowsDefault(e EffectEdge) bool {
	if e.Scoped {
		return true
	}
	switch e.Kind {
	case EdgeAlloc:
		return false
	case EdgeCall, EdgeWitness:
		return r.defaultOf(e.Target)
	case EdgeCallback:
		if e.Param != 0 && !slices.Contains(r.Fn(r.Entities[e.Param].Parent).CalledParams, e.Param) {
			return true
		}
		if e.Target != 0 {
			return r.defaultOf(e.Target)
		}
		return e.Mods&EffNoalloc != 0
	}
	return true
}

// Foreign code calls an export(C) function without going through the entry, so each is a root of its own.
func (r *Result) allocRoots() []EntityID {
	var roots []EntityID
	if entry := r.MainFn(); entry != 0 {
		roots = append(roots, entry)
	}
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind != EntFn || e.Flags&EfPoison != 0 || r.Packages[e.Pkg].Std {
			continue
		}
		if info := r.Fn(EntityID(id)); info.Export != "" && info.Body != 0 {
			roots = append(roots, EntityID(id))
		}
	}
	return roots
}

// checkDefaultAllocator enforces `allocator = none` at every root: the first
// edge that reaches the ambient allocator is reported.
func (r *Result) checkDefaultAllocator() {
	for _, root := range r.allocRoots() {
		if r.defaultOf(root) {
			continue
		}
		for _, e := range r.edgesOf(root) {
			if r.edgeAllowsDefault(e) {
				continue
			}
			if r.reportStorageAlloc(e) {
				break
			}
			what := "this allocation"
			if e.Kind == EdgeCall || e.Kind == EdgeWitness || e.Kind == EdgeCallback && e.Target != 0 {
				what = "`" + r.Entities[e.Target].Name + "`"
			}
			r.errAt(e.File, e.Node, cNoDefaultAllocator, what)
			break
		}
	}
}

func effectsText(m Effects) string {
	out := ""
	for _, e := range []struct {
		bit  Effects
		name string
	}{{EffPure, "pure"}, {EffNoalloc, "noalloc"}, {EffAsync, "async"}, {EffUnsafe, "unsafe"}} {
		if m&e.bit != 0 {
			if out != "" {
				out += " "
			}
			out += e.name
		}
	}
	return out
}
