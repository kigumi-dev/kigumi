package mir

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

const firstParamSrc = `
import task from std/task
fn startIn[F: FnOnce[fn() -> Int]](pool: task.BlockingPool, out: &mut Array[Future[Int]], w: escaping F) { out.push(pool.run(w).unwrap()) }
async fn lendsPlain(g: Future[Int], pool: task.BlockingPool) -> Int {
    let mut other = "o"
    let mut fs: Array[Future[Int]] = Array.empty()
    startIn(pool, &mut fs, () => { other = "x"; 1 })
    0
}
async fn reassigned(mut h: Future[Int]?, pool: task.BlockingPool) -> Int {
    let mut other = "o"
    h = Some(pool.run(() => { other = "x"; 1 }).unwrap())
    match h {
        Some(f) -> await f,
        None -> 0
    }
}
async fn awaitsParam(mut x: Future[Int], o: Future[Int]?, b: Bool) -> Int {
    match o {
        Some(f) -> { if b { x = f } },
        None -> {}
    }
    await x
}
`

func collected(p *Program, g *Func) *borrowCheck {
	c := &borrowCheck{p: p, f: g, holders: map[LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[LocalID]bool{}}
	c.collect()
	c.offloadNode = map[sem.EntityID]syntax.NodeID{}
	return c
}

func firstOp(g *Func, op Opcode) Inst {
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				return in
			}
		}
	}
	return Inst{}
}

func TestLentPlaceWithoutHolderLeavesFirstParam(t *testing.T) {
	p, body := srcProgram(t, firstParamSrc)
	g := body("lendsPlain")
	c := collected(p, g)
	in := calls(p, g, "startIn")[0]
	fs := localNamed(t, g, "fs")
	stores, dsts := c.storePlaces(in.Args[1]), c.offloadDsts(in, flowSets{})
	if !slices.Contains(stores, fs) || !slices.Contains(dsts, fs) || slices.Contains(stores, g.Params[0]) || slices.Contains(dsts, g.Params[0]) {
		t.Errorf("store places %v, offload places %v: want fs and not the first parameter, which no holder lends", stores, dsts)
	}
}

func TestOffloadFlowReachesFirstParam(t *testing.T) {
	p, body := srcProgram(t, firstParamSrc)
	g := body("reassigned")
	c := collected(p, g)
	if !handsOff(c.solveFlow(c.offloadSeed, c.offloadStep).ever[g.Params[0]]) {
		t.Errorf("h, the first parameter, does not hold the handle stored in it")
	}
	if o, ok := c.awaitedPayloadOf(firstOp(g, OpAwait).Args[0], map[LocalID]bool{g.Params[0]: true}); !ok || o != g.Params[0] {
		t.Errorf("awaiting h's payload finishes %v (%v), want h", o, ok)
	}
}

func TestAwaitedParamKeepsItsCallersValue(t *testing.T) {
	p, body := srcProgram(t, firstParamSrc)
	g := body("awaitsParam")
	c := collected(p, g)
	if o, ok := c.awaitedPayloadOf(firstOp(g, OpAwait).Args[0], map[LocalID]bool{localNamed(t, g, "o"): true}); ok {
		t.Errorf("awaiting x finishes %v, want none: x may still be the caller's", o)
	}
}

func TestInFlightTouchOfFirstParamNeedsAResult(t *testing.T) {
	p, body := srcProgram(t, firstParamSrc)
	g := body("reassigned")
	c := collected(p, g)
	if c.touchesInFlight(Inst{Op: OpRelease, Args: []LocalID{g.Params[0]}}, ownerRef{owner: g.Params[0]}) {
		t.Errorf("releasing the first parameter counts as writing it, though a release defines no local")
	}
}
