package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) verifyDropOwned(f *midir.Func) *Error {
	lent := p.unownedRefs(f)
	var origin []bool
	if f.RCBase != 0 {
		origin = p.lentOrigins(f, lent)
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			if in.Op == midir.OpNewCell && !in.Lent && f.RCBase != 0 && lent[in.Args[0]] {
				return fail("droplent", midir.BlockID(bid), i, "wraps %%%s, a reference this frame does not own, in a cell that releases it", f.Locals[in.Args[0]].Name)
			}
			if in.Op != midir.OpDrop && in.Op != midir.OpRelease {
				continue
			}
			if in.Op == midir.OpRelease && f.Async && slices.Contains(f.Params, in.Args[0]) {
				continue
			}
			if len(in.Args) == 1 && (lent[in.Args[0]] || f.Locals[in.Args[0]].Borrowed || (origin != nil && origin[in.Args[0]])) {
				return fail("droplent", midir.BlockID(bid), i, "releases %%%s, a reference this frame does not own", f.Locals[in.Args[0]].Name)
			}
		}
	}
	return nil
}

// verifyReturnOwned rejects returning a lent reference as an owned result. Only
// planned bodies are held to it; before plan-rc a copy stands for a later share.
func (p *verifier) verifyReturnOwned(f *midir.Func) *Error {
	if f.RCBase == 0 || p.R.Types.Kind(f.Ret) == sem.KRef {
		return nil
	}
	origin := p.lentOrigins(f, p.unownedRefs(f))
	for bid, blk := range f.Blocks {
		if blk.Term.Op == midir.TermReturn && origin[blk.Term.Args[0]] {
			return fail("droplent", midir.BlockID(bid), len(blk.Insts), "returns %%%s, a reference the caller still owns", f.Locals[blk.Term.Args[0]].Name)
		}
	}
	return nil
}

func (p *verifier) lentOrigins(f *midir.Func, lent []bool) []bool {
	out := make([]bool, len(f.Locals))
	for l := range out {
		out[l] = f.Locals[l].Borrowed || lent[l]
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if (in.Op == midir.OpAlias || in.Op == midir.OpMove) && !out[in.Dst] && out[in.Args[0]] {
					out[in.Dst], changed = true, true
				}
			}
		}
	}
	return out
}

func (p *verifier) directlyLent(f *midir.Func, l midir.LocalID) bool {
	return f.Locals[l].Borrowed || p.unownedRefs(f)[l]
}
