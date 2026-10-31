package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// checkImmediateCaptures: a capBorrow closure's owners must stay untouched from its creation to the call it feeds.
func checkImmediateCaptures(state *Analyzer, f *midir.Func) map[reportKey]int {
	p := state.Input
	c := quietBorrowCheck(state, f)
	memo := flowMemo{}
	for bi := range f.Blocks {
		blk := &f.Blocks[bi]
		for i, in := range blk.Insts {
			// OpClosure also forms a bound-method value whose Ent names the method.
			if in.Op != midir.OpClosure || p.R.Entity(in.Ent).Kind != sem.EntClosure {
				continue
			}
			info := p.R.Closure(in.Ent)
			if !info.Immediate {
				continue
			}
			var owners []midir.LocalID
			for ci, cap := range info.Captures {
				if h, ok := c.holders[in.Args[ci]]; ok {
					if h.hasOwner {
						for _, o := range c.flowOwners(in.Args[ci], memo) {
							owners = append(owners, o.owner)
						}
					}
					continue
				}
				if cap.Mode != sem.CapBorrow {
					continue
				}
				root, _, _, ok := c.projectionPlace(in.Args[ci])
				// A Borrowed local is already pinned by checkBorrowedMoves.
				if ok && !c.f.Locals[root].Borrowed {
					owners = append(owners, root)
				}
			}
			if len(owners) > 0 {
				c.checkImmediateSpan(midir.BlockID(bi), i, in.Dst, owners)
			}
		}
	}
	return c.reported
}

// quietBorrowCheck collects f's holders without reporting, for a check after the main pass already reported them.
func quietBorrowCheck(state *Analyzer, f *midir.Func) *borrowCheck {
	p := state.Input
	c := &borrowCheck{p: p, f: f, state: state, holders: map[midir.LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[midir.LocalID]bool{}}
	c.quiet = true
	c.collect()
	c.quiet = false
	return c
}

// checkImmediateSpan walks every block reachable before the closure is consumed: a branching sibling argument can move the call to a later block.
func (c *borrowCheck) checkImmediateSpan(bid midir.BlockID, at int, closure midir.LocalID, owners []midir.LocalID) {
	visited := map[midir.BlockID]bool{}
	var walk func(bid midir.BlockID, from int)
	walk = func(bid midir.BlockID, from int) {
		if visited[bid] {
			return
		}
		visited[bid] = true
		blk := &c.f.Blocks[bid]
		for j := from; j < len(blk.Insts); j++ {
			in := blk.Insts[j]
			for _, owner := range owners {
				if touched, _, _ := c.touch(in, owner); touched {
					name := c.f.Locals[owner].Name
					c.report(in.Node, owner, "borrow-conflict", "`"+name+"` is captured by a closure passed to this call and cannot be used until the call returns")
				}
			}
			for _, a := range in.Args {
				if a == closure {
					return
				}
			}
		}
		for _, a := range blk.Term.Args {
			if a == closure {
				return
			}
		}
		for _, t := range blk.Term.Targets {
			walk(t, 0)
		}
	}
	walk(bid, at+1)
}
