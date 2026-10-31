package sem

import "kigumi/internal/compiler/syntax"

// collectStatic creates the entity for a top-level `static`/`static mut` declaration, owned by Kigumi or imported/exported across the C ABI.
func (r *Result) collectStatic(f FileID, d syntax.NodeID, owner StaticOwner, abi string) {
	t := r.tree(f)
	s := staticDecl(t, d)
	id := r.declareTop(f, d, s.Name, EntStatic, r.resolveVis(f, s.Vis, r.packageOf(f)))
	if id == 0 {
		return
	}
	r.Entities[id].Detail = r.addStatic(StaticInfo{
		Mut:   s.Flags&syntax.FlagMut != 0,
		Owner: owner,
		Abi:   abi,
	})
}
