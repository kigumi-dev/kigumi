package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// verifyCounted holds a counted Option or Result to the two shapes the backends settle its counts in:
// unboxed in an async body only as a crossing between two adjacent instructions, and a crossing read by a single operand.
func (p *verifier) verifyCounted(f *midir.Func) *Error {
	counted := func(l midir.LocalID) bool { return f.IsDirect(l) && p.R.CountedAdt(f.Locals[l].Type) }
	for i := range f.Locals {
		if f.Async && counted(midir.LocalID(i)) && !f.Locals[i].Cross {
			return fail("direct", 0, 0, "%s keeps a counted value unboxed in an async frame", p.local(f, midir.LocalID(i)))
		}
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			seen := map[midir.LocalID]bool{}
			for _, a := range in.Args {
				if !f.Locals[a].Cross || !p.R.CountedAdt(f.Locals[a].Type) {
					continue
				}
				if seen[a] {
					return fail("direct", midir.BlockID(bid), i, "%s reads the counted crossing %s twice", in.Op, p.local(f, a))
				}
				seen[a] = true
			}
		}
	}
	return nil
}

// countedCrossSource names the named local a crossing of a counted value copies; a move of the crossing consumes that local.
func (p *verifier) countedCrossSource(f *midir.Func, in *midir.Inst, tracked []bool) (midir.LocalID, bool) {
	if in.Op != midir.OpBoxDirect && in.Op != midir.OpUnboxDirect || !f.Locals[in.Dst].Cross || !tracked[in.Args[0]] || !p.R.CountedAdt(in.Type) {
		return 0, false
	}
	return in.Args[0], true
}

func movedCross(in *midir.Inst, crossed map[midir.LocalID]midir.LocalID) (midir.LocalID, bool) {
	if in.Op != midir.OpMove {
		return 0, false
	}
	src, ok := crossed[in.Args[0]]
	return src, ok
}
