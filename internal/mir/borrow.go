package mir

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Borrow rules, checked per function on the MIR: a borrow may not
// escape (return, store, capture), the owner cannot be touched while an
// exclusive borrow is live, and cannot be mutated, moved or dropped while
// a shared borrow is live. Liveness is by last use.

// ownerRef is one local a borrow may actually alias, with the field chain
// (see projectionPlace) it was taken through.
type ownerRef struct {
	owner LocalID
	mut   bool
	path  []int
	exact bool
}

// borrowInfo tracks every owner a holder might alias, not just one: an ambiguous
// same-kind call result (sameKindCandidate) can't name its single true source.
type borrowInfo struct {
	owners   []ownerRef
	hasOwner bool
	node     syntax.NodeID
	// lifetime is the named lifetime a param-inherited holder (hasOwner false)
	// traces back to, 0 when it cannot be named.
	lifetime sem.EntityID
	// fromStatic marks a holder traced to a static's storage, so
	// checkLifetimeEscape can report it with its own wording.
	fromStatic bool
}

type borrowCheck struct {
	p       *Program
	f       *Func
	holders map[LocalID]*borrowInfo
	liveOut []map[LocalID]bool
	// definedIn holds the holders that may have been assigned on some
	// path into a block: a binding that `is` assigns only when its pattern
	// matches is live at the join but not yet defined before the test.
	definedIn []map[LocalID]bool
	reported  map[reportKey]bool
	quiet     bool
	// defs maps a temporary to its defining instruction (nil when defined
	// more than once), built on first use.
	defs map[LocalID]*Inst
	// chainOnly caches chainedOnly's result per local.
	chainOnly map[LocalID]bool
}

func analyzeBorrows(p *Program, f *Func) {
	c := &borrowCheck{p: p, f: f, holders: map[LocalID]*borrowInfo{}, reported: map[reportKey]bool{}}
	c.collect()
	if len(c.holders) == 0 {
		return
	}
	c.liveness()
	c.definitions()
	for bid := range f.Blocks {
		c.checkBlock(BlockID(bid))
	}
}

// collect finds the locals holding borrows: borrow results, copies and field reads
// of them, reference-typed parameters, and by-value params of a named-lifetime-generic type.
func (c *borrowCheck) collect() {
	for _, prm := range c.f.Params {
		typ := c.f.Locals[prm].Type
		lt, node := c.paramLifetime(prm)
		switch {
		case c.p.R.Types.Kind(typ) == sem.KRef:
			c.holders[prm] = &borrowInfo{lifetime: lt, node: node}
		case lt != 0:
			c.holders[prm] = &borrowInfo{lifetime: lt, node: node}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, blk := range c.f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case OpBorrow:
					if _, ok := c.holders[in.Dst]; !ok {
						if root, path, exact, ok := c.projectionPlace(in.Args[0]); ok {
							ref := ownerRef{owner: root, mut: in.Index == 1, path: path, exact: exact}
							c.holders[in.Dst] = &borrowInfo{owners: []ownerRef{ref}, hasOwner: true, node: in.Node}
							changed = true
						} else if def := c.definition(in.Args[0]); def != nil && def.Op == OpGlobalGet {
							// A static-origin borrow has no local owner to touch-check
							// against, but still has to reject escape like any other.
							c.holders[in.Dst] = &borrowInfo{node: in.Node, fromStatic: true}
							changed = true
						}
					}
				case OpCopy, OpMove:
					if src, ok := c.holders[in.Args[0]]; ok {
						if _, has := c.holders[in.Dst]; !has {
							c.holders[in.Dst] = src
							changed = true
						}
					}
				case OpRecord, OpCall, OpCallValue, OpBuiltin, OpFutureNew:
					if _, has := c.holders[in.Dst]; !has {
						if src, ok := c.lifetimeSource(in); ok {
							c.holders[in.Dst] = src
							changed = true
						}
					}
				case OpField:
					if _, has := c.holders[in.Dst]; !has && c.resultMayCarryLifetime(c.f.Locals[in.Dst].Type) {
						if src, ok := c.holders[in.Args[0]]; ok {
							c.holders[in.Dst] = src
							changed = true
						}
					}
				}
			}
		}
	}
}

// checkBlock walks a block backward, tracking the holders live after each
// instruction, and reports the conflicts and escapes.
func (c *borrowCheck) checkBlock(bid BlockID) {
	blk := &c.f.Blocks[bid]
	live := map[LocalID]bool{}
	for l := range c.liveOut[bid] {
		live[l] = true
	}
	for _, a := range blk.Term.Args {
		if b, ok := c.holders[a]; ok {
			// A holder with no owner is a borrow inherited from a
			// parameter, not one taken of a local: returning it is the
			// intended lifetime escape, and the caller's
			// own call site becomes the new holder (lifetimeSource).
			if !b.hasOwner {
				c.checkLifetimeEscape(a, b)
				live[a] = true
				continue
			}
			c.report(b.node, a, "borrow-escape", "a borrow cannot be returned")
			live[a] = true
		}
	}
	before := c.definedBefore(bid)
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		c.checkInst(in, intersect(live, before[i]))
		if _, ok := c.holders[in.Dst]; ok && in.Op != OpDrop && in.Op != OpSetField && in.Op != OpSetIndex && in.Op != OpCellSet && in.Op != OpBoxReplace && in.Op != OpGlobalSet && in.Op != OpFutureDone {
			delete(live, in.Dst)
		}
		for _, a := range uses(in) {
			if _, ok := c.holders[a]; ok {
				live[a] = true
			}
		}
	}
}

// checkInst sees the holders live after in; a conflict is a touch of an
// owner whose borrow outlives this instruction.
func (c *borrowCheck) checkInst(in Inst, liveAfter map[LocalID]bool) {
	c.checkCapture(in)
	c.checkSuspend(in, liveAfter)
	for holder := range liveAfter {
		b := c.holders[holder]
		if !b.hasOwner || holder == in.Dst {
			continue
		}
		for _, o := range b.owners {
			touched, path, exact := c.touch(in, o.owner)
			if !touched || !fieldPathsOverlap(path, o.path, exact, o.exact) {
				continue
			}
			mutates := in.Op == OpMove || in.Op == OpDrop || (in.Op == OpBorrow && in.Index == 1) ||
				in.Op == OpSetField || in.Op == OpSetIndex || in.Op == OpCellSet || in.Op == OpBoxReplace
			name := c.f.Locals[o.owner].Name
			switch {
			case in.Op == OpDrop && in.EndScope:
				c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is still borrowed when its scope ends here")
			case in.Op == OpMove || in.Op == OpDrop:
				c.report(in.Node, o.owner, "move-while-borrowed", "`"+name+"` is borrowed and cannot be moved or dropped here")
			case o.mut:
				c.report(in.Node, o.owner, "borrow-conflict", "`"+name+"` is exclusively borrowed and cannot be used here")
			case mutates || in.Dst == o.owner:
				c.report(in.Node, o.owner, "borrow-conflict", "`"+name+"` is borrowed and cannot be mutated here")
			}
		}
	}
}
