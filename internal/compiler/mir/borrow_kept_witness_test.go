package mir

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

func TestKeptWitnessSummaries(t *testing.T) {
	p, body := srcProgram(t, `
interface Sink {
    fn put(mut self, f: Future[usize])
}
interface Consumer {
    fn consume(mut self, f: Future[usize])
}
type Hold['a] = { pub r &'a mut Array[Future[usize]] }
type Quiet = { pub n Int }
fn Hold['a].put(mut self, f: Future[usize]) { self.r.push(f) }
fn Quiet.put(mut self, f: Future[usize]) { let _ = f }
fn Quiet.consume(mut self, f: Future[usize]) { let _ = f }
fn seal['a](h: &mut Hold['a], q: &mut Quiet) {
    let v: &mut Sink = h
    let w: &mut Sink = q
    let x: &mut Consumer = q
}
fn store(f: fn(&mut Int) -> Future[usize], c: &mut Int, h: &mut Sink) {
    h.put(f(c))
}
fn consume(f: fn(&mut Int) -> Future[usize], c: &mut Int, h: &mut Consumer) {
    h.consume(f(c))
}
`)
	req := p.R.Requirement(p.R.PackageMember(p.R.PackageByPath("main"), "Sink"), "put")
	if ws, ok := p.R.SealedWitnesses(req); !ok || len(ws) != 2 {
		t.Fatalf("sealed witnesses = %v, %v; want two implementations", ws, ok)
	}
	for _, tc := range []struct {
		name string
		kept bool
	}{{"store", true}, {"consume", false}} {
		g := body(tc.name)
		c := &borrowCheck{p: p, f: g}
		if s := c.keptOf(g.Ent); s.storesInto(2, 1) != tc.kept {
			t.Errorf("%s: stores c in h = %v, want %v", tc.name, s.storesInto(2, 1), tc.kept)
		}
	}
	if n := p.R.ConformMissesAfterSeal(); n != 0 {
		t.Errorf("late conformance queries = %d", n)
	}
}

func TestKeptUnknownWitnessFlow(t *testing.T) {
	p, body := srcProgram(t, `
interface Sink {
    fn put(mut self, f: Future[usize])
}
fn out(h: &mut Sink, f: Future[usize]) { h.put(f) }
`)
	g := body("out")
	c := collected(p, g)
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if (in.Op != midir.OpCall && in.Op != midir.OpCallValue) || !c.ifaceRequirement(in.Ent) {
				continue
			}
			fls := c.keptFlows(in)
			if len(fls) != 1 || len(fls[0].values) != 1 || !fls[0].opaque {
				t.Fatalf("unknown witness flows = %+v", fls)
			}
			if root, _, caller, ok := c.storeRoot(fls[0]); !ok || !caller || root != g.Params[0] {
				t.Fatalf("unknown witness destination = %d, %v, %v", root, caller, ok)
			}
			return
		}
	}
	t.Fatal("no requirement call")
}
