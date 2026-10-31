package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

// cellUse is a place a closure reaches in a cell it shares: a field path (nil for the whole value), read or written.
type cellUse struct {
	path   ownquery.Path
	writes bool
}

type closureUseKey struct {
	closure sem.EntityID
	idx     int
}

// cellRunHit reports whether in may run a closure reaching o's place in a way the borrow forbids, and whether it writes:
// a call or await that can run or drop an argument holding it, a drop that may run a user `drop`, or an async body's end.
func (c *borrowCheck) cellRunHit(in midir.Inst, o ownerRef) (hit, writes bool) {
	return c.cellRunHitExcept(in, o, nil)
}

// cellRunHitExcept is cellRunHit leaving out each closure skip names for the value in would run it through.
func (c *borrowCheck) cellRunHitExcept(in midir.Inst, o ownerRef, skip func(midir.LocalID, sem.EntityID) bool) (hit, writes bool) {
	return c.runHitVia(in, o, skip, cellReach)
}

func (c *borrowCheck) runHitVia(in midir.Inst, o ownerRef, skip func(midir.LocalID, sem.EntityID) bool, reach capReach) (hit, writes bool) {
	loc := c.f.Locals[o.owner]
	if !loc.Cell || loc.Ent == 0 {
		return false, false
	}
	check := func(a midir.LocalID) {
		for _, cl := range c.argClosures(a) {
			if skip != nil && skip(a, cl) {
				continue
			}
			for i, cp := range c.p.R.Closure(cl).Captures {
				if cp.Local != loc.Ent || !reach(cp.Mode) {
					continue
				}
				for _, u := range c.closureUses(cl, i) {
					if ownquery.FieldPathsOverlap(u.path, o.path) && (u.writes || o.mut) {
						hit, writes = true, writes || u.writes
					}
				}
			}
		}
	}
	if taskClose(in) {
		check(c.spawnScope())
	}
	if in.Op == midir.OpDrop && c.dropRuns(in.Args[0]) || c.replacesDropping(in) {
		check(in.Args[0])
	}
	for i, a := range in.Args {
		if c.argRunnable(in, i) || (in.Op == midir.OpCall || in.Op == midir.OpCallValue) && c.dropRuns(a) {
			check(a)
		}
	}
	return hit, writes
}

func taskClose(in midir.Inst) bool { return in.Op == midir.OpBuiltin && in.Str == "task.close" }

// replacesDropping reports a store that replaces a value whose user `drop` may run: the old element, field or cell content.
func (c *borrowCheck) replacesDropping(in midir.Inst) bool {
	if in.Op != midir.OpSetField && in.Op != midir.OpSetIndex && in.Op != midir.OpCellSet {
		return false
	}
	return c.typeRuns(c.f.Locals[in.Args[len(in.Args)-1]].Type, runsDrop) || c.dropRuns(in.Args[0])
}

// checkCellRunArgs reports a borrow handed to a call that also runs a closure reaching the borrowed place: a
// borrow the loop over holders live after the call cannot see, or a `&mut` passed through a write-back cell.
func (c *borrowCheck) checkCellRunArgs(in midir.Inst, liveAfter map[midir.LocalID]bool) {
	c.checkLentTwice(in)
	for _, a := range in.Args {
		b, ok := c.holders[a]
		if ps := c.lentMutPlaces(a); !ok && len(ps) > 0 {
			for _, p := range ps {
				o := ownerRef{owner: p.root, mut: true, path: p.path, exact: true}
				if hit, _ := c.cellRunHit(in, o); hit && c.placeCell(p.root) && in.Op != midir.OpAwait {
					c.reportTouch(in, a, &borrowInfo{owners: []ownerRef{o}, hasOwner: true, node: p.node}, o)
				}
			}
			continue
		}
		if !ok || !b.hasOwner || liveAfter[a] || in.Op == midir.OpAwait {
			continue
		}
		for _, o := range b.owners {
			if hit, _ := c.cellRunHit(in, o); hit && !o.view {
				c.reportTouch(in, a, b, o)
			}
		}
	}
}

// checkLentTwice reports two `&mut` write-back cells of overlapping field places among one call's arguments; a scalar
// `&mut` is a holder the holder loop checks, the receiver is checkRecvArgs', and places through an index may be disjoint.
func (c *borrowCheck) checkLentTwice(in midir.Inst) {
	if in.Op != midir.OpCall && in.Op != midir.OpCallValue {
		return
	}
	first := 0
	if in.Op == midir.OpCall && in.Ent != 0 && c.p.R.Entity(in.Ent).Kind == sem.EntFn && c.p.R.Fn(in.Ent).Recv != sem.RecvNone {
		first = 1
	}
	var seen []lentPlace
	for k := first; k < len(in.Args); k++ {
		a := in.Args[k]
		ps := c.lentMutPlaces(a)
		if _, held := c.holders[a]; len(ps) != 1 || !ps[0].exact || held {
			continue
		}
		for _, p := range seen {
			if p.root == ps[0].root && ownquery.FieldPathsOverlap(p.path, ps[0].path) {
				name := ps[0].root
				if ps[0].hasVia {
					name = ps[0].via
				}
				n := c.f.Locals[name].Name
				c.reportHelped(in.Node, name, "borrow-conflict", "`"+n+"` is passed twice as `&mut`; the callee would get two exclusive borrows of the same value", 0, "",
					"split the call so each `&mut "+n+"` goes to a call of its own, or pass a different place for one of them")
			}
		}
		seen = append(seen, ps[0])
	}
}

// argClosures are the closures a value may hold: those its type names, and those it was built from here.
func (c *borrowCheck) argClosures(a midir.LocalID) []sem.EntityID {
	var out []sem.EntityID
	if a != c.spawnScope() {
		out = c.typeClosures(c.f.Locals[a].Type)
	}
	for f := range c.heldClosures(a) {
		if cl := sem.EntityID(f); f != dropFact && !slices.Contains(out, cl) {
			out = append(out, cl)
		}
	}
	return out
}

// typeClosures are the closure types a value of t may hold: t itself, its type arguments, and what those closures capture.
func (c *borrowCheck) typeClosures(t sem.TypeID) []sem.EntityID {
	if out, ok := c.closureTypeMemo[t]; ok || t == 0 {
		return out
	}
	if c.closureTypeMemo == nil {
		c.closureTypeMemo = map[sem.TypeID][]sem.EntityID{}
	}
	c.closureTypeMemo[t] = nil
	var out []sem.EntityID
	add := func(cls []sem.EntityID) {
		for _, cl := range cls {
			if !slices.Contains(out, cl) {
				out = append(out, cl)
			}
		}
	}
	n := c.p.R.Types.Node(t)
	if n.Kind == sem.KClosure && n.Ent != 0 && c.p.R.Entity(n.Ent).Kind == sem.EntClosure {
		add([]sem.EntityID{n.Ent})
		for _, cp := range c.p.R.Closure(n.Ent).Captures {
			add(c.typeClosures(c.p.R.Entity(cp.Local).Type))
		}
	}
	add(c.typeClosures(n.Elem))
	for _, a := range n.Args {
		add(c.typeClosures(a))
	}
	c.closureTypeMemo[t] = out
	return out
}

// closureUses are the places closure cl reaches in the cell it captures at idx; a body not lowered may write all of it.
func (c *borrowCheck) closureUses(cl sem.EntityID, idx int) []cellUse {
	key := closureUseKey{cl, idx}
	if out, ok := c.closureUseMemo[key]; ok {
		return out
	}
	if c.closureUseMemo == nil {
		c.closureUseMemo = map[closureUseKey][]cellUse{}
	}
	g := c.p.ByEnt[cl]
	out := []cellUse{{nil, true}}
	if g != nil && idx < len(g.Params) {
		out = cellUses(c.p, g, g.Params[idx], 0)
	}
	c.closureUseMemo[key] = out
	return out
}
