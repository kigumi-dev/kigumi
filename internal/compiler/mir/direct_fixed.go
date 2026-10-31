package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *Program) markFixed(f *midir.Func) {
	fixed := p.fixedLocals(f)
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			if insts[i].Op == midir.OpBoxDirect && fixed[insts[i].Args[0]] {
				insts[i].Fixed = true
			}
		}
	}
}

// FixedLocals marks the locals holding a compile-time value, which both backends hand over as one immortal.
func (p *Program) FixedLocals(f *midir.Func) []bool { return p.fixedLocals(f) }

func (p *Program) fixedLocals(f *midir.Func) []bool {
	defs := make([]int, len(f.Locals))
	for _, prm := range f.Params {
		defs[prm]++
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst]++
			}
		}
	}
	out := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if !in.Op.Yields() || defs[in.Dst] != 1 || f.Locals[in.Dst].Cell {
				continue
			}
			switch in.Op {
			case midir.OpUnit:
				out[in.Dst] = true
			case midir.OpConst:
				out[in.Dst] = p.fixedLit(in)
			case midir.OpAlias, midir.OpMove, midir.OpCopy, midir.OpShare, midir.OpBoxDirect:
				out[in.Dst] = out[in.Args[0]]
			case midir.OpVariant:
				out[in.Dst] = allFixed(in.Args, out)
			}
			out[in.Dst] = out[in.Dst] || f.Locals[in.Dst].Type == sem.TyUnit
		}
	}
	return out
}

func allFixed(args []midir.LocalID, fixed []bool) bool {
	for _, a := range args {
		if !fixed[a] {
			return false
		}
	}
	return true
}

func (p *Program) fixedLit(in midir.Inst) bool {
	switch in.Lit.Kind {
	case sem.LitInt, sem.LitChar, sem.LitString:
		return true
	}
	t := p.R.Types
	return in.Str != "" && t.IsInteger(in.Type) || t.IsFloat(in.Type)
}
