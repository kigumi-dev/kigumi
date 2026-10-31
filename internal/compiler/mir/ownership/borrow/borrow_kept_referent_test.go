package borrow

import (
	"testing"
)

const keptReferentSrc = `
fn outG[H](f: fn(&mut Array[String]) -> Future[usize], c: &mut Array[String], h: &mut H, put: fn(&mut H, Future[usize]) -> Void) {
    put(h, f(c))
}
fn relayG[H](f: fn(&mut Array[String]) -> Future[usize], c: &mut Array[String], h: &mut H, put: fn(&mut H, Future[usize]) -> Void) {
    outG(f, c, h, put)
}
interface Sink {
    fn put(move self, f: Future[usize])
}
fn sinkG[H: Sink](f: fn(&mut Array[String]) -> Future[usize], c: &mut Array[String], h: H) {
    h.put(f(c))
}
interface RefSink {
    fn putRef(mut self, f: Future[usize])
}
fn boundG[S: RefSink](f: fn(&mut Array[String]) -> Future[usize], c: &mut Array[String], h: &mut S) {
    h.putRef(f(c))
}
fn dynG(f: fn(&mut Array[String]) -> Future[usize], c: &mut Array[String], h: &mut RefSink) {
    h.putRef(f(c))
}
interface Decode {
    fn parse(x: &String) -> Self
    fn put(mut self, x: Self)
}
fn parsed[H: Decode](h: &mut H, x: &String) {
    h.put(H.parse(x))
}
interface Maker {
    fn make(c: &mut Array[String]) -> Future[usize]
    fn put(mut self, f: Future[usize])
}
fn makerG[H: Maker](h: &mut H, c: &mut Array[String]) {
    h.put(H.make(c))
}
fn takeFuture(fs: &mut Array[Future[usize]]) -> Future[usize]? {
    fs.takeAt(0)
}
`

func TestKeptReferentSummaries(t *testing.T) {
	p, body := srcProgram(t, keptReferentSrc)
	for _, name := range []string{"outG", "relayG", "sinkG", "boundG", "dynG"} {
		g := body(name)
		c := &borrowCheck{p: p.input, f: g}
		if s := c.keptOf(g.Ent); !s.storesInto(2, 1) {
			t.Errorf("%s: an unknown callback does not keep c in h: %+v", name, s)
		}
	}
	g := body("dynG")
	c := &borrowCheck{p: p.input, f: g}
	if typ := g.Locals[g.Params[2]].Type; !c.heldAsyncType(typ) || !c.hidesLayout(typ) {
		t.Error("an existential view hides its holder's layout")
	}
	g = body("parsed")
	c = &borrowCheck{p: p.input, f: g}
	s := c.keptOf(g.Ent)
	if s.storesInto(0, 1) || s.movesInto(0, 1) {
		t.Errorf("the declared parser keeps a reference from plain contents: %+v", s)
	}
	g = body("makerG")
	if s := c.keptOf(g.Ent); !s.storesInto(0, 1) {
		t.Errorf("the unknown Future maker does not keep its reference: %+v", s)
	}
	g = body("takeFuture")
	if s := c.keptOf(g.Ent); s.keepsResult(0) || !s.movesOut(0) {
		t.Errorf("the primitive keeps its receiver instead of moving contents: %+v", s)
	}
}
