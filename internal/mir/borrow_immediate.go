package mir

import "kigumi/internal/sem"

// checkImmediateCaptures: a capBorrow closure's owners must stay untouched from its creation to the call it feeds.
func checkImmediateCaptures(p *Program, f *Func) {
	c := quietBorrowCheck(p, f)
	for bi := range f.Blocks {
		blk := &f.Blocks[bi]
		for i, in := range blk.Insts {
			// OpClosure also forms a bound-method value whose Ent names the method.
			if in.Op != OpClosure || p.R.Entity(in.Ent).Kind != sem.EntClosure {
				continue
			}
			info := p.R.Closure(in.Ent)
			if !info.Immediate {
				continue
			}
			var owners []LocalID
			for ci, cap := range info.Captures {
				if h, ok := c.holders[in.Args[ci]]; ok {
					if h.hasOwner {
						for _, o := range h.owners {
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
				c.checkImmediateSpan(BlockID(bi), i, in.Dst, owners)
			}
		}
	}
}

// quietBorrowCheck collects f's holders without reporting, for a check after the main pass already reported them.
func quietBorrowCheck(p *Program, f *Func) *borrowCheck {
	c := &borrowCheck{p: p, f: f, holders: map[LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[LocalID]bool{}}
	c.quiet = true
	c.collect()
	c.quiet = false
	return c
}

// checkImmediateSpan walks every block reachable before the closure is consumed: a branching sibling argument can move the call to a later block.
func (c *borrowCheck) checkImmediateSpan(bid BlockID, at int, closure LocalID, owners []LocalID) {
	visited := map[BlockID]bool{}
	var walk func(bid BlockID, from int)
	walk = func(bid BlockID, from int) {
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
