package borrow

import (
	"slices"
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
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

// unboundedPoolProgram strips BlockingPool.run's Send bounds so sem lets a cell-writing closure reach the in-flight check.
func unboundedPoolProgram(t *testing.T, src string) (*fixture, func(string) *midir.Func) {
	t.Helper()
	std := semfixture.LoadStd(t, "../../../../../std")
	for _, pkg := range std {
		for i, tree := range pkg.Files {
			text := strings.Replace(string(tree.File.Src), "run[T: Send, F: FnOnce[fn() -> T] + Send]", "run[T, F: FnOnce[fn() -> T]]", 1)
			if text != string(tree.File.Src) {
				pkg.Files[i] = syntax.Parse(token.NewFile(tree.File.Name, []byte(text)))
				return stdProgram(t, std, src)
			}
		}
	}
	t.Fatal("std has no Send-bounded BlockingPool.run")
	return nil, nil
}

func collected(p *fixture, g *midir.Func) *borrowCheck {
	c := &borrowCheck{p: p.input, state: New(p.input, nil), f: g, holders: map[midir.LocalID]*borrowInfo{}, reported: map[reportKey]int{}, keptTemp: map[midir.LocalID]bool{}}
	c.collect()
	c.offloadNode = map[sem.EntityID]syntax.NodeID{}
	return c
}

func firstOp(g *midir.Func, op midir.Opcode) midir.Inst {
	for _, blk := range g.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				return in
			}
		}
	}
	return midir.Inst{}
}

func TestLentPlaceWithoutHolderLeavesFirstParam(t *testing.T) {
	p, body := unboundedPoolProgram(t, firstParamSrc)
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
	p, body := unboundedPoolProgram(t, firstParamSrc)
	g := body("reassigned")
	c := collected(p, g)
	if !handsOff(c.solveFlow(c.offloadSeed, c.offloadStep).ever[g.Params[0]]) {
		t.Errorf("h, the first parameter, does not hold the handle stored in it")
	}
	if o, ok := c.awaitedPayloadOf(firstOp(g, midir.OpAwait).Args[0], map[midir.LocalID]bool{g.Params[0]: true}); !ok || o != g.Params[0] {
		t.Errorf("awaiting h's payload finishes %v (%v), want h", o, ok)
	}
}

func TestAwaitedParamKeepsItsCallersValue(t *testing.T) {
	p, body := unboundedPoolProgram(t, firstParamSrc)
	g := body("awaitsParam")
	c := collected(p, g)
	if o, ok := c.awaitedPayloadOf(firstOp(g, midir.OpAwait).Args[0], map[midir.LocalID]bool{localNamed(t, g, "o"): true}); ok {
		t.Errorf("awaiting x finishes %v, want none: x may still be the caller's", o)
	}
}

func TestInFlightTouchOfFirstParamNeedsAResult(t *testing.T) {
	p, body := unboundedPoolProgram(t, firstParamSrc)
	g := body("reassigned")
	c := collected(p, g)
	if c.touchesInFlight(midir.Inst{Op: midir.OpRelease, Args: []midir.LocalID{g.Params[0]}}, ownerRef{owner: g.Params[0]}) {
		t.Errorf("releasing the first parameter counts as writing it, though a release defines no local")
	}
}
