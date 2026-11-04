package borrow

import (
	"fmt"
	"slices"
	"testing"

	mirflow "kigumi/internal/compiler/mir/flow"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const calleeLayoutSrc = `
pub type V['a] = { pub s &'a String }
pub type Sink['a] = { pub mut items &'a mut Array[V['a]] }
interface Pusher['a] {
    fn pushRef(move self, x: &'a String)
}
fn Sink['a].pushRef(move self, x: &'a String) { self.items.push(V { s: x }) }
fn keepK['a, K: Pusher['a]](x: &'a String, k: K) { k.pushRef(x) }
pub type H = { pub n Int }
fn H.keepG['a, K: Pusher['a]](self, x: &'a String, k: K) { k.pushRef(x) }
fn H.pick['a](self, x: &String, y: &'a String) -> &'a String { y }
fn H.pickG['a, K: Pusher['a]](self, x: &String, y: &'a String, k: K) -> &'a String { y }
fn direct['a](x: &'a String, y: &String, k: Sink['a], k2: Sink['a], k3: Sink['a]) {
    let h = H { n: 1 }
    keepK(x, k)
    h.keepG(x, k2)
    let p = h.pick(y, x)
    let q = h.pickG(y, x, k3)
}
fn viaValue['a](x: &'a String, y: &String, k: Sink['a], k2: Sink['a], k3: Sink['a]) {
    let h = H { n: 1 }
    let g: fn(&'a String, Sink['a]) -> Void = keepK
    let m: fn(&'a String, Sink['a]) -> Void = h.keepG
    let pm = h.pick
    let qm: fn(&String, &'a String, Sink['a]) -> &'a String = h.pickG
    g(x, k)
    m(x, k2)
    let p = pm(y, x)
    let q = qm(y, x, k3)
}
`

// TestCalleeOriginLaysOutLikeDirectCall: a bind with witnesses, a method value and an applied method value hand their
// body, for every check and for the result's lifetime, the operands a direct call to that body passes.
func TestCalleeOriginLaysOutLikeDirectCall(t *testing.T) {
	p, body := srcProgram(t, calleeLayoutSrc)
	direct, value := body("direct"), body("viaValue")
	want, wantLt := map[string][]string{}, map[string][]string{}
	dc := &borrowCheck{p: p.input, state: New(p.input, nil), f: direct}
	for _, in := range insts(direct, midir.OpCall) {
		name := p.R.Entity(in.Ent).Name
		want[name] = roles(direct, in.Args)
		wantLt[name] = dc.ltRoles(in.Ent, in.Args)
	}
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: value}
	seen := 0
	for _, in := range insts(value, midir.OpCallValue) {
		origins, ok := c.calleeOrigins(value, mirflow.SoleDefs(value), in.Args[0])
		if !ok || len(origins) != 1 || origins[0].body == nil {
			t.Fatalf("callv %v: origins %+v, %v", in.Args, origins, ok)
		}
		o := origins[0]
		name := p.R.Entity(o.ent).Name
		got := roles(value, o.params(in.Args[1:]))
		if len(got) != len(o.body.Params) || !slices.Equal(got, want[name]) {
			t.Errorf("%s: value call lays out %v, direct call %v", name, got, want[name])
		}
		for k := range in.Args[1:] {
			if j := o.argParam(k); got[j] != roles(value, in.Args[1+k:2+k])[0] {
				t.Errorf("%s: argument %d lands on parameter %d (%s)", name, k, j, got[j])
			}
		}
		if r, params, ok := c.valueResultCallee(in); !ok || r.ent != o.ent || !slices.Equal(c.ltRoles(r.ent, params), wantLt[name]) {
			t.Errorf("%s: result lifetime reads %v, direct call %v", name, c.ltRoles(r.ent, params), wantLt[name])
		}
		seen++
	}
	if seen != 4 || len(want) != 4 || !slices.Contains(wantLt["pick"], "param x") || !slices.Contains(wantLt["pickG"], "param x") {
		t.Fatalf("value calls %d, direct calls %v, lifetimes %v", seen, want, wantLt)
	}
}

func insts(g *midir.Func, op midir.Opcode) []midir.Inst {
	var out []midir.Inst
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				out = append(out, in)
			}
		}
	}
	return out
}

// roles names what each local is built from, through copies: a parameter, or the instruction defining it.
func roles(g *midir.Func, ls []midir.LocalID) []string {
	defs := mirflow.SoleDefs(g)
	var out []string
	for _, l := range ls {
		l = copiedFrom(defs, l)
		switch d := defs[l]; {
		case slices.Contains(g.Params, l):
			out = append(out, "param "+g.Locals[l].Name)
		case d == nil:
			out = append(out, "?")
		default:
			out = append(out, fmt.Sprintf("%v %d", d.Op, d.Ent))
		}
	}
	return out
}

// ltRoles are the roles of the operands a call passing args runs e with whose lifetime is e's result lifetime.
func (c *borrowCheck) ltRoles(e sem.EntityID, args []midir.LocalID) []string {
	info := c.p.R.Fn(e)
	var out []string
	if lt := c.fnRetLifetime(info); lt != 0 {
		for _, cand := range c.retLifetimeCandidates(info, lt, 0) {
			out = append(out, roles(c.f, args[cand.arg:cand.arg+1])...)
		}
	}
	return out
}
