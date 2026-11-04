package borrow

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
)

const refOriginSrc = `
interface FromRef[K] {
    fn fromRef(k: &K) -> Self
}
type View['a] = { pub s &'a String }
type Tag = { pub n usize }
fn View['a].fromRef(k: &'a String) -> View['a] { View { s: k } }
fn Tag.fromRef(k: &String) -> Tag { Tag { n: k.len() } }
type One[K, V] = { pub k K, pub vs Array[V] }
fn One[K, V: FromRef[K]].addSelf(self, dst: &mut Array[V]) { dst.push(V.fromRef(&self.k)) }
fn One[K, V: FromRef[K]].relaySelf(self, dst: &mut Array[V]) { self.addSelf(dst) }
fn apply0[V, F: Fn[fn() -> V]](dst: &mut Array[V], f: F) { dst.push(f()) }
fn viaCap2[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { apply0(dst, () => V.fromRef(k)) }
type KRef2['a] = { pub r &'a String }
fn addR2['a, V: FromRef[String]](dst: &mut Array[V], h: KRef2['a]) { dst.push(V.fromRef(h.r)) }
type RefOne['a] = { pub k &'a String }
fn RefOne['a].addK[V: FromRef[String]](self, dst: &mut Array[V]) { dst.push(V.fromRef(self.k)) }
fn pickFrom[K](ks: &Array[K], n: usize) -> &K { &ks[0] }
fn viaHelper[K, V: FromRef[K]](dst: &mut Array[V], ks: &Array[K]) {
    let n = dst.len()
    dst.push(V.fromRef(pickFrom(ks, n)))
}
fn addTo[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { dst.push(V.fromRef(k)) }
fn relay[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { addTo(dst, k) }
fn use() {
    let name = "n"
    let mut vs = Array.of(View { s: &name })
    let mut ts = Array.of(Tag { n: 0 })
    One { k: "k", vs: Array.of(View { s: &name }) }.relaySelf(&mut vs)
    viaCap2(&mut vs, &name)
    addR2(&mut vs, KRef2 { r: &name })
    RefOne { k: &name }.addK(&mut vs)
    viaHelper(&mut vs, &Array.of("x"))
    relay(&mut ts, &name)
}
`

func TestRefKeepsOrigins(t *testing.T) {
	p, body := srcProgram(t, refOriginSrc+wideSrc(40))
	var view sem.EntityID
	for e, g := range p.ByEnt {
		if g != nil && p.R.Entity(e).Name == "fromRef" && p.R.Entity(p.R.Fn(e).Owner).Name == "View" {
			view = e
		}
	}
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: body("use")}
	bound := func(slot int) fnBind { return fnBind{slot: {{ent: view}}} }
	for _, tc := range []struct {
		name string
		fn   string
		bind fnBind
		t, j int
		kind refKind
		kept bool
	}{
		{"a relayed receiver lends its place", "relaySelf", bound(2), 1, 0, refOwn, true},
		{"a captured reference is the closure's input", "viaCap2", bound(2), 0, 1, refOwn, true},
		{"a by-value holder lends what it holds", "addR2", bound(2), 0, 1, refHeld, true},
		{"a by-value holder lends no place", "addR2", bound(2), 0, 1, refOwn, false},
		{"a field read through the receiver is what it holds", "addK", bound(2), 1, 0, refHeld, true},
		{"a field read through the receiver is not its place", "addK", bound(2), 1, 0, refOwn, false},
		{"a scalar operand of a helper is no origin", "viaHelper", bound(2), 0, 0, refOwn, false},
		{"the helper's reference operand is", "viaHelper", bound(2), 0, 1, refOwn, true},
		{"a slot past the first window", "wide", bound(42), 0, 41, refOwn, true},
		{"the target of a wide body keeps nothing of itself", "wide", bound(42), 0, 0, refOwn, false},
	} {
		s := c.calleeRefKeeps(body(tc.fn).Ent, tc.bind)
		if got := s.storesInto(tc.t, tc.j, tc.kind); got != tc.kept {
			t.Errorf("%s: %s keeps %d in %d = %v, want %v", tc.name, tc.fn, tc.j, tc.t, got, tc.kept)
		}
	}
	if fns := c.witnessSet(body("relay").Ent, 0); len(fns) != 1 || p.R.Entity(fns[0].ent).Name != "fromRef" || fns[0].ent == view {
		t.Errorf("relay is instantiated with Tag only, witnessSet = %+v", fns)
	}
}

// wideSrc is a generic over n Int parameters between its target and the reference it keeps.
func wideSrc(n int) string {
	var b strings.Builder
	b.WriteString("fn wide[K, V: FromRef[K]](dst: &mut Array[V]")
	for i := range n {
		b.WriteString(", p" + string(rune('a'+i%26)) + strings.Repeat("x", i/26) + ": Int")
	}
	b.WriteString(", k: &K) { dst.push(V.fromRef(k)) }\n")
	return b.String()
}
