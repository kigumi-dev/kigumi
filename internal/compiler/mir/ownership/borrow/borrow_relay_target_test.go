package borrow

import (
	"slices"
	"testing"

	mirflow "kigumi/internal/compiler/mir/flow"
)

const dispatchRelaySrc = `
fn two['a](g: fn(V['a]) -> Void, h: fn(V['a]) -> Void, b: Bool, x: V['a]) {
    apply((v: V['a]) => { applyIn((y: V['a]) => { if b { g(y) } else { h(y) } }, v) }, x)
}
fn three['a](g: fn(V['a]) -> Void, h: fn(V['a]) -> Void, j: fn(V['a]) -> Void, n: Int, x: V['a]) {
    apply((v: V['a]) => { applyIn((y: V['a]) => { match n { 0 -> g(y), 1 -> h(y), _ -> j(y) } }, v) }, x)
}
fn mixed['a](g: fn(V['a]) -> Void, b: Bool, x: V['a]) {
    let h = (v: V['a]) => { _ = v.s }
    apply((v: V['a]) => { applyIn((y: V['a]) => { if b { g(y) } else { h(y) } }, v) }, x)
}
fn quiet['a](b: Bool, x: V['a]) {
    let g = (v: V['a]) => { _ = v.s }
    let h = (v: V['a]) => { _ = v.s }
    apply((v: V['a]) => { applyIn((y: V['a]) => { if b { g(y) } else { h(y) } }, v) }, x)
}
fn unknown['a](g: fn(V['a]) -> Void, h: fn(V['a]) -> Void, b: Bool, x: V['a]) {
    let mut k = g
    if b { k = h }
    apply((v: V['a]) => { applyIn((y: V['a]) => { k(y) }, v) }, x)
}
`

func TestParamRelayDispatchTargets(t *testing.T) {
	p, body := srcProgram(t, relaySrc+dispatchRelaySrc)
	for _, tc := range []struct {
		name string
		want []relayTarget
	}{
		{"two", []relayTarget{{kind: relayParam, param: 0}, {kind: relayParam, param: 1}}},
		{"three", []relayTarget{{kind: relayParam, param: 0}, {kind: relayParam, param: 1}, {kind: relayParam, param: 2}}},
		{"mixed", []relayTarget{{kind: relayParam, param: 0}}},
		{"nested", []relayTarget{{kind: relayNone}}},
		{"quiet", nil},
		{"unknown", []relayTarget{{kind: relayUnknown}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := body(tc.name)
			c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
			sites := c.paramSites(paramKey{g: g, i: len(g.Params) - 1, kind: siteOut})
			var got []relayTarget
			for _, s := range sites {
				if !slices.Contains(got, s.callee) {
					got = append(got, s.callee)
				}
			}
			if len(got) != len(tc.want) || slices.ContainsFunc(tc.want, func(s relayTarget) bool { return !slices.Contains(got, s) }) {
				t.Errorf("targets = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRelayUnknownAndFirstParameter(t *testing.T) {
	p, body := srcProgram(t, relaySrc+dispatchRelaySrc)
	g := body("two")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	defs := mirflow.SoleDefs(g)
	if g.Params[0] != 0 {
		t.Fatalf("first parameter = %d, want local 0", g.Params[0])
	}
	o := c.calleeOrigin(g, defs, 0)
	if !o.isParam || o.param != 0 || !c.frameTarget(g, defs, relayTarget{kind: relayParam, param: 0}, g.Params, 8) {
		t.Fatal("first parameter was not treated as a possible storing callee")
	}
	for _, target := range []relayTarget{{kind: relayUnknown}, {kind: relayParam, param: len(g.Params)}} {
		if !c.frameTarget(g, defs, target, g.Params, 8) || c.quietCallee(storeFlow{args: g.Params}, paramStore{callee: target}) {
			t.Errorf("target %v was treated as quiet", target)
		}
	}
}
