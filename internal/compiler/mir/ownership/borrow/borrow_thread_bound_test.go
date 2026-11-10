package borrow

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const threadBoundSrc = `
import {Shared} from std/alloc
type Hub = { n Int }
fn Hub.ship[T: Send](self, x: T) {}
fn Hub.keep[T](self, x: T) {}
fn direct(h: Hub) { h.keep(Shared.new(1)) }
fn viaValue(h: Hub) {
    let k = h.keep
    k(Shared.new(2))
}
fn shipped(h: Hub) { h.ship(3) }
`

// TestThreadBoundBackstop turns a call of keep into one of ship, a value sem never saw reach a Send bound.
func TestThreadBoundBackstop(t *testing.T) {
	for _, c := range []struct {
		body string
		op   midir.Opcode
	}{{"direct", midir.OpCall}, {"viaValue", midir.OpClosure}} {
		t.Run(c.body, func(t *testing.T) {
			p, body := srcProgram(t, threadBoundSrc)
			ship, keep := methodNamed(t, p.R, "ship"), methodNamed(t, p.R, "keep")
			g := body(c.body)
			swapped := false
			for b := range g.Blocks {
				for i, in := range g.Blocks[b].Insts {
					if in.Op == c.op && in.Ent == keep {
						g.Blocks[b].Insts[i].Ent = ship
						swapped = true
					}
				}
			}
			if !swapped {
				t.Fatalf("no %s of keep in %s", c.op, c.body)
			}
			bc := &borrowCheck{p: p.input, state: New(p.input, nil), f: g, holders: map[midir.LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[midir.LocalID]bool{}}
			bc.checkThreadBounds()
			if len(g.Diags) != 1 || g.Diags[0].Code != "E300" || g.Diags[0].Loc.Span.Start == 0 {
				t.Fatalf("diagnostics %+v, want one E300 at the call", g.Diags)
			}
		})
	}
	t.Run("shipped", func(t *testing.T) {
		p, body := srcProgram(t, threadBoundSrc)
		g := body("shipped")
		bc := &borrowCheck{p: p.input, state: New(p.input, nil), f: g, holders: map[midir.LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[midir.LocalID]bool{}}
		bc.checkThreadBounds()
		if len(g.Diags) != 0 {
			t.Fatalf("a Send value was refused: %+v", g.Diags)
		}
	})
}

func methodNamed(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntFn && e.Name == name && r.Fn(sem.EntityID(id)).Recv != sem.RecvNone {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no method %s", name)
	return 0
}
