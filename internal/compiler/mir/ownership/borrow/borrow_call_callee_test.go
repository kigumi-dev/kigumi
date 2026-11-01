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
				if e, _ := c.flowCallee(in); e != 0 {
					t.Errorf("unknown function parameter resolved to %d", e)
				}
				seen["unknown"] = true
			}
			if in.Op != midir.OpFutureNew {
				continue
			}
			e, args := c.flowCallee(in)
			if e == 0 || len(args) != 2 || in.Ent != 0 {
				t.Fatalf("direct future callee = %d, args = %v, instruction = %+v", e, args, in)
			}
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
					targets, values := c.carryFlows(call)
					if len(values) != 1 || values[0] != args[0] {
						t.Fatalf("%s: carried values = %v, want %v", call.Op, values, args[:1])
					}
					assertSignatureFlow(t, targets, e, args)
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
