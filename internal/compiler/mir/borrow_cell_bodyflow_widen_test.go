package mir

import "testing"

// TestPlaceRootsWidensAReferenceFromElsewhere checks that a reference defined by anything but a projection, borrow,
// cell, copy or call is not pinned to locals.
func TestPlaceRootsWidensAReferenceFromElsewhere(t *testing.T) {
	p, member := bodyFlowProgram(t)
	refType := p.R.Entity(p.R.Fn(member("setF")).Params[0]).Type
	g := &Func{Locals: []Local{{}, {Type: refType}}}
	g.Blocks = []Block{{Insts: []Inst{{Op: OpGlobalGet, Dst: 1}}}}
	c := &borrowCheck{p: p, f: g}
	if _, ok := c.placeRoots(g, allDefs(g), 1); ok {
		t.Fatal("a reference read from a global was pinned to this body's locals")
	}
}

// TestCaptureStoresOfABusyClosureAreOpaque asks first for a closure whose body calls the fn building it, so the fn's
// flow meets the closure still being computed and must take its capture stores as a value from elsewhere.
func TestCaptureStoresOfABusyClosureAreOpaque(t *testing.T) {
	p, member := bodyFlowProgram(t)
	outer := p.ByEnt[member("recursingCapture")]
	var closure *Func
	for _, blk := range outer.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpClosure {
				closure = p.ByEnt[in.Ent]
			}
		}
	}
	c := &borrowCheck{p: p, f: closure}
	c.bodyFacts(closure)
	fs := c.bodyFacts(outer)
	for l, loc := range outer.Locals {
		if loc.Name == "g" && !fs[LocalID(l)][opaqueFact] {
			t.Fatal("g in the fn building a closure still being computed does not hold a value from elsewhere")
		}
	}
}
