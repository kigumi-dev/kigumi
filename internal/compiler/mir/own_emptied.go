package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// emptied reports whether every field of root's record that owns anything was moved out on every path.
func (a *owner) emptied(root midir.LocalID, st *dstate) bool {
	kids := a.pl.children[root]
	t := a.f.Locals[root].Type
	if len(kids) == 0 || a.p.R.Types.Kind(t) != sem.KNamed {
		return false
	}
	ent := a.p.R.Types.Node(t).Ent
	if a.p.R.Entity(ent).Kind != sem.EntType || a.p.R.TypeDecl(ent).Form != sem.FormRecord {
		return false
	}
	for i, f := range a.p.R.TypeDecl(ent).Fields {
		rep, ok := kids[i]
		if !ok {
			if !a.p.R.IsCopy(a.p.R.Entity(f).Type) {
				return false
			}
			continue
		}
		if !a.p.R.IsCopy(a.f.Locals[rep].Type) && st.field.get(rep) != ownMoved && st.val.get(rep) != ownMoved {
			return false
		}
	}
	return true
}
