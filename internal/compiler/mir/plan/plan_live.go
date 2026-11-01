package plan

import (
	"slices"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// liveCtx releases a temporary once on every path: after its last read, or on the edge to a block that no longer needs it.
type liveCtx struct {
	g      *flowGraph
	isTemp func(midir.LocalID) bool
	life   *borrowLife
	points map[Point][]midir.LocalID
	edges  map[mir.Edge][]midir.LocalID
}

func newLiveCtx(f *midir.Func, isTemp func(midir.LocalID) bool, life *borrowLife, points map[Point][]midir.LocalID) *liveCtx {
	return &liveCtx{g: newFlowGraph(f), isTemp: isTemp, life: life, points: points, edges: map[mir.Edge][]midir.LocalID{}}
}

func noteBlock(m map[midir.LocalID]map[int]bool, t midir.LocalID, b int) {
	if m[t] == nil {
		m[t] = map[int]bool{}
	}
	m[t][b] = true
}

func (c *liveCtx) reads(args []midir.LocalID, t midir.LocalID) bool { return c.life.reads(args, t) }

func (c *liveCtx) defines(in midir.Inst, t midir.LocalID) bool {
	return in.Dst == t && definesTemp(in, c.isTemp)
}

func (c *liveCtx) summary(b int, t midir.LocalID) (exposed, defined bool) {
	blk := c.g.f.Blocks[b]
	for _, in := range blk.Insts {
		if !defined && c.reads(in.Args, t) {
			exposed = true
		}
		if c.defines(in, t) {
			defined = true
		}
	}
	return exposed || !defined && c.reads(blk.Term.Args, t), defined
}

// solve reports false when a read is reachable without a definition.
func (c *liveCtx) solve(t midir.LocalID, touch map[int]bool) (liveIn, liveOut map[int]bool, ok bool) {
	exposed, defined := map[int]bool{}, map[int]bool{}
	var work []int
	liveIn, liveOut = map[int]bool{}, map[int]bool{}
	for b := range touch {
		e, d := c.summary(b, t)
		exposed[b], defined[b] = e, d
		if e {
			liveIn[b] = true
			work = append(work, b)
		}
	}
	for len(work) > 0 {
		b := work[len(work)-1]
		work = work[:len(work)-1]
		for _, p := range c.g.preds[b] {
			liveOut[p] = true
			if !defined[p] && !liveIn[p] {
				liveIn[p] = true
				work = append(work, p)
			}
		}
	}
	return liveIn, liveOut, !liveIn[0]
}

func (c *liveCtx) leaveEdges(t midir.LocalID, liveIn, liveOut map[int]bool) {
	for x := range liveOut {
		for slot, y := range c.g.succ(x) {
			if !liveIn[int(y)] {
				e := mir.Edge{From: x, Slot: slot}
				c.edges[e] = append(c.edges[e], t)
			}
		}
	}
}

// place reports false, recording nothing, when a read is reachable without a definition.
func (c *liveCtx) place(t midir.LocalID, touch map[int]bool) bool {
	liveIn, liveOut, ok := c.solve(t, touch)
	if !ok {
		return false
	}
	for b := range touch {
		c.inBlock(t, b, liveOut[b])
	}
	c.leaveEdges(t, liveIn, liveOut)
	return true
}

// placeAbandoned releases a temporary a later read consumes on the edges that leave its live range early; the consuming read itself releases nothing.
func (c *liveCtx) placeAbandoned(t midir.LocalID, touch map[int]bool) {
	if liveIn, liveOut, ok := c.solve(t, touch); ok {
		c.leaveEdges(t, liveIn, liveOut)
	}
}

func (c *liveCtx) inBlock(t midir.LocalID, b int, live bool) {
	blk := c.g.f.Blocks[b]
	if c.reads(blk.Term.Args, t) && !live {
		c.release(Point{b, len(blk.Insts)}, t)
		live = true
	}
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		in := blk.Insts[i]
		def, read := c.defines(in, t), c.reads(in.Args, t)
		if (def || read) && !live {
			c.release(Point{b, i}, t)
		}
		if def {
			live = false
		}
		if read {
			live = true
		}
	}
}

func (c *liveCtx) release(at Point, t midir.LocalID) {
	if !slices.Contains(c.points[at], t) {
		c.points[at] = append(c.points[at], t)
	}
}

// edgePoints returns the edge releases that need a block of their own.
func (c *liveCtx) edgePoints() map[mir.Edge][]midir.LocalID {
	split := map[mir.Edge][]midir.LocalID{}
	for e, ts := range c.edges {
		slices.Sort(ts)
		blk := c.g.f.Blocks[e.From]
		to := int(blk.Term.Targets[e.Slot])
		switch {
		case len(blk.Term.Targets) == 1:
			at := Point{e.From, len(blk.Insts)}
			c.points[at] = append(c.points[at], ts...)
		case len(c.g.preds[to]) == 1:
			at := Point{to, -1}
			c.points[at] = append(c.points[at], ts...)
		default:
			split[e] = ts
		}
	}
	return split
}
