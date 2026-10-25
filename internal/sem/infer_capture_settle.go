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
	if c.copyNow(e.Type) || !c.r.Types.ContainsVar(c.vars.resolve(e.Type)) {
		return
	}
	cell := e.Flags&EfCapturedMut != 0 || !ctx.immediate && c.writtenInLoopOf(ent, n)
	c.openCaptures = append(c.openCaptures, openCapture{site: captureSite{ctx: ctx, idx: len(ctx.list) - 1}, ent: ent, cell: cell})
}

func (c *checker) markOpenWritten(ent EntityID) {
	for i := range c.openCaptures {
		if c.openCaptures[i].ent == ent {
			c.openCaptures[i].cell = true
		}
	}
}

// settleOpenCaptures gives a capture whose local settled to a copy type the mode a known type would have had.
func (c *checker) settleOpenCaptures() {
	kept := c.openCaptures[:0]
	for _, oc := range c.openCaptures {
		switch t := c.r.Entities[oc.ent].Type; {
		case c.copyNow(t):
			c.recopyCapture(oc)
		case c.r.Types.ContainsVar(c.vars.resolve(t)):
			kept = append(kept, oc)
		}
	}
	c.openCaptures = kept
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
