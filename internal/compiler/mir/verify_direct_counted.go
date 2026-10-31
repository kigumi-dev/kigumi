package mir

// verifyCounted holds a counted Option or Result to the two shapes the backends settle its counts in:
// unboxed in an async body only as a crossing between two adjacent instructions, and a crossing read by a single operand.
func (p *verifier) verifyCounted(f *Func) *Error {
	counted := func(l LocalID) bool { return f.IsDirect(l) && p.R.CountedAdt(f.Locals[l].Type) }
	for i := range f.Locals {
		if f.Async && counted(LocalID(i)) && !f.Locals[i].Cross {
			return fail("direct", 0, 0, "%s keeps a counted value unboxed in an async frame", p.local(f, LocalID(i)))
		}
	}
	for bid, blk := range f.Blocks {
		for i, in := range blk.Insts {
			seen := map[LocalID]bool{}
			for _, a := range in.Args {
				if !f.Locals[a].Cross || !p.R.CountedAdt(f.Locals[a].Type) {
					continue
				}
				if seen[a] {
					return fail("direct", BlockID(bid), i, "%s reads the counted crossing %s twice", in.Op, p.local(f, a))
				}
				seen[a] = true
			}
		}
	}
	return nil
}

// countedCrossSource names the named local a crossing of a counted value copies; a move of the crossing consumes that local.
func (p *verifier) countedCrossSource(f *Func, in *Inst, tracked []bool) (LocalID, bool) {
	if in.Op != OpBoxDirect && in.Op != OpUnboxDirect || !f.Locals[in.Dst].Cross || !tracked[in.Args[0]] || !p.R.CountedAdt(in.Type) {
		return 0, false
	}
	return in.Args[0], true
}

func movedCross(in *Inst, crossed map[LocalID]LocalID) (LocalID, bool) {
	if in.Op != OpMove {
		return 0, false
	}
	src, ok := crossed[in.Args[0]]
	return src, ok
}
