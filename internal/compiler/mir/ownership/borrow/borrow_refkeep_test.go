package borrow

import (
	"testing"

	"kigumi/internal/compiler/sem"
)

const refKeepSrc = `
interface FromRef[K] {
    fn fromRef(k: &K) -> Self
}
interface Wrap[V] {
    fn wrap(v: V) -> Self
}
type View['a] = { pub s &'a String }
type Tag['a] = { pub v View['a]?, pub n usize }
type Boxed[V] = { pub v V }
fn View['a].fromRef(k: &'a String) -> View['a] { View { s: k } }
fn Tag['a].fromRef(k: &String) -> Tag['a] { Tag { v: None, n: k.len() } }
fn Boxed[V].wrap(v: V) -> Boxed[V] { Boxed { v: v } }
fn addTo[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { dst.push(V.fromRef(k)) }
fn relay[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { addTo(dst, k) }
type Nest[K, V, W] = { pub mut ws Array[W], pub vs Array[V], pub ks Array[K] }
fn Nest[K, V: FromRef[K], W: Wrap[V]].add(mut self, k: &K) { self.ws.push(W.wrap(V.fromRef(k))) }
fn ping[K, V: FromRef[K]](dst: &mut Array[V], k: &K, n: Int) {
    if n > 0 { pong(dst, k, n - 1) }
}
fn pong[K, V: FromRef[K]](dst: &mut Array[V], k: &K, n: Int) {
    if n > 0 { ping(dst, k, n - 1) } else { dst.push(V.fromRef(k)) }
}
interface Mint[K] {
    fn mint(k: &K) -> Self
}
fn mintTo[K, V: Mint[K]](dst: &mut Array[V], k: &K) { dst.push(V.mint(k)) }
fn copyOut[K: Copy](dst: &mut Array[K], k: &K) { dst.push(*k) }
fn use() {
    let name = "n"
    let mut vs = Array.of(View { s: &name })
    let mut ts = Array.of(Tag { v: None, n: 0 })
    let mut nb = Nest { ws: Array.of(Boxed { v: View { s: &name } }), vs: Array.of(View { s: &name }), ks: Array.of("k") }
    addTo(&mut vs, &name)
    addTo(&mut ts, &name)
    relay(&mut vs, &name)
    nb.add(&name)
    ping(&mut vs, &name, 1)
    ping(&mut ts, &name, 1)
}
`

func TestRefKeepsThroughWitnesses(t *testing.T) {
	p, body := srcProgram(t, refKeepSrc)
	method := func(owner string) sem.EntityID {
		for e, g := range p.ByEnt {
			if g != nil && p.R.Entity(e).Name == "fromRef" && p.R.Entity(p.R.Fn(e).Owner).Name == owner {
				return e
			}
		}
		t.Fatalf("no %s.fromRef", owner)
		return 0
	}
	view, tag := method("View"), method("Tag")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: body("use")}
	if !c.declaredRefKeeps(view).keepsResult(0, refOwn) || c.declaredRefKeeps(tag).keepsResult(0, refOwn) {
		t.Errorf("declared keeps: View %v, Tag %v", c.declaredRefKeeps(view), c.declaredRefKeeps(tag))
	}
	var boxedWrap sem.EntityID
	for e := range p.ByEnt {
		if p.R.Entity(e).Name == "wrap" && p.R.Fn(e).Owner != 0 && p.R.Entity(p.R.Fn(e).Owner).Name == "Boxed" {
			boxedWrap = e
		}
	}
	for _, tc := range []struct {
		fn   string
		bind map[int][]sem.EntityID
		kept bool
	}{
		{"addTo", map[int][]sem.EntityID{2: {view}}, true},
		{"addTo", map[int][]sem.EntityID{2: {tag}}, false},
		{"addTo", nil, true},
		{"relay", map[int][]sem.EntityID{2: {view}}, true},
		{"relay", map[int][]sem.EntityID{2: {tag}}, false},
		{"add", map[int][]sem.EntityID{2: {view}, 3: {boxedWrap}}, true},
		{"add", map[int][]sem.EntityID{2: {tag}, 3: {boxedWrap}}, false},
		{"ping", map[int][]sem.EntityID{3: {view}}, true},
		{"ping", map[int][]sem.EntityID{3: {tag}}, false},
		{"pong", map[int][]sem.EntityID{3: {tag}}, false},
		{"mintTo", nil, false},
		{"copyOut", nil, false},
	} {
		bind := fnBind{}
		for j, ents := range tc.bind {
			for _, e := range ents {
				bind[j] = append(bind[j], boundFn{ent: e})
			}
		}
		s := c.refKeeps(body(tc.fn).Ent, bind)
		if s == nil || s.storesInto(0, 1, refOwn) != tc.kept {
			t.Errorf("%s %v: keeps k in dst = %+v, want %v", tc.fn, tc.bind, s, tc.kept)
		}
	}
}
