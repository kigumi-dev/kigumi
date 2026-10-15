package llgen

import (
	"slices"

	"kigumi/internal/mir"
)

// A literal returns in a register only beside a computed value: literals alone are free as immortal boxes.
func (e *emitter) returnsInRegister(f *mir.Func, regs map[mir.LocalID]scalarInfo) bool {
	computed := false
	for _, blk := range f.Blocks {
		if blk.Term.Op != mir.TermReturn {
			continue
		}
		l := blk.Term.Args[0]
		if _, ok := regs[l]; !ok || !e.rawOrLiteral(f, l, map[mir.LocalID]bool{}) {
			return false
		}
		computed = computed || e.raw[l]
	}
	return computed
}

func (e *emitter) rawOrLiteral(f *mir.Func, l mir.LocalID, seen map[mir.LocalID]bool) bool {
	if seen[l] {
		return true
	}
	seen[l] = true
	if slices.Contains(f.Params, l) {
		return e.raw[l]
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || in.Dst != l || in.Op == mir.OpConst || e.rawDef(f, in, e.raw) {
				continue
			}
			if !copyLike(in.Op) || !e.rawOrLiteral(f, in.Args[0], seen) {
				return false
			}
		}
	}
	return true
}
