package mir

import (
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

const sweepSrc = `
fn grow(a: &mut Array[Int]) { a.push(1) }
fn pick(x: &mut Array[Int]) -> &mut Array[Int] { x }
fn third(mut a: Array[String]) -> String { let c = () => a[0]; a[0] = "changed"; c() }
fn lendsFirst(mut a: Array[Int], n: Int) { grow(&mut a) }
fn lendsPicked(mut a: Array[Int], n: Int) { grow(pick(&mut a)) }
fn seeds(p: Array[Int]) { let q = p }
`

func TestCaptureShareChangesFirstParam(t *testing.T) {
	p, body := srcProgram(t, sweepSrc)
	g := body("third")
	if !slices.ContainsFunc(newReadCheck(p, g).changes(), func(e moveEvent) bool { return e.root == g.Params[0] }) {
		t.Errorf("the write a[0] = .. to the first parameter is no change a sharing closure sees")
	}
}

func TestFlowPlaceOfFirstParam(t *testing.T) {
	p, body := srcProgram(t, sweepSrc)
	g := body("lendsFirst")
	c := collected(p, g)
	if l, ok := c.flowPlace(storeFlow{target: calls(p, g, "grow")[0].Args[0], ref: true}); !ok || l != g.Params[0] {
		t.Errorf("&mut a borrows %v (%v), want the first parameter", l, ok)
	}
	g = body("lendsPicked")
	c = collected(p, g)
	if l, ok := c.flowPlace(storeFlow{target: calls(p, g, "grow")[0].Args[0], ref: true}); ok {
		t.Errorf("a reference a call returns borrows %v, want none known", l)
	}
}

func TestReplayFlowSeedsOnlyResults(t *testing.T) {
	p, body := srcProgram(t, sweepSrc)
	g := body("seeds")
	c := collected(p, g)
	fl := c.solveFlow(func(midir.Inst) []uint32 { return []uint32{7} }, nil)
	if fl.ever[g.Params[0]][7] {
		t.Errorf("an instruction without a result seeded the first parameter")
	}
	if !fl.ever[localNamed(t, g, "q")][7] {
		t.Errorf("q does not take its own seed")
	}
}
