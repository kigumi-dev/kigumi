package mir

import "testing"

const relaySrc = `
type V['a] = { pub s &'a String }
type Sink['a] = { pub items &'a mut Array[V['a]] }
fn apply['a](g: fn(V['a]) -> Void, x: V['a]) { g(x) }
fn applyIn['a](g: fn(V['a]) -> Void, x: V['a]) { g(x) }
fn applySink['a](g: fn(Sink['a]) -> Void, x: Sink['a]) { g(x) }
fn applyFuture(g: fn(Future[Int]) -> Void, x: Future[Int]) { g(x) }
fn store['a](r: &mut Array[V['a]], x: V['a]) {
    apply((y: V['a]) => { r.push(y) }, x)
}
fn nested['a](r: &mut Array[V['a]], x: V['a]) {
    apply((y: V['a]) => { applyIn((z: V['a]) => { r.push(z) }, y) }, x)
}
fn read['a](x: V['a]) {
    apply((y: V['a]) => { applyIn((z: V['a]) => { _ = z.s }, y) }, x)
}
fn moveSink['a](r: &mut Array[Sink['a]], x: Sink['a]) {
    applySink((y: Sink['a]) => { applySink((z: Sink['a]) => { r.push(z) }, y) }, x)
}
fn moveFuture(r: &mut Array[Future[Int]], x: Future[Int]) {
    applyFuture((y: Future[Int]) => { applyFuture((z: Future[Int]) => { r.push(z) }, y) }, x)
}
fn forwarded['a](g: fn(V['a]) -> Void, x: V['a]) {
    apply((y: V['a]) => { applyIn((z: V['a]) => { g(z) }, y) }, x)
}
`

func TestParamRelaySummaryKinds(t *testing.T) {
	p, body := srcProgram(t, relaySrc)
	for _, name := range []string{"store", "nested", "read", "moveSink", "moveFuture", "forwarded"} {
		t.Run(name, func(t *testing.T) {
			g := body(name)
			c := &borrowCheck{p: p, f: g}
			i := len(g.Params) - 1
			want := name != "read"
			if got := c.paramOut(g, i); got != want {
				t.Errorf("paramOut = %v, want %v", got, want)
			}
			if got, want := c.paramMoves(g, i), name == "moveSink" || name == "moveFuture"; got != want {
				t.Errorf("paramMoves = %v, want %v", got, want)
			}
			if got, want := c.paramStores(g, i), name == "moveSink"; got != want {
				t.Errorf("paramStores = %v, want %v", got, want)
			}
			if name == "forwarded" {
				if sites := c.paramSites(paramKey{g: g, i: i, kind: siteOut}); len(sites) != 1 || sites[0].callee != (relayTarget{kind: relayParam, param: 0}) {
					t.Errorf("forwarded sites = %v, want a relay to the first parameter", sites)
				}
			}
		})
	}
}
