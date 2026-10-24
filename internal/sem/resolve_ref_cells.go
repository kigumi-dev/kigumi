package sem

import (
	"slices"

	"kigumi/internal/syntax"
)

// A `&mut T` binding gets a cell only via one the caller lends over the place, decided per binding so both sides agree on shape.

// refCellEdge propagates backwards: a cell at `to` forces one at `from`.
type refCellEdge struct{ from, to EntityID }

// ent 0 marks a receiver this body cannot lend a cell over.
type refCellRecv struct {
	ent     EntityID
	callee  EntityID
	file    FileID
	node    syntax.NodeID
	witness bool
	what    string
	iface   TypeID
}

type refCellSite struct {
	ent   EntityID
	file  FileID
	node  syntax.NodeID
	async bool
}

// A scalar `&mut` is boxed at every call site already; only the rest of `&mut T` is decided here.
func (r *Result) wholeRefBinding(t TypeID) bool {
	if _, scalar := r.ScalarRefElem(t); scalar {
		return false
	}
	_, ok := r.MutRefElem(t)
	return ok
}

func (c *checker) refBinding(n syntax.NodeID) (EntityID, bool) {
	ent, ok := c.identPlaceEnt(n)
	if !ok {
		return 0, false
	}
	if k := c.r.Entities[ent].Kind; k != EntParam && k != EntLocal {
		return 0, false
	}
	return ent, c.r.wholeRefBinding(c.r.Entities[ent].Type)
}

func (c *checker) noteWholeRefAssign(lhs syntax.NodeID) {
	if ent, ok := c.refBinding(lhs); ok {
		c.r.refCellSeeds = append(c.r.refCellSeeds, ent)
		c.noteRefCellSite(ent, lhs, false)
	}
}

// param 0 means no declaration to read, so the binding is taken as needing a cell.
func (c *checker) noteRefArg(a syntax.NodeID, param EntityID, async bool) {
	c.noteSelfLentArg(a, param)
	ent, ok := c.refBinding(a)
	if !ok {
		return
	}
	if param == 0 {
		c.r.refCellSeeds = append(c.r.refCellSeeds, ent)
	} else {
		c.r.refCellEdges = append(c.r.refCellEdges, refCellEdge{from: ent, to: param})
	}
	c.noteRefCellSite(ent, a, async)
}

// The caller builds the cell over the place already, so only the async shape can still fail.
func (c *checker) noteRefPlaceArg(a syntax.NodeID, param EntityID, async bool) {
	if param == 0 || !async {
		return
	}
	if c.r.wholeRefBinding(c.r.Entities[param].Type) {
		c.r.refCellArgs = append(c.r.refCellArgs, refCellSite{ent: param, file: c.f, node: a, async: true})
	}
}

func (c *checker) noteRefCellSite(ent EntityID, n syntax.NodeID, async bool) {
	c.r.refCellSites = append(c.r.refCellSites, refCellSite{ent: ent, file: c.f, node: n, async: async})
}

func (r *Result) RefCellParam(ent EntityID) bool { return r.refCellParams[ent] }

func (r *Result) resolveRefCells() {
	r.refCellParams = map[EntityID]bool{}
	r.closeSelfReplacement()
	var soft []EntityID
	for _, q := range r.refCellRecvs {
		switch {
		case !r.recvReplaces(q):
		case q.ent == 0 && q.what != "":
			r.errAt(q.file, q.node, cRecvRefNoPlace, q.what, r.Entities[q.callee].Name)
		case q.ent == 0:
			r.errAt(q.file, q.node, cRefCellLentRecv, r.Entities[q.callee].Name)
		case q.witness:
			soft = append(soft, q.ent)
		default:
			r.refCellSeeds = append(r.refCellSeeds, q.ent)
		}
	}
	edges := append(r.refCellEdges, r.requirementRefEdges()...)
	r.refCellEdgeList = edges
	r.refCellHard = closeRefCells(r.refCellSeeds, edges, map[EntityID]bool{})
	closeRefCells(r.refCellSeeds, edges, r.refCellParams)
	closeRefCells(soft, edges, r.refCellParams)
	r.reportRefCellSites()
	r.rejectAsyncSelfReplace()
	r.reportLentRefArgs()
	r.conformSealed = len(r.conformMemo)
}

func (r *Result) recvReplaces(q refCellRecv) bool {
	if q.witness {
		return r.witnessReplacesSelf(q.callee, q.iface)
	}
	return r.replacesSelf(q.callee)
}

// The cells decided so far, without reporting; the final pass in resolveRefCells reads the settled replacements.
func (r *Result) provisionalRefCells() map[EntityID]bool {
	seeds := slices.Clone(r.refCellSeeds)
	for _, q := range r.refCellRecvs {
		if q.ent != 0 && r.recvReplaces(q) {
			seeds = append(seeds, q.ent)
		}
	}
	return closeRefCells(seeds, append(slices.Clone(r.refCellEdges), r.requirementRefEdges()...), map[EntityID]bool{})
}

// A cell at `to` forces one at `from`.
func closeRefCells(seeds []EntityID, edges []refCellEdge, into map[EntityID]bool) map[EntityID]bool {
	for _, ent := range seeds {
		into[ent] = true
	}
	for changed := true; changed; {
		changed = false
		for _, e := range edges {
			if into[e.to] && !into[e.from] {
				into[e.from], changed = true, true
			}
		}
	}
	return into
}

// Only ties parameters when both the requirement's and witness's are `&mut T`.
func (r *Result) requirementRefEdges() []refCellEdge {
	var out []refCellEdge
	for _, p := range r.witnessPairs {
		rp, wp := r.Fn(p.req).Params, r.Fn(p.m).Params
		if len(rp) != len(wp) {
			continue
		}
		for i := range rp {
			if r.wholeRefBinding(r.Entities[rp[i]].Type) && r.wholeRefBinding(r.Entities[wp[i]].Type) {
				out = append(out, refCellEdge{from: rp[i], to: wp[i]}, refCellEdge{from: wp[i], to: rp[i]})
			}
		}
	}
	return out
}

// A write-back cannot reach a borrow bound by `let`, or an async argument, whose future outlives this frame.
func (r *Result) reportRefCellSites() {
	seen := map[EntityID]bool{}
	for _, s := range append(r.refCellSites, r.refCellArgs...) {
		if !r.refCellParams[s.ent] {
			continue
		}
		if s.async {
			r.errAt(s.file, s.node, cRefCellAsyncArg)
			continue
		}
		// One report per binding, not per use of it.
		if r.Entities[s.ent].Kind == EntLocal && !seen[s.ent] {
			seen[s.ent] = true
			r.errAt(s.file, s.node, cRefCellNoOwner, r.Entities[s.ent].Name)
		}
	}
}
