package borrow

import (
	"strings"
	"testing"
)

const genericCallbackSource = `
type Sub[M] = | Resize(tag: pure fn(Int, Int) -> M)
type Sourced[A, M] = { subs Sub[M], model A }
type App[A, M] = { subs pure fn(A) -> Sourced[A, M], update pure fn(A, M) -> A }
fn flatten[M](s: &Sub[M]) -> Array[Sub[M]] {
    match s { Resize(tag) -> Array.of(Sub.Resize(tag)) }
}
fn advance[A, M](app: App[A, M], model: A, msg: M) -> A { (app.update)(model, msg) }
fn fire[A, M](app: App[A, M], model: A) -> A {
    let Sourced { subs: raw, model: held } = (app.subs)(model)
    let subs = flatten(&raw)
    let mut acc = held
    for s in subs {
        if s is Sub.Resize(tag) { acc = advance(app, acc, tag(1, 2)) }
    }
    acc
}
let app = App {
    subs: (x: Int) => Sourced { subs: Sub.Resize((w: Int, h: Int) => w + h), model: x }
    update: (x: Int, m: Int) => x + m
}
print "${fire(app, 4)}"
`

func TestGenericCallbackSubscription(t *testing.T) {
	for _, variant := range []string{"helper", "direct", "copied"} {
		t.Run(variant, func(t *testing.T) {
			src := genericCallbackSource
			switch variant {
			case "direct":
				src = strings.ReplaceAll(src, "advance(app, acc, tag(1, 2))", "(app.update)(acc, tag(1, 2))")
			case "copied":
				src = strings.ReplaceAll(src, "let Sourced {", "let subscribe = app.subs\n    let Sourced {")
				src = strings.ReplaceAll(src, "(app.subs)(model)", "subscribe(model)")
			}
			p, _ := srcProgram(t, src)
			if p.HasErrors() {
				t.Fatal(p.Render())
			}
		})
	}
}

func TestGenericCallbackOwnedDrops(t *testing.T) {
	for _, tc := range []struct{ name, declaration, value string }{
		{"record", "type Wrapped[T] = { pub value T, pub tag fn(T) -> Int }", "Wrapped { value: p, tag: (x: Pusher['a]) => 1 }"},
		{"adt", "type Wrapped[T] = Item(value: T, tag: fn(T) -> Int)", "Wrapped.Item(p, (x: Pusher['a]) => 1)"},
		{"array", "", "Array.of(p)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := `type V['a] = { pub s &'a String }
type Pusher['a] = resource { pub out &'a mut Array[V['a]], pub v V['a] }
fn Pusher['a].drop(move self) { self.out.push(self.v) }
` + tc.declaration + `
fn discard[T](x: T) {}
fn f['a](out: &'a mut Array[V['a]]) {
    let local = "local-${1}"
    let p = Pusher { out: out, v: V { s: &local } }
    discard(` + tc.value + `)
}
`
			p, _ := srcProgram(t, src)
			if !strings.Contains(p.Render(), "a borrow cannot be stored into `out` here") {
				t.Fatalf("destructor escaped a local borrow:\n%s", p.Render())
			}
		})
	}
}
