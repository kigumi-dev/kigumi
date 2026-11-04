package borrow

import (
	"testing"

	"kigumi/internal/compiler/sem"
)

const refKeepDefaultSrc = `
interface FromRef[K] {
    fn fromRef(k: &K) -> Self
}
type View['a] = { pub s &'a String }
type Tag['a] = { pub v View['a]?, pub n usize }
type Pair[V] = { pub v V, pub n usize }
fn View['a].fromRef(k: &'a String) -> View['a] { View { s: k } }
fn Tag['a].fromRef(k: &String) -> Tag['a] { Tag { v: None, n: k.len() } }
fn Pair[V: FromRef[String]].fromRef(k: &String) -> Pair[V] { Pair { v: V.fromRef(k), n: k.len() } }
type Self2[K, V] = { pub mut ks Array[K], pub mut vs Array[V] }
fn Self2[K, V: FromRef[K]].selfLoop(mut self) { self.vs.push(V.fromRef(&self.ks[0])) }
fn firstOf[K](ks: &Array[K]) -> &K { &ks[0] }
fn addFirst[K, V: FromRef[K]](dst: &mut Array[V], ks: &Array[K]) { dst.push(V.fromRef(firstOf(ks))) }
fn apply[K, V, F: Fn[fn(&K) -> V]](dst: &mut Array[V], k: &K, f: F) { dst.push(f(k)) }
fn viaApply[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { apply(dst, k, (x: &K) => V.fromRef(x)) }
fn addTo[K, V: FromRef[K]](dst: &mut Array[V], k: &K) { dst.push(V.fromRef(k)) }
fn mkArr[K, V: FromRef[K]](k: &K) -> Array[V] { Array.of(V.fromRef(k)) }
async fn addLater[K, V: FromRef[K]](dst: &mut Array[V], k: &K) -> usize {
    dst.push(V.fromRef(k))
    dst.len()
}
fn use() {
    let name = "n"
    let mut vs = Array.of(View { s: &name })
    let mut ps = Array.of(Pair { v: Tag { v: None, n: 0 }, n: 0 })
    let mut b = Self2 { ks: Array.of("k"), vs: Array.of(View { s: &name }) }
    b.selfLoop()
    addFirst(&mut vs, &b.ks)
    viaApply(&mut vs, &name)
    addTo(&mut vs, &name)
    addTo(&mut ps, &name)
    for v in mkArr(&name) {
        vs.push(v)
    }
    let f = addLater(&mut vs, &name)
}
`

func TestRefKeepsConservativeDefaults(t *testing.T) {
	p, body := srcProgram(t, refKeepDefaultSrc)
	method := func(name, owner string) sem.EntityID {
		for e, g := range p.ByEnt {
			if g != nil && p.R.Entity(e).Name == name && p.R.Fn(e).Owner != 0 && p.R.Entity(p.R.Fn(e).Owner).Name == owner {
				return e
			}
		}
		t.Fatalf("no %s.%s", owner, name)
		return 0
	}
	view, tag, pair := method("fromRef", "View"), method("fromRef", "Tag"), method("fromRef", "Pair")
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: body("use")}
	bound := func(slot int, f boundFn) fnBind { return fnBind{slot: {f}} }
	pairOfTag := boundFn{ent: pair, sub: fnBind{1: {{ent: tag}}}}
	pairOfView := boundFn{ent: pair, sub: fnBind{1: {{ent: view}}}}
	for _, tc := range []struct {
		name   string
		fn     string
		bind   fnBind
		t, j   int
		result bool
		kept   bool
	}{
		{"self-store through mut self", "selfLoop", bound(1, boundFn{ent: view}), 0, 0, false, true},
		{"origin through a helper call", "addFirst", bound(2, boundFn{ent: view}), 0, 1, false, true},
		{"closure capturing the witness", "viaApply", bound(2, boundFn{ent: view}), 0, 1, false, true},
		{"closure capturing a quiet witness", "viaApply", bound(2, boundFn{ent: tag}), 0, 1, false, false},
		{"generic witness bound to a quiet one", "addTo", bound(2, pairOfTag), 0, 1, false, false},
		{"generic witness bound to a keeping one", "addTo", bound(2, pairOfView), 0, 1, false, true},
		{"wrapped result", "mkArr", bound(1, boundFn{ent: view}), 0, 0, true, true},
		{"async body", "addLater", bound(2, boundFn{ent: view}), 0, 1, false, true},
	} {
		s := c.calleeRefKeeps(body(tc.fn).Ent, tc.bind)
		got := s.storesInto(tc.t, tc.j, refOwn)
		if tc.result {
			got = s.keepsResult(tc.j, refOwn)
		}
		if got != tc.kept {
			t.Errorf("%s: %s keeps = %v (%+v), want %v", tc.name, tc.fn, got, s, tc.kept)
		}
	}
	if s := c.valueKeeps(c.p.R.Entity(c.p.R.Fn(body("apply").Ent).Params[2]).Type); !s.keepsResult(0, refOwn) {
		t.Errorf("an unbound callable returning a type parameter keeps nothing: %+v", s)
	}
}
