package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// promoteCaptures gives every closure that captured ent a shared cell, even
// ones already closed: ctx.list aliases the backing array copied into ClosureInfo.
func (c *checker) promoteCaptures(n syntax.NodeID, ent EntityID, selfCall bool) {
	sites := c.captureSites[ent]
	c.markOpenWritten(ent)
	if len(sites) == 0 || c.movedCaptureWrite(n, ent, sites) {
		return
	}
	c.r.Entities[ent].Flags |= EfCapturedMut
	c.noteMutCapture(n, ent, selfCall)
	for _, site := range sites {
		if site.ctx.list[site.idx].Mode == capCopy {
			site.ctx.list[site.idx].Mode = capCell
		}
	}
}

// Closures own a move-only capture: an outside write reinitializes the local, an inside one goes to the closure's own cell; a write from a lambda passed to a call makes the local a cell that reading closures share.
func (c *checker) movedCaptureWrite(n syntax.NodeID, ent EntityID, sites []captureSite) bool {
	if c.copyNow(c.r.Entities[ent].Type) {
		return false
	}
	keeps, takes, shared := false, false, false
	for _, s := range sites {
		switch m := s.ctx.list[s.idx].Mode; {
		case m == capMove:
			keeps, takes = true, true
		case m == capRead:
			keeps = true
		case m == capBorrow && withinKeeper(c, s.ctx, sites):
		default:
			shared = true
		}
	}
	if !keeps || shared && !takes {
		return false
	}
	if shared {
		c.errAt(n, cCaptureKeptShared, c.r.Entities[ent].Name, c.r.Entities[ent].Name)
		return true
	}
	for _, ctx := range c.lambdas {
		if _, ok := ctx.captured[ent]; !ok {
			continue
		}
		for _, s := range sites {
			if c.scopeWithin(s.ctx.scope, ctx.scope) {
				s.ctx.list[s.idx].OwnCell = true
			}
		}
	}
	return true
}

func withinKeeper(c *checker, ctx *lambdaCtx, sites []captureSite) bool {
	for _, k := range sites {
		if k.ctx != ctx && k.ctx.list[k.idx].Mode == capMove && c.scopeWithin(ctx.scope, k.ctx.scope) {
			return true
		}
	}
	return false
}

// ownCellAbove: a closure nested in one that keeps ent in its own cell shares that cell.
func (c *checker) ownCellAbove(i int, ent EntityID) bool {
	for _, outer := range c.lambdas[:i] {
		if j, ok := outer.captured[ent]; ok && outer.list[j].OwnCell {
			return true
		}
	}
	return false
}

// demoteReadCaptures: a stored closure that neither consumes nor writes a move-only capture shares it instead of taking it.
func (c *checker) demoteReadCaptures(ctx *lambdaCtx) {
	if ctx.immediate {
		return
	}
	for i, cp := range ctx.list {
		if cp.Mode != capMove || cp.OwnCell || slices.Contains(ctx.writes, cp.Local) || consumesCapture(ctx, cp.Local) {
			continue
		}
		ctx.list[i].Mode = capRead
	}
}

func consumesCapture(ctx *lambdaCtx, ent EntityID) bool {
	for _, k := range ctx.consumes {
		if k.place.ent == ent {
			return true
		}
	}
	return false
}

// rejectKeptMut: a closure that takes a local some other closure writes through a shared cell would give it two owners; one that only reads shares the cell.
func (c *checker) rejectKeptMut(ctx *lambdaCtx) {
	for _, cp := range ctx.list {
		if n, ok := ctx.keptMut[cp.Local]; ok && cp.Mode == capMove {
			name := c.r.Entities[cp.Local].Name
			c.errAt(n, cCaptureKeptShared, name, name)
		}
	}
}
