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
		for _, blk := range c.f.Blocks {
			for _, in := range blk.Insts {
				if c.collectInst(in) {
					changed = true
				}
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
		if has {
			return false
		}
		if root, path, exact, ok := c.projectionPlace(in.Args[0]); ok {
			ref := ownerRef{owner: root, mut: in.Index == 1, path: path, exact: exact}
			return set(&borrowInfo{owners: []ownerRef{ref}, hasOwner: true, node: in.Node}, true)
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
	case OpField, OpIndex, OpPayload:
		if !has && c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
			return set(c.readInfo(in.Args[0]))
		}
	}
	return false
}

// widen merges b into dst's sink; a sink created here blames b's borrow, as sharing b did.
func (c *borrowCheck) widen(dst LocalID, b *borrowInfo) {
	c.merge(c.sink(dst, b.node), b)
}

// widens reports whether dst keeps the union of every value written to it instead
// of the first one: a store target, or a named local of a type carrying a lifetime.
func (c *borrowCheck) widens(dst LocalID) bool {
	if c.targets[dst] {
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
		if !c.elementHolder(c.f.Locals[a].Type) {
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
	if in.Dst == 0 || !c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
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
