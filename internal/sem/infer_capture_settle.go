package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// openCapture is a capture decided while its local's type was open; cell is the copy mode's verdict so far.
type openCapture struct {
	site captureSite
	ent  EntityID
	cell bool
}

func (c *checker) noteOpenCapture(ctx *lambdaCtx, n syntax.NodeID, ent EntityID) {
	e := &c.r.Entities[ent]
	if c.copyNow(e.Type) || !c.openType(e.Type, c.openCaptures) {
		return
	}
	c.openCaptures = append(c.openCaptures, openCapture{site: captureSite{ctx: ctx, idx: len(ctx.list) - 1}, ent: ent, cell: c.startsShared(ctx, n, ent)})
}

func (c *checker) markOpenWritten(ent EntityID) {
	for i := range c.openCaptures {
		if c.openCaptures[i].ent == ent {
			c.openCaptures[i].cell = true
		}
	}
}

// openType: t may still settle to a copy type, holding a type variable or naming a closure with a capture in open.
func (c *checker) openType(t TypeID, open []openCapture) bool {
	t = c.vars.resolve(t)
	if c.r.Types.ContainsVar(t) {
		return true
	}
	n := c.r.Types.Node(t)
	return n.Kind == KClosure && slices.ContainsFunc(open, func(oc openCapture) bool { return oc.site.ctx.ent == n.Ent })
}

// settleOpenCaptures gives a capture whose local settled to a copy type the mode a known type would have had; a closure
// turns copy once its own open captures do, so the rounds repeat until none settles.
func (c *checker) settleOpenCaptures() {
	for again := true; again; {
		open := c.openCaptures
		c.openCaptures = nil
		for _, oc := range open {
			if c.copyNow(c.r.Entities[oc.ent].Type) {
				c.recopyCapture(oc)
			} else {
				c.openCaptures = append(c.openCaptures, oc)
			}
		}
		again = len(c.openCaptures) < len(open)
	}
	for again := true; again; {
		open := c.openCaptures
		c.openCaptures = slices.DeleteFunc(slices.Clone(open), func(oc openCapture) bool { return !c.openType(c.r.Entities[oc.ent].Type, open) })
		again = len(c.openCaptures) < len(open)
	}
}

func (c *checker) recopyCapture(oc openCapture) {
	ctx := oc.site.ctx
	cp := &ctx.list[oc.site.idx]
	cp.Mode, cp.OwnCell = capCopy, false
	if oc.cell {
		cp.Mode = capCell
		c.r.Entities[oc.ent].Flags |= EfCapturedMut
	}
	ctx.consumes = slices.DeleteFunc(ctx.consumes, func(k lambdaConsume) bool { return k.place.ent == oc.ent })
	ctx.reserveUses = slices.DeleteFunc(ctx.reserveUses, func(u captureUse) bool { return u.ent == oc.ent })
	delete(ctx.keptMut, oc.ent)
	info := c.r.closure(ctx.ent)
	info.Copy, info.Once = closureCopy(ctx.list), c.lambdaOnce(ctx)
}
