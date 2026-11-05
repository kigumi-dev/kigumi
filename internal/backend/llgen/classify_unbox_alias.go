package llgen

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// nameAliases hold no reference of their own: each aliases a named local or another such alias.
func nameAliases(f *midir.Func) map[midir.LocalID]bool {
	defs := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if definesDst(in.Op) {
				defs[in.Dst]++
			}
		}
	}
	out := map[midir.LocalID]bool{}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op != midir.OpAlias || len(in.Args) != 1 || out[in.Dst] || f.Locals[in.Dst].Ent != 0 || defs[in.Dst] != 1 {
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
func (e *emitter) scalarAliasBoxes(f *midir.Func) {
	uses := map[midir.LocalID]int{}
	borrowOf := map[midir.LocalID]midir.LocalID{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for _, a := range in.Args {
				uses[a]++
			}
			if in.Op == midir.OpBorrow {
				borrowOf[in.Args[0]] = in.Dst
			}
		}
		for _, a := range blk.Term.Args {
			uses[a]++
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpAlias && e.nameAlias[in.Dst] && !e.isScalar(in.Dst) && e.isScalar(in.Args[0]) && uses[in.Dst] == 1 {
				if h, ok := boxHolder(in.Dst, borrowOf, uses); ok {
					e.boxTmp[h] = true
				}
			}
		}
	}
}

// A borrow's reader still reads the box, so the release moves there; a borrow read twice leaks the box instead.
func boxHolder(l midir.LocalID, borrowOf map[midir.LocalID]midir.LocalID, uses map[midir.LocalID]int) (midir.LocalID, bool) {
	for {
		b, ok := borrowOf[l]
		if !ok {
			return l, true
		}
		if uses[b] != 1 {
			return 0, false
		}
		l = b
	}
}
