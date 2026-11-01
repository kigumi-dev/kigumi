package plan

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A crossing is produced and consumed between two neighbouring
// instructions, so it never needs a frame slot of its own.
func crossTemp(f *midir.Func, t sem.TypeID, direct bool) midir.LocalID {
	f.Locals = append(f.Locals, midir.Local{Name: fmt.Sprintf("x%d", len(f.Locals)), Type: t, Cross: true})
	f.Direct = append(f.Direct, direct)
	if len(f.Slot) > 0 {
		f.Slot = append(f.Slot, -1)
	}
	return midir.LocalID(len(f.Locals) - 1)
}

// replaceArg copies on first write: the builder shares argument slices.
func replaceArg(args []midir.LocalID, i int, l midir.LocalID) []midir.LocalID {
	cp := make([]midir.LocalID, len(args))
	copy(cp, args)
	cp[i] = l
	return cp
}

// countlessRefs marks the reference temporaries holding no count of their own,
// so a crossing out of one gives nothing back when its consumer keeps the copy.
func (p *planner) countlessRefs(f *midir.Func) []bool {
	c := refCounts{defs: map[midir.LocalID][]midir.Inst{}, owns: map[midir.LocalID]bool{}}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && f.Locals[in.Dst].Ent == 0 {
				c.defs[in.Dst] = append(c.defs[in.Dst], in)
			}
		}
	}
	out := make([]bool, len(f.Locals))
	for l, loc := range f.Locals {
		out[l] = loc.Ent == 0 && p.R.Types.Kind(loc.Type) == sem.KRef && !c.own(midir.LocalID(l))
	}
	return out
}

type refCounts struct {
	defs map[midir.LocalID][]midir.Inst
	owns map[midir.LocalID]bool
}

// own follows aliases back to where the value was made: a call, a field read or a share takes a count,
// while a borrow, a lent element and an alias of a named local take none.
func (c *refCounts) own(l midir.LocalID) bool {
	if v, ok := c.owns[l]; ok {
		return v
	}
	c.owns[l] = false
	v := len(c.defs[l]) > 0
	for _, in := range c.defs[l] {
		switch {
		case in.Op == midir.OpBorrow, in.Op == midir.OpLendBox, in.Op == midir.OpBuiltin && in.Str == "iter.at_ref":
			v = false
		case (in.Op == midir.OpAlias || in.Op == midir.OpMove || in.Op == midir.OpCopy && !in.Share) && len(in.Args) == 1:
			v = v && c.own(in.Args[0])
		}
	}
	c.owns[l] = v
	return v
}
