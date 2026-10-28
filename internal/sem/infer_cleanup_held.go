package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

type heldMuts struct {
	holders       map[EntityID][]mutHolder
	closureWrites map[EntityID][]EntityID
	plainAssigns  []assignSite
	// patWhole is set for the pattern of a `let`, which moves its whole initializer.
	patWhole    bool
	mutArgParam map[syntax.NodeID]EntityID
	scalarCopy  map[EntityID]scalarCopy
	copyHold    map[EntityID]EntityID
	copyLit     map[syntax.NodeID]EntityID
	// holderAssigns lists the values a `let mut` holder of a `&mut` is rebound to after its initializer.
	holderAssigns map[EntityID][]syntax.NodeID
	// partOf maps a `for` or pattern binding that can carry a `&mut` to the value it was taken from.
	partOf      map[EntityID]syntax.NodeID
	partAssigns map[EntityID][]partAssign
	reroot      loopReroot
	patDepth    int
	// reborrows: `&mut` field arguments lent to one synchronous call instead of moved.
	reborrows map[syntax.NodeID]bool
	// wantRead: the next synth reads through a reference, so a `&mut` field is not taken.
	wantRead bool

	// localWrites lists where each local is written (assignment, `&mut`, `mut self` call), for captures in a loop or call.
	localWrites map[EntityID][]syntax.NodeID
}

type mutHolder struct {
	root   EntityID
	fields []EntityID
	node   syntax.NodeID
}

func (c *checker) noteMutHolder(local EntityID, init syntax.NodeID) {
	init = c.stripParen(init)
	var held []mutHolder
	switch c.t.Kind(init) {
	case syntax.BorrowExpr:
		b := c.t.View(init).AsBorrow()
		if b.Flags()&syntax.FlagMut == 0 {
			return
		}
		if p, ok := c.buildPlace(b.Operand().ID()); ok && p.Root != 0 && !p.Deref {
			held = append(held, mutHolder{root: p.Root, fields: p.Fields, node: init})
		}
	case syntax.Lambda:
		for _, ent := range c.held.closureWrites[c.info.Defs[init]] {
			held = append(held, mutHolder{root: ent, node: init})
		}
	}
	held = slices.DeleteFunc(held, func(h mutHolder) bool { return len(c.reserved[h.root]) > 0 })
	if len(held) == 0 {
		return
	}
	if c.held.holders == nil {
		c.held.holders = map[EntityID][]mutHolder{}
	}
	c.held.holders[local] = held
}

func (c *checker) noteLambdaWrite(ent EntityID) {
	for _, ctx := range c.lambdas {
		if _, ok := ctx.captured[ent]; ok && !slices.Contains(ctx.writes, ent) {
			ctx.writes = append(ctx.writes, ent)
		}
	}
}

func (c *checker) reserveHolderUse(n syntax.NodeID, local EntityID) {
	if len(c.reserved) == 0 {
		return
	}
	for _, h := range c.held.holders[local] {
		if i, held := c.reservedWithin(h.root, h.fields); held {
			r := c.reserved[h.root][i]
			if d, ok := c.r.diagAt(c.f, n, cCleanupReservedBorrowMut, r.name); ok {
				c.r.emitDiag(c.f, d.WithNote(c.loc(r.cleanup), c.reserverNote(r)).WithNote(c.loc(h.node), "it was made here, before the cleanup"))
			}
			return
		}
	}
}

func (c *checker) noteClosureWrites(ctx *lambdaCtx) {
	if len(ctx.writes) == 0 {
		return
	}
	if c.held.closureWrites == nil {
		c.held.closureWrites = map[EntityID][]EntityID{}
	}
	c.held.closureWrites[ctx.ent] = ctx.writes
}
