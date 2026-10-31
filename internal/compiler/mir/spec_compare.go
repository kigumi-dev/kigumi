package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// The NaN test folds to false once the witness is a known function that is not a float primitive.
func (s *specializer) foldUnordered(sp *spec, f *midir.Func, defs map[midir.LocalID]*midir.Inst, slots []sem.WitnessSlot, in *midir.Inst) {
	k := witnessSlotOf(f, defs, in.Args[0], slots)
	if k < 0 || sp.wit[k].Kind != sem.WitnessFn || FloatWitnessKey(StdKey(s.p.R, sp.wit[k].Ent, true)) {
		return
	}
	*in = midir.Inst{Op: midir.OpConst, Dst: in.Dst, Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: in.Node}
}

func FloatWitnessKey(key string) bool {
	switch key {
	case "prelude.f64.equals", "prelude.f64.compareTo", "prelude.f32.equals", "prelude.f32.compareTo":
		return true
	}
	return false
}

func foldCompareGuards(f *midir.Func, defs map[midir.LocalID]*midir.Inst) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			in := &insts[i]
			if in.Op != midir.OpBuiltin || in.Str != builtinCmpGuard {
				continue
			}
			if d := defs[in.Args[1]]; d != nil && d.Op == midir.OpConst && d.Type == sem.TyBool && !d.Lit.Bool {
				*in = midir.Inst{Op: midir.OpAlias, Dst: in.Dst, Args: []midir.LocalID{in.Args[0]}, Node: in.Node}
			}
		}
	}
}
