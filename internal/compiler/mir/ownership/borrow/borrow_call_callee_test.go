package borrow

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const flowCalleeSrc = `
type RH['a] = { pub mut r &'a Array[Int] }
async fn putA['a](c: &'a Array[Int], h: &mut RH['a]) -> usize {
    h.r = c
    0
}
async fn carryA(f: Future[usize], h: &mut Array[Future[usize]]) {
    h.push(f)
}
fn caller(w: fn(Int) -> Int, f: Future[usize]) {
    let z = Array.of(1)
    let mut hold = RH { r: &z }
    let _ = putA(&z, &mut hold)
    let mut hs = Array.empty[Future[usize]]()
    let _ = carryA(f, &mut hs)
    let _ = w(1)
}
`

func TestCallFlowsResolveCallee(t *testing.T) {
	p, body := srcProgram(t, flowCalleeSrc)
	g := body("caller")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g, holders: map[midir.LocalID]*borrowInfo{}}
	seen := map[string]bool{}
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpCallValue && in.Ent == 0 {
				if ts := c.flowCallees(in); len(ts) != 0 {
					t.Errorf("unknown function parameter resolved to %+v", ts)
				}
				seen["unknown"] = true
			}
			if in.Op != midir.OpFutureNew {
				continue
			}
			ts := c.flowCallees(in)
			if len(ts) != 1 || len(ts[0].args) != 2 || ts[0].kept != nil || in.Ent != 0 {
				t.Fatalf("direct future callees = %+v, instruction = %+v", ts, in)
			}
			e, args := ts[0].ent, ts[0].args
			name := p.R.Entity(e).Name
			for _, call := range []midir.Inst{
				in,
				{Op: midir.OpCall, Ent: e, Args: args},
				{Op: midir.OpCallValue, Ent: e, Args: in.Args},
				{Op: midir.OpCallValue, Args: in.Args},
			} {
				if name == "putA" {
					assertSignatureFlow(t, c.callFlows(call), e, args)
				}
				if name == "carryA" {
					c.holders[args[0]] = &borrowInfo{}
					assertSignatureFlow(t, c.carryFlows(call), e, args)
				}
			}
			seen[name] = true
		}
	}
	for _, name := range []string{"putA", "carryA", "unknown"} {
		if !seen[name] {
			t.Errorf("missing %s call", name)
		}
	}
}

func assertSignatureFlow(t *testing.T, flows []storeFlow, e sem.EntityID, args []midir.LocalID) {
	t.Helper()
	if len(flows) != 1 {
		t.Fatalf("flows = %+v, want one", flows)
	}
	f := flows[0]
	if f.callee != e || f.target != args[1] || f.param != 1 || !f.ref || len(f.values) != 1 || f.values[0].arg != args[0] {
		t.Errorf("flow = %+v, callee = %d, args = %v", f, e, args)
	}
}
