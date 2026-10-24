package mir

import "kigumi/internal/sem"

// collect finds the locals holding borrows (borrows, their copies and reads, reference and
// named-lifetime params, containers of such values), widening containers until nothing grows.
func (c *borrowCheck) collect() {
	c.sinks = map[LocalID]*borrowInfo{}
	c.contents = map[LocalID]*borrowInfo{}
	c.caps = map[LocalID]*borrowInfo{}
	c.targets = c.storeTargets()
	c.collectParams()
	for changed := true; changed; {
		changed = false
		c.grew = false
		for bid, blk := range c.f.Blocks {
			for i, in := range blk.Insts {
				c.resets.at = carrySite{block: BlockID(bid), idx: i}
				if c.collectInst(in) {
					changed = true
				}
				c.collectCarry(in, BlockID(bid), i)
			}
		}
		changed = changed || c.grew
	}
}

func (c *borrowCheck) collectParams() {
	fn := c.p.R.Entity(c.f.Ent).Kind == sem.EntFn
	for _, prm := range c.f.Params {
		typ := c.f.Locals[prm].Type
		lt, node := c.paramLifetime(prm)
		if lt == 0 && fn && c.p.R.Types.Kind(typ) != sem.KRef {
			lt = c.soleLifetime(typ)
		}
		if c.p.R.Types.Kind(typ) != sem.KRef && lt == 0 {
			continue
		}
		b := &borrowInfo{lifetime: lt, node: node}
		if lt == 0 && c.multiLifetime(typ) {
			b.multi = typ
		}
		if c.targets[prm] && !c.callerPlace(prm) {
			c.merge(c.sink(prm, node), b)
			continue
		}
		c.holders[prm] = b
	}
}

func (c *borrowCheck) merge(dst, src *borrowInfo) {
	if mergeInto(dst, src) {
		c.grew = true
	}
}

func (c *borrowCheck) collectInst(in Inst) bool {
	_, has := c.holders[in.Dst]
	set := func(b *borrowInfo, ok bool) bool {
		if !ok || has {
			return false
		}
		c.holders[in.Dst] = b
		return true
	}
	switch in.Op {
	case OpBorrow:
		root, path, exact, ok := c.patternPlace(in.Args[0])
		if has {
			if ok {
				c.merge(c.holders[in.Dst], c.throughRef(root, path, exact, in.Index == 1))
			}
			return false
		}
		if ok {
			ref := ownerRef{owner: root, mut: in.Index == 1, path: path, exact: exact, lifetime: c.projectionLifetime(root, in.Args[0])}
			b := &borrowInfo{owners: []ownerRef{ref}, hasOwner: true, node: in.Node}
			mergeInto(b, c.throughRef(root, path, exact, in.Index == 1))
			return set(b, true)
		}
		if b, ok := c.placelessBorrow(in); ok {
			return set(b, true)
		}
		if def := c.definition(in.Args[0]); def != nil && def.Op == OpGlobalGet {
			// A static-origin borrow has no local owner to touch-check
			// against, but still has to reject escape like any other.
			return set(&borrowInfo{node: in.Node, fromStatic: true}, true)
		}
	case OpCopy, OpMove:
		src, ok := c.holders[in.Args[0]]
		if ok && c.widens(in.Dst) {
			c.widen(in.Dst, src)
			return false
		}
		return set(src, ok)
	case OpLendBox:
		return set(c.lendBoxInfo(in), true)
	case OpArray:
		c.collectArray(in)
	case OpSetIndex, OpSetField:
		c.collectFlows(in)
	case OpRecord, OpVariant, OpCall, OpCallValue, OpFutureNew:
		c.collectFlows(in)
		if _, ok := c.sinks[in.Dst]; ok || c.widens(in.Dst) {
			if b, ok := c.lifetimeSource(in); ok {
				c.widen(in.Dst, b)
			}
			return false
		}
		if !has {
			return set(c.lifetimeSource(in))
		}
	case OpBuiltin:
		c.collectBuiltin(in)
	case OpField, OpIndex, OpPayload, OpFieldMove:
		if !has && c.projectsHeld(in) {
			return set(c.readInfo(in.Args[0]))
		}
	}
	return false
}

// widen merges b into dst's sink; a sink created here blames b's borrow, as sharing b did.
func (c *borrowCheck) widen(dst LocalID, b *borrowInfo) {
	c.merge(c.sink(dst, b.node), b)
	c.noteResetSite(dst, b)
}

// widens reports whether dst keeps the union of every value written to it instead of the first
// one: a store target, a temporary joining branches, or a named local of a type carrying a lifetime.
func (c *borrowCheck) widens(dst LocalID) bool {
	if c.targets[dst] || c.multiDefined(dst) {
		return true
	}
	l := c.f.Locals[dst]
	return l.Ent != 0 && c.elementHolder(l.Type)
}

// collectArray makes an array literal a container of its elements' sources
// when each borrow it holds is a value with a named lifetime, not a bare `&T`.
func (c *borrowCheck) collectArray(in Inst) {
	var srcs []*borrowInfo
	for _, a := range in.Args {
		b, ok := c.holders[a]
		if !ok {
			continue
		}
		if t := c.f.Locals[a].Type; !c.elementHolder(t) && !c.namesTypeParam(t) {
			return
		}
		srcs = append(srcs, b)
	}
	if len(srcs) == 0 {
		return
	}
	s := c.sink(in.Dst, in.Node)
	for _, b := range srcs {
		c.merge(s, b)
	}
}

// collectBuiltin ties a builtin's result (a loop's element) to what every
// argument holds, and to the referent's elements for a reference argument.
func (c *borrowCheck) collectBuiltin(in Inst) {
	if in.Dst == 0 {
		return
	}
	if ret := c.f.Locals[in.Dst].Type; !c.resultMayCarryLifetime(ret) {
		c.collectBuiltinCarry(in, ret)
		return
	}
	for _, a := range in.Args {
		if b, ok := c.holders[a]; ok {
			c.merge(c.sink(in.Dst, in.Node), b)
		}
		if c.p.R.Types.Kind(c.f.Locals[a].Type) != sem.KRef {
			continue
		}
		if b, ok := c.argContents(a); ok {
			c.merge(c.sink(in.Dst, in.Node), b)
		}
	}
}

// collectBuiltinCarry passes an owned argument's untagged borrow on to a result that may take one
// of its Futures, Tasks or closures, as a `for` element moved out of an owned container.
func (c *borrowCheck) collectBuiltinCarry(in Inst, ret sem.TypeID) {
	for _, a := range in.Args {
		t := c.f.Locals[a].Type
		if c.p.R.Types.Kind(t) == sem.KRef || c.resultMayCarryLifetime(t) || !c.mayCarry(ret, t) {
			continue
		}
		if b, ok := c.holders[a]; ok {
			c.merge(c.sink(in.Dst, in.Node), b)
		}
	}
}
