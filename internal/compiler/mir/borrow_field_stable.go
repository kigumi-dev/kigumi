package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

type useSite struct {
	in  *Inst // nil for a terminator
	arg int
}

type fieldState struct {
	stable map[LocalID]bool
	uses   map[LocalID][]useSite
}

// Lending l by `&mut` keeps it stable only when the callee stores past the references l holds.
func (c *borrowCheck) fieldStable(l LocalID) bool {
	if v, ok := c.fields.stable[l]; ok {
		return v
	}
	loc := c.f.Locals[l]
	v := loc.Ent != 0 && !loc.Cell && !loc.Borrowed && !slices.Contains(c.f.Params, l) && c.literalOf(l) != nil && !c.writesPlace(l, map[LocalID]bool{})
	if c.fields.stable == nil {
		c.fields.stable = map[LocalID]bool{}
	}
	c.fields.stable[l] = v
	return v
}

// t names a place in a holder: the holder, a `&mut` to it, or a non-reference read below either.
func (c *borrowCheck) writesPlace(t LocalID, seen map[LocalID]bool) bool {
	if seen[t] {
		return false
	}
	seen[t] = true
	isRef := func(l LocalID) bool { return c.p.R.Types.Kind(c.f.Locals[l].Type) == sem.KRef }
	ref := isRef(t)
	for _, u := range c.usesOf(t) {
		in := u.in
		switch {
		case in == nil:
			if ref {
				return true
			}
		case in.Op == OpDrop || in.Op == OpRelease || in.Op == OpIsVariant || in.Op == OpTag || in.Op == OpIsType || in.Op == OpBorrow && in.Index == 0:
		case (in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet) && u.arg == 0:
			return true
		case in.Op == OpBorrow:
			if c.writesPlace(in.Dst, seen) {
				return true
			}
		case (in.Op == OpField || in.Op == OpPayload || in.Op == OpIndex || in.Op == OpUnbox) && u.arg == 0:
			if !isRef(in.Dst) && c.writesPlace(in.Dst, seen) {
				return true
			}
		case in.Op == OpCopy || in.Op == OpMove || in.Op == OpAlias || in.Op == OpShare:
			if ref && c.writesPlace(in.Dst, seen) {
				return true
			}
		default:
			if ref && c.lendWritesPlace(in, u.arg) {
				return true
			}
		}
	}
	return false
}

// A store past a reference the place holds leaves the place's own fields as they were.
func (c *borrowCheck) lendWritesPlace(in *Inst, k int) bool {
	h := c.p.ByEnt[in.Ent]
	if h == nil || in.Op != OpClosure && (in.Op != OpCall || c.p.R.Entity(in.Ent).Kind != sem.EntFn) {
		return true
	}
	return slices.ContainsFunc(c.paramStoreSites(h, k), func(s paramStore) bool { return !s.beyond })
}

func (c *borrowCheck) usesOf(l LocalID) []useSite {
	if c.fields.uses == nil {
		c.fields.uses = map[LocalID][]useSite{}
		for b := range c.f.Blocks {
			blk := &c.f.Blocks[b]
			for i := range blk.Insts {
				for k, a := range blk.Insts[i].Args {
					c.fields.uses[a] = append(c.fields.uses[a], useSite{in: &blk.Insts[i], arg: k})
				}
			}
			for _, a := range blk.Term.Args {
				c.fields.uses[a] = append(c.fields.uses[a], useSite{})
			}
		}
	}
	return c.fields.uses[l]
}
