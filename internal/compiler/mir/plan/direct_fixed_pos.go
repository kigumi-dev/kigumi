package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

func (p *planner) fixedUse(f *midir.Func, in midir.Inst, out []bool) {
	switch {
	case mir.ProtoBuiltin(in):
		for _, a := range in.Args {
			out[a] = true
		}
	case mir.FixedFill(in):
		out[in.Dst] = true
	case in.Op == midir.OpIndex || in.Op == midir.OpSetIndex || mir.IterBuiltin(in):
		if p.R.IsFixedArray(f.Locals[in.Args[0]].Type) {
			out[in.Args[0]] = true
		}
	}
}
