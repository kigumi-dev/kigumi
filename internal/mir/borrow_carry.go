package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// carrySite is one call that put a value carrying a borrow of owner into a
// container reached through the call's `mut self` receiver or a `&mut` argument.
type carrySite struct {
	block BlockID
	idx   int
	owner LocalID
	from  LocalID // when set, the site holds owner only if from holds it there
}

// carryFlows lists a call's `mut self` / `&mut` arguments and by-value `&mut` holders, and the by-value arguments
// carrying a borrow their types do not name (a Future or Task); a plain type parameter's borrow goes through callFlows.
func (c *borrowCheck) carryFlows(in Inst) (targets []storeFlow, values []LocalID) {
	if in.Op != OpCall && in.Op != OpCallValue || in.Ent == 0 || c.p.R.Entity(in.Ent).Kind != sem.EntFn {
		return nil, nil
	}
	info := c.p.R.Fn(in.Ent)
	recvIdx := 0
	if in.Op == OpCallValue {
		recvIdx = 1
	}
	for i, s := range c.callSlots(info, in.Args, recvIdx) {
		if s.target || s.holder {
			targets = append(targets, storeFlow{target: s.arg, ref: s.target, byValue: s.holder, callee: in.Ent, param: i})
			continue
		}
		if i == 0 && info.Recv != sem.RecvNone || c.p.R.Types.Kind(s.exact) == sem.KRef {
			continue
		}
		t := c.f.Locals[s.arg].Type
		if _, ok := c.holders[s.arg]; ok && !c.resultMayCarryLifetime(t) && (c.holdsDeferred(t) || !c.namesTypeParam(t)) {
			values = append(values, s.arg)
		}
	}
	if len(values) == 0 {
		return nil, nil
	}
	for i := range targets {
		for _, v := range values {
			targets[i].values = append(targets[i].values, flowArg{arg: v})
		}
	}
	return targets, values
}

// collectCarry makes each local container a carried value goes into a holder of
// what that value borrows; the owners are marked so only a reaching call counts.
func (c *borrowCheck) collectCarry(in Inst, bid BlockID, idx int) {
	targets, values := c.carryFlows(in)
	for _, fl := range targets {
		for _, root := range c.storeDests(fl) {
			c.targets[root] = true
			s := c.sink(root, in.Node)
			at := idx
			if wb, found := c.tieSite(fl.target, root); found {
				at = wb
			}
			for _, v := range values {
				b := c.holders[v]
				carried := *b
				carried.owners = nil
				for _, o := range b.owners {
					o.carried, o.node = true, in.Node
					carried.owners = append(carried.owners, o)
					c.addCarrySite(root, carrySite{block: bid, idx: at, owner: o.owner})
				}
				c.merge(s, &carried)
			}
		}
	}
}

func (c *borrowCheck) addCarrySite(root LocalID, site carrySite) {
	if c.carry == nil {
		c.carry = map[LocalID][]carrySite{}
	}
	for _, s := range c.carry[root] {
		if s == site {
			return
		}
	}
	c.carry[root] = append(c.carry[root], site)
}

// checkCarry rejects putting a carried borrow into a container the caller owns.
func (c *borrowCheck) checkCarry(in Inst) {
	targets, values := c.carryFlows(in)
	for _, fl := range targets {
		if _, _, caller, ok := c.storeRoot(fl); ok && caller {
			c.report(in.Node, values[0], "borrow-escape", "a borrow cannot be stored")
		}
	}
}

// carryReaches reports whether some call carrying owner into holder reaches instruction at
// of the current block without holder being reassigned, moved out or dropped on the way.
func (c *borrowCheck) carryReaches(holder, owner LocalID, at int) bool {
	sites := c.carry[holder]
	if len(sites) == 0 {
		return true
	}
	key := [2]LocalID{holder, owner}
	in, ok := c.carryIn[key]
	if !ok {
		in = c.carryDataflow(holder, owner, sites, false)
		if c.carryIn == nil {
			c.carryIn = map[[2]LocalID][]bool{}
		}
		c.carryIn[key] = in
	}
	return c.carryStep(c.curBlock, holder, owner, sites, in[c.curBlock], at, false)
}

// stores also ends the borrow at a store that replaces holder's only borrow-carrying field.
func (c *borrowCheck) carryDataflow(holder, owner LocalID, sites []carrySite, stores bool) []bool {
	in := make([]bool, len(c.f.Blocks))
	for changed := true; changed; {
		changed = false
		for bid := range c.f.Blocks {
			out := c.carryStep(BlockID(bid), holder, owner, sites, in[bid], len(c.f.Blocks[bid].Insts), stores)
			if !out {
				continue
			}
			for _, t := range c.f.Blocks[bid].Term.Targets {
				if !in[t] {
					in[t], changed = true, true
				}
			}
		}
	}
	return in
}

func (c *borrowCheck) carryStep(bid BlockID, holder, owner LocalID, sites []carrySite, cur bool, end int, stores bool) bool {
	insts := c.f.Blocks[bid].Insts
	for i := 0; i < end && i < len(insts); i++ {
		in := insts[i]
		ends := shapes[in.Op].result && in.Dst == holder || (in.Op == OpMove || in.Op == OpDrop) && len(in.Args) > 0 && in.Args[0] == holder
		if ends && !c.writeBackKeeps(bid, i, holder) || stores && c.resetsAt(bid, i, holder) || c.partReplaced(bid, i, holder, owner, sites) {
			cur = false
		}
		for _, s := range sites {
			if s.block == bid && s.idx == i && s.owner == owner {
				cur = cur || s.from == 0 || c.tieHeld(s.from, owner, bid, i, len(c.f.Locals))
			}
		}
	}
	return cur
}

// carriedResultSource passes a by-value argument's untagged borrow (a Future, a Task, an array
// of them) on to a result that may keep the argument, as `fn id(f: Future[T]) -> Future[T]`.
func (c *borrowCheck) carriedResultSource(in Inst, recvIdx int) (*borrowInfo, bool) {
	ret := c.f.Locals[in.Dst].Type
	if c.resultMayCarryLifetime(ret) || c.p.R.IsCopy(ret) {
		return nil, false
	}
	var found []*borrowInfo
	for _, s := range c.callSlots(c.p.R.Fn(in.Ent), in.Args, recvIdx) {
		for _, b := range c.slotCarry(s, ret) {
			if !slices.Contains(found, b) {
				found = append(found, b)
			}
		}
	}
	switch len(found) {
	case 0:
		return nil, false
	case 1:
		return found[0], true
	}
	sink := c.sink(in.Dst, in.Node)
	for _, b := range found {
		c.merge(sink, b)
	}
	return sink, true
}

// carriedArg is what an argument carries into a result that may keep it; a closure built here
// carries what it borrows from its captures.
func (c *borrowCheck) carriedArg(a LocalID) (*borrowInfo, bool) {
	if b, ok := c.holders[a]; ok {
		return b, true
	}
	if def := c.definition(a); def != nil && def.Op == OpClosure && c.p.R.Entity(def.Ent).Kind == sem.EntClosure {
		if b := c.deferredCaptures(def); b != nil {
			return b, true
		}
	}
	return nil, false
}
