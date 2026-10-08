package llgen

import "kigumi/internal/mir"

// nameAliases hold no reference of their own: each aliases a named local or another such alias.
func nameAliases(f *mir.Func) map[mir.LocalID]bool {
	defs := map[mir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if definesDst(in.Op) {
				defs[in.Dst]++
			}
		}
	}
	out := map[mir.LocalID]bool{}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op != mir.OpAlias || len(in.Args) != 1 || out[in.Dst] || f.Locals[in.Dst].Ent != 0 || defs[in.Dst] != 1 {
					continue
				}
				if src := in.Args[0]; f.Locals[src].Ent != 0 || out[src] {
					out[in.Dst], changed = true, true
				}
			}
		}
	}
	return out
}

// No plan releases the box a native value gets on its way into a name alias, so its one reader does.
func (e *emitter) scalarAliasBoxes(f *mir.Func) {
	uses := map[mir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				uses[a]++
			}
		}
		for _, a := range blk.Term.Args {
			uses[a]++
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpAlias && e.nameAlias[in.Dst] && !e.isScalar(in.Dst) && e.isScalar(in.Args[0]) && uses[in.Dst] == 1 {
				e.boxTmp[in.Dst] = true
			}
		}
	}
}
