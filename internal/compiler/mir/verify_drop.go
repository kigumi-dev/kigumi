package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

func (p *Program) verifyDropOwned(f *Func) *Error {
	lent := p.unownedRefs(f)
	var origin []bool
	if f.rcBase != 0 {
		origin = p.lentOrigins(f, lent)
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op == OpNewCell && !in.Lent && f.rcBase != 0 && lent[in.Args[0]] {
				return fail("droplent", BlockID(bid), i, "wraps %%%s, a reference this frame does not own, in a cell that releases it", f.Locals[in.Args[0]].Name)
			}
			if in.Op != OpDrop && in.Op != OpRelease {
				continue
			}
			if in.Op == OpRelease && f.Async && slices.Contains(f.Params, in.Args[0]) {
				continue
			}
			if len(in.Args) == 1 && (lent[in.Args[0]] || f.Locals[in.Args[0]].Borrowed || (origin != nil && origin[in.Args[0]])) {
				return fail("droplent", BlockID(bid), i, "releases %%%s, a reference this frame does not own", f.Locals[in.Args[0]].Name)
			}
		}
	}
	return nil
}

// verifyReturnOwned rejects returning a lent reference as an owned result. Only
// planned bodies are held to it; before plan-rc a copy stands for a later share.
func (p *Program) verifyReturnOwned(f *Func) *Error {
	if f.rcBase == 0 || p.R.Types.Kind(f.Ret) == sem.KRef {
		return nil
	}
	origin := p.lentOrigins(f, p.unownedRefs(f))
	for bid, blk := range f.Blocks {
		if blk.Term.Op == TermReturn && origin[blk.Term.Args[0]] {
			return fail("droplent", BlockID(bid), len(blk.Insts), "returns %%%s, a reference the caller still owns", f.Locals[blk.Term.Args[0]].Name)
		}
	}
	return nil
}

func (p *Program) lentOrigins(f *Func, lent []bool) []bool {
	out := make([]bool, len(f.Locals))
	for l := range out {
		out[l] = f.Locals[l].Borrowed || lent[l]
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if (in.Op == OpAlias || in.Op == OpMove) && !out[in.Dst] && out[in.Args[0]] {
					out[in.Dst], changed = true, true
				}
			}
		}
	}
	return out
}

func (p *Program) directlyLent(f *Func, l LocalID) bool {
	return f.Locals[l].Borrowed || p.unownedRefs(f)[l]
}
