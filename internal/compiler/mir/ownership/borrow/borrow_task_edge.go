package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// onEdge drops the handles of a value branch e shows empty (an option found empty, an array a `for` loop emptied); a
// handle nothing holds any more on that path counts as joined there.
func (r *taskRun) onEdge(st taskState, term midir.Term, e int) taskState {
	if term.Op != midir.TermBranch || len(term.Args) == 0 {
		return st
	}
	x, ok := r.emptyOn(term.Args[0], e == 0)
	if !ok || st.present[x].empty() {
		return st
	}
	out := st.clone()
	gone := r.handleBits(out.present[x])
	for l, v := range out.present {
		if l == x || r.readFrom(l, x) {
			out.present[l] = v.minus(gone)
		}
	}
	for _, v := range out.present {
		gone = gone.minus(v)
	}
	out.joined = out.joined.or(gone)
	return out
}

// readFrom reports a local read out of x, through copies: on a path where x holds nothing, it holds nothing of x either.
func (r *taskRun) readFrom(l, x midir.LocalID) bool {
	for range 8 {
		defs := r.defs[l]
		if len(defs) != 1 || len(defs[0].Args) == 0 {
			return false
		}
		switch d := defs[0]; d.Op {
		case midir.OpCopy, midir.OpMove, midir.OpAlias:
			l = d.Args[0]
		case midir.OpPayload, midir.OpField, midir.OpFieldMove, midir.OpIndex:
			return d.Args[0] == x
		case midir.OpBuiltin:
			return d.Str == "iter.at_move" && d.Args[0] == x
		default:
			return false
		}
	}
	return false
}

// emptyOn is the value branch condition cond, taken as when, shows to hold no handle.
func (r *taskRun) emptyOn(cond midir.LocalID, when bool) (midir.LocalID, bool) {
	defs := r.defs[cond]
	if len(defs) != 1 {
		return 0, false
	}
	d := defs[0]
	switch {
	case d.Op == midir.OpIsVariant && len(d.Args) == 1:
		return d.Args[0], r.variantEmpty(r.g.Locals[d.Args[0]].Type, d.Ent, when)
	case d.Op == midir.OpBinary && d.Str == "<" && len(d.Args) == 2 && !when:
		if ls := r.defs[d.Args[1]]; len(ls) == 1 && ls[0].Op == midir.OpBuiltin && ls[0].Str == "iter.len" && len(ls[0].Args) == 1 {
			return ls[0].Args[0], true
		}
	}
	return 0, false
}

// variantEmpty reports whether a value of t is known to hold no handle once it is (or, when not is, is not) variant v.
func (r *taskRun) variantEmpty(t sem.TypeID, v sem.EntityID, is bool) bool {
	n := r.c.p.R.Types.Node(t)
	if n.Kind != sem.KNamed || r.c.p.R.Entity(n.Ent).Kind != sem.EntType {
		return false
	}
	d := r.c.p.R.TypeDecl(n.Ent)
	if d == nil || d.Form != sem.FormAdt {
		return false
	}
	for _, w := range d.Variants {
		if (w == v) == is && len(r.c.p.R.Variant(w).Payload) > 0 {
			return false
		}
	}
	return true
}
