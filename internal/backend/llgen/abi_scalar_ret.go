package llgen

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

// A literal returns in a register only beside a computed value: literals alone are free as immortal boxes.
func (e *emitter) returnsInRegister(f *midir.Func, regs map[midir.LocalID]scalarInfo) bool {
	computed := false
	for _, blk := range f.Blocks {
		if blk.Term.Op != midir.TermReturn {
			continue
		}
		l := blk.Term.Args[0]
		if _, ok := regs[l]; !ok || !e.rawOrLiteral(f, l, map[midir.LocalID]bool{}) {
			return false
		}
		computed = computed || e.raw[l]
	}
	return computed
}

func (e *emitter) rawOrLiteral(f *midir.Func, l midir.LocalID, seen map[midir.LocalID]bool) bool {
	if seen[l] {
		return true
	}
	seen[l] = true
	if slices.Contains(f.Params, l) {
		return e.raw[l]
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || in.Dst != l || in.Op == midir.OpConst || e.rawDef(f, in, e.raw) {
				continue
			}
			if !copyLike(in.Op) || !e.rawOrLiteral(f, in.Args[0], seen) {
				return false
			}
		}
	}
	return true
}
