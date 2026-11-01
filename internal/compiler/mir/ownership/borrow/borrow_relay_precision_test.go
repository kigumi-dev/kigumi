package borrow

import (
	"testing"
)

func TestRelayKnownReadSummary(t *testing.T) {
	p, body := srcProgram(t, `
import {Map} from std/map
type V['a] = { pub s &'a String }
fn lookup['a](m: &Map[String, V['a]], key: String) -> V['a]? { m.get(&key) }
`)
	g := body("lookup")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
	sites := c.paramSites(paramKey{g: g, i: 0, kind: siteOut})
	for _, s := range sites {
		if s.callee.kind == relayUnknown {
			t.Errorf("known read has unknown relay: %v", sites)
		}
	}
}

func TestRelayReferenceOwnLifetime(t *testing.T) {
	p, body := srcProgram(t, `
type V['a] = { pub s &'a String }
fn scoped['a](v: &V['a]) { _ = v.s }
fn named['a](v: &'a V['a]) { _ = v.s }
`)
	for _, name := range []string{"scoped", "named"} {
		g := body(name)
		c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g}
		if got := c.paramKeepsRef(g, 0); got != (name == "named") {
			t.Errorf("%s keeps reference = %v", name, got)
		}
		if !c.paramKeepsBorrow(g.Locals[g.Params[0]].Type) {
			t.Errorf("%s cannot keep contents with a named lifetime", name)
		}
	}
}
