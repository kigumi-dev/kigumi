package instance

import "kigumi/internal/compiler/sem"

// closureScope are the parameters closure cl's identity is applied to: those of the generic it is written in,
// then those an enclosing type test opened that its captures or signature name.
func closureScope(r *sem.Result, cl sem.EntityID) []sem.EntityID {
	out := Scope(r, Enclosing(r, cl))
	seen := map[sem.EntityID]bool{}
	add := func(t sem.TypeID) {
		for _, p := range Params(r, t) {
			if _, opened := r.OpenedLocal(p); opened && !seen[p] && !within(r, r.Entity(p).Parent, cl) {
				seen[p] = true
				out = append(out, p)
			}
		}
	}
	info := r.Closure(cl)
	for _, c := range info.Captures {
		add(r.Entity(c.Local).Type)
	}
	add(info.Sig)
	return out
}

func within(r *sem.Result, ent, cl sem.EntityID) bool {
	for ; ent != 0; ent = r.Entity(ent).Parent {
		if ent == cl {
			return true
		}
	}
	return false
}
