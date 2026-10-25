package mir

import "kigumi/internal/sem"

// heldBorrow: a borrow this body stored into a caller place outlives the call, so a later write here would dangle it.
type heldBorrow struct {
	holder LocalID
	site   carrySite
	info   *borrowInfo
}

type heldState struct {
	borrows []heldBorrow
	in      map[heldBorrow][]bool
}

func (c *borrowCheck) checkHeld(in Inst) {
	if in.Op == OpDrop {
		return
	}
	for _, h := range c.held.borrows {
		o := h.info.owners[0]
		if touched, path, _ := c.touch(in, o.owner); touched && fieldPathsOverlap(path, o.path) && c.heldReaches(h) {
			c.reportTouch(in, h.holder, h.info, o)
		}
	}
}

func (c *borrowCheck) heldBorrows() []heldBorrow {
	var out []heldBorrow
	for bid, blk := range c.f.Blocks {
		for i, in := range blk.Insts {
			c.callNode = in.Node
			add := func(root LocalID, place sem.TypeID, b *borrowInfo) {
				for _, o := range b.owners {
					var at []LocalID
					if c.callerStorageFrom(o.owner, false, 0, &at) && len(at) == 1 && c.writableCaller(at[0]) {
						o.node, o.tied, o.caller = in.Node, false, true
						o.mut = o.mut && !c.plainData(place)
						out = append(out, heldBorrow{holder: root, site: carrySite{block: BlockID(bid), idx: i, owner: o.owner}, info: &borrowInfo{owners: []ownerRef{o}, hasOwner: true, node: in.Node}})
					}
				}
			}
			if in.Op == OpCellSet {
				if root, place, caller, ok := c.storeRoot(storeFlow{target: in.Args[0], ref: true}); ok && caller {
					if b, ok := c.holders[in.Args[len(in.Args)-1]]; ok {
						add(root, place, b)
					}
				}
				continue
			}
			for _, fl := range c.storeFlows(in) {
				root, place, caller, ok := c.storeRoot(fl)
				if !ok || !caller {
					continue
				}
				for _, v := range fl.values {
					if b, has := c.heldInfo(fl, v); has {
						add(root, place, b)
					}
				}
			}
		}
	}
	return out
}

func (c *borrowCheck) heldInfo(fl storeFlow, v flowArg) (*borrowInfo, bool) {
	b, ok := c.tieInfo(v, fl.generic, false)
	if !ok || b.hasOwner || v.contents || c.p.R.Types.Kind(c.f.Locals[v.arg].Type) != sem.KRef {
		return b, ok && b.hasOwner
	}
	return c.referentBorrow(v.arg, b, sem.RecvNone)
}

// heldReaches stops at a whole replacement of the holder.
func (c *borrowCheck) heldReaches(h heldBorrow) bool {
	key := heldBorrow{holder: h.holder, site: h.site}
	in, ok := c.held.in[key]
	if !ok {
		in = make([]bool, len(c.f.Blocks))
		for changed := true; changed; {
			changed = false
			for bid := range c.f.Blocks {
				if !c.heldStep(BlockID(bid), h, in[bid], len(c.f.Blocks[bid].Insts)) {
					continue
				}
				for _, t := range c.f.Blocks[bid].Term.Targets {
					if !in[t] {
						in[t], changed = true, true
					}
				}
			}
		}
		c.held.in[key] = in
	}
	return c.heldStep(c.curBlock, h, in[c.curBlock], c.curIdx)
}

func (c *borrowCheck) heldStep(bid BlockID, h heldBorrow, cur bool, end int) bool {
	insts := c.f.Blocks[bid].Insts
	for i := 0; i < end && i < len(insts); i++ {
		if insts[i].Op == OpCellSet && insts[i].Args[0] == h.holder {
			cur = false
		}
		if bid == h.site.block && i == h.site.idx {
			cur = true
		}
	}
	return cur
}
