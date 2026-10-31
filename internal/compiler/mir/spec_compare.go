package mir

import "kigumi/internal/compiler/sem"

// The NaN test folds to false once the witness is a known function that is not a float primitive.
func (s *specializer) foldUnordered(sp *spec, f *Func, defs map[LocalID]*Inst, slots []sem.WitnessSlot, in *Inst) {
	k := witnessSlotOf(f, defs, in.Args[0], slots)
	if k < 0 || sp.wit[k].Kind != sem.WitnessFn || FloatWitnessKey(StdKey(s.p.R, sp.wit[k].Ent, true)) {
		return
	}
	*in = Inst{Op: OpConst, Dst: in.Dst, Lit: sem.Literal{Kind: sem.LitBool}, Type: sem.TyBool, Node: in.Node}
}

func FloatWitnessKey(key string) bool {
	switch key {
	case "prelude.f64.equals", "prelude.f64.compareTo", "prelude.f32.equals", "prelude.f32.compareTo":
		return true
	}
	return false
}

func foldCompareGuards(f *Func, defs map[LocalID]*Inst) {
	for bid := range f.Blocks {
		insts := f.Blocks[bid].Insts
		for i := range insts {
			in := &insts[i]
			if in.Op != OpBuiltin || in.Str != builtinCmpGuard {
				continue
			}
			if d := defs[in.Args[1]]; d != nil && d.Op == OpConst && d.Type == sem.TyBool && !d.Lit.Bool {
				*in = Inst{Op: OpAlias, Dst: in.Dst, Args: []LocalID{in.Args[0]}, Node: in.Node}
			}
		}
	}
}
