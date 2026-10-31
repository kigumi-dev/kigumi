package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// projectionPlace follows a temporary read out of a value back to its named local, with the field chain
// (root-first); an index or unbox step cuts the chain there, marking it inexact.
func (c *borrowCheck) projectionPlace(l midir.LocalID) (root midir.LocalID, path stepPath, exact bool, ok bool) {
	return c.walkPlace(l, false)
}

// patternPlace is projectionPlace also crossing a temporary borrow and a borrowed loop's element,
// so a binding destructured from `&e` or `&mut e`, or bound by `for x in &e`, holds e itself.
func (c *borrowCheck) patternPlace(l midir.LocalID) (root midir.LocalID, path stepPath, exact bool, ok bool) {
	return c.walkPlace(l, true)
}

// throughRef is what a borrow of root's referent at path holds when root is a reference
// local: the places root borrows, extended by path, so they stay borrowed while it lives.
func (c *borrowCheck) throughRef(root midir.LocalID, path stepPath, exact, mut bool) *borrowInfo {
	b, ok := c.holders[root]
	if !ok || !b.hasOwner || c.p.R.Types.Kind(c.f.Locals[root].Type) != sem.KRef {
		return nil
	}
	out := &borrowInfo{hasOwner: true, node: b.node}
	cut := c.fromCall(root)
	for _, o := range b.owners {
		if o.view || o.carried {
			continue
		}
		p := slices.Clone(o.path)
		if o.exact && !cut {
			p = append(p, path...)
		}
		out.owners = append(out.owners, ownerRef{owner: o.owner, mut: mut, path: p, exact: o.exact && exact && !cut, lifetime: o.lifetime})
	}
	if len(out.owners) == 0 {
		return nil
	}
	return out
}

func (c *borrowCheck) walkPlace(l midir.LocalID, throughBorrow bool) (root midir.LocalID, path stepPath, exact bool, ok bool) {
	exact = true
	for {
		if c.f.Locals[l].Ent != 0 {
			return l, path, exact, true
		}
		in := c.definition(l)
		if in == nil || len(in.Args) == 0 {
			return 0, nil, false, false
		}
		switch in.Op {
		case midir.OpField, midir.OpPayload, midir.OpIndex, midir.OpUnbox:
			path, exact = path.under(in, exact, nil)
			l = in.Args[0]
		case midir.OpCopy:
			l = in.Args[0]
		case midir.OpBorrow:
			if !throughBorrow {
				return 0, nil, false, false
			}
			l = in.Args[0]
		case midir.OpBuiltin:
			if !throughBorrow || !iterElement(in) {
				return 0, nil, false, false
			}
			path, exact = nil, false
			l = in.Args[0]
		case midir.OpCellGet:
			if !c.f.Locals[in.Args[0]].Borrowed && !c.placeCell(in.Args[0]) {
				return 0, nil, false, false
			}
			l = in.Args[0]
		default:
			return 0, nil, false, false
		}
	}
}

// chainedOnly reports whether every use of l is as the base of a further
// field/index/payload/unbox step, or as what a borrow takes directly (a
// mid-chain hop like `self.inner` on the way to `self.inner.y`, or to
// `&self.inner`). Such a hop is not itself a touch: the deeper step it
// feeds resolves the full path back to the root via projectionPlace (touch's
// own OpBorrow case for a direct borrow), and a prefix of an overlapping
// path always overlaps too, so checking only the deepest step is
// equivalent and avoids double-reporting one borrow.
func (c *borrowCheck) chainedOnly(l midir.LocalID) bool {
	if c.chainOnly == nil {
		c.chainOnly = map[midir.LocalID]bool{}
		total, chained := map[midir.LocalID]int{}, map[midir.LocalID]int{}
		count := func(a midir.LocalID) { total[a]++ }
		for b := range c.f.Blocks {
			for _, in := range c.f.Blocks[b].Insts {
				for _, a := range in.Args {
					count(a)
				}
				switch in.Op {
				case midir.OpField, midir.OpSetField, midir.OpIndex, midir.OpPayload, midir.OpUnbox, midir.OpBorrow:
					if len(in.Args) > 0 {
						chained[in.Args[0]]++
					}
				}
			}
			for _, a := range c.f.Blocks[b].Term.Args {
				count(a)
			}
		}
		for id, n := range chained {
			if n > 0 && total[id] == n {
				c.chainOnly[id] = true
			}
		}
	}
	return c.chainOnly[l]
}

// definition finds the single instruction defining a temporary; nil when
// it has none or several.
func (c *borrowCheck) definition(l midir.LocalID) *midir.Inst {
	if c.defs == nil {
		c.defs = map[midir.LocalID]*midir.Inst{}
		for b := range c.f.Blocks {
			for i := range c.f.Blocks[b].Insts {
				in := &c.f.Blocks[b].Insts[i]
				if !in.Op.Yields() || c.f.Locals[in.Dst].Ent != 0 {
					continue
				}
				if _, dup := c.defs[in.Dst]; dup {
					c.defs[in.Dst] = nil
				} else {
					c.defs[in.Dst] = in
				}
			}
		}
	}
	return c.defs[l]
}

func (c *borrowCheck) takesPayload(in midir.Inst) bool {
	if in.Op != midir.OpCopy || c.p.R.IsCopy(c.f.Locals[in.Dst].Type) {
		return false
	}
	if src := c.f.Locals[in.Args[0]]; src.Ent != 0 {
		return !in.Share && !src.Borrowed && !src.Cell && c.f.Locals[in.Dst].Ent != 0
	}
	d := c.definition(in.Args[0])
	if d == nil {
		return false
	}
	switch d.Op {
	case midir.OpPayload:
		return d.Place || c.f.Locals[in.Dst].Ent != 0
	case midir.OpField:
		return c.f.Locals[in.Dst].Ent != 0
	}
	return false
}

// takeNode: the copy out of a `||` hit carries no node of its own, so it reports at the payload read.
func (c *borrowCheck) takeNode(in midir.Inst) syntax.NodeID {
	if in.Node == 0 && c.takesPayload(in) && c.f.Locals[in.Args[0]].Ent == 0 {
		return c.definition(in.Args[0]).Node
	}
	return in.Node
}
