package sem_test

import (
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func checkOp(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	r := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
	if r.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", r.Render())
	}
	return r
}

func findFn(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entities[id]; e.Kind == sem.EntFn && e.Name == name {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("function %q not found", name)
	return 0
}

func TestOpSummaryFormula(t *testing.T) {
	r := checkOp(t, `
type Box = { n String }

fn callIt(f: fn() -> Void) {
    f()
}

fn callDead(f: fn() -> Void) {}

noalloc fn quiet() {}

fn allocating() {
    let b = Box { n: "x" }
}

fn useQuiet() {
    callIt(quiet)
}

fn useAllocating() {
    callIt(allocating)
}

fn useDeadAllocating() {
    callDead(allocating)
}

let n = 1
`)
	callIt := findFn(t, r, "callIt")
	if s := r.Fn(callIt).Ops; s.Facts[sem.FactAlloc].Maybe || !s.Unresolved {
		t.Fatalf("callIt's own summary must stay clean and Unresolved, got %+v", s)
	}
	if got := r.Fn(findFn(t, r, "useQuiet")).Ops.Facts[sem.FactAlloc].Maybe; got {
		t.Fatal("useQuiet must not inherit an allocation: its callback never allocates")
	}
	allocCause := r.Fn(findFn(t, r, "useAllocating")).Ops.Facts[sem.FactAlloc]
	if !allocCause.Maybe {
		t.Fatal("useAllocating must inherit allocating's allocation through the resolved callback")
	}
	if allocCause.Cause.Edge.Kind != sem.EdgeCallback {
		t.Fatalf("the recorded first cause at the call site must be the callback edge itself, got %v", allocCause.Cause.Edge.Kind)
	}
	if got := r.Fn(findFn(t, r, "useDeadAllocating")).Ops.Facts[sem.FactAlloc].Maybe; got {
		t.Fatal("a callback parameter the callee never calls must not propagate its argument's facts")
	}
}

func TestOpSummaryFixpointOrderIndependent(t *testing.T) {
	before := checkOp(t, `
type Box = { n String }

fn leaf() {
    let b = Box { n: "x" }
}

fn caller() {
    leaf()
}

let n = 1
`)
	after := checkOp(t, `
type Box = { n String }

fn caller() {
    leaf()
}

fn leaf() {
    let b = Box { n: "x" }
}

let n = 1
`)
	for _, r := range []*sem.Result{before, after} {
		got := r.Fn(findFn(t, r, "caller")).Ops.Facts[sem.FactAlloc]
		if !got.Maybe || got.Cause.Edge.Kind != sem.EdgeCall {
			t.Fatalf("caller must transitively see leaf's allocation via its call edge regardless of declaration order, got %+v", got)
		}
	}

	mutual := checkOp(t, `
type Box = { n String }

fn mutualA(n: Int) {
    if n > 0 {
        mutualB(n - 1)
    }
}

fn mutualB(n: Int) {
    if n > 0 {
        mutualA(n - 1)
    } else {
        let b = Box { n: "x" }
    }
}

let n = 1
`)
	for _, name := range []string{"mutualA", "mutualB"} {
		if !mutual.Fn(findFn(t, mutual, name)).Ops.Facts[sem.FactAlloc].Maybe {
			t.Fatalf("%s must see the allocation reachable through mutual recursion", name)
		}
	}
}

func TestOpSummaryBlockCauseOrderIndependent(t *testing.T) {
	offloadedFirst := checkOp(t, `
import fs from std/fs
import task from std/task

async fn mixed() -> String! {
    let pool = task.BlockingPool.new(4)
    let fut = pool.run(() => 1).unwrap()
    _ = await fut
    host.files().read(fs.Path.fromString("x")?)?
}

let n = 1
`)
	directFirst := checkOp(t, `
import fs from std/fs
import task from std/task

async fn mixed() -> String! {
    _ = host.files().read(fs.Path.fromString("x")?)?
    let pool = task.BlockingPool.new(4)
    let fut = pool.run(() => 1).unwrap()
    _ = await fut
    "ok"
}

let n = 1
`)
	for name, r := range map[string]*sem.Result{"pool.run reached first": offloadedFirst, "direct read reached first": directFirst} {
		fb := r.Fn(findFn(t, r, "mixed")).Ops.Facts[sem.FactBlock]
		if !fb.Maybe || fb.Offloaded || fb.Cause.Unknown {
			t.Fatalf("%s: a real direct-blocking call must win over an offloaded one regardless of edge order, got %+v", name, fb)
		}
	}
}

func TestOpSummaryBlockOffloadedThroughPoolClosure(t *testing.T) {
	r := checkOp(t, `
import fs from std/fs
import task from std/task

async fn useProvider() -> Result[String, task.BlockingError] {
    let pool = task.BlockingPool.new(4)
    let fut = pool.run(() => {
        host.files().read(fs.Path.fromString("x").unwrap()).unwrap()
    })?
    await fut
}

let n = 1
`)
	fb := r.Fn(findFn(t, r, "useProvider")).Ops.Facts[sem.FactBlock]
	if fb.Maybe && !fb.Offloaded {
		t.Fatalf("the closure's direct-blocking read leaked into useProvider's own FactBlock instead of merging as Offloaded, got %+v", fb)
	}
}

func TestOpSummaryProvenance(t *testing.T) {
	r := checkOp(t, `
type Box = { n String }

fn c() {
    let b = Box { n: "x" }
}

fn b() {
    c()
}

fn a() {
    b()
}

let n = 1
`)
	chain := r.ExplainOpFact(findFn(t, r, "a"), sem.FactAlloc)
	if !chain.Maybe || len(chain.Chain) != 3 {
		t.Fatalf("expected a three-hop chain (a->b, b->c, c's own allocation site), got %+v", chain)
	}
	if want := "calls `b`"; chain.Chain[0].Text != want {
		t.Fatalf("first hop = %q, want %q", chain.Chain[0].Text, want)
	}
	if want := "calls `c`"; chain.Chain[1].Text != want {
		t.Fatalf("second hop = %q, want %q", chain.Chain[1].Text, want)
	}
	if want := "may allocate here"; chain.Chain[2].Text != want {
		t.Fatalf("third hop = %q, want %q", chain.Chain[2].Text, want)
	}
}

// TestOpSummaryReleaseAndSuspend checks the two facts with no `noalloc`-era
// equivalent: a Shared drop releases, an `await` point suspends.
func TestOpSummaryReleaseAndSuspend(t *testing.T) {
	r := checkOp(t, `
import {Shared} from std/alloc

fn holdsShared() {
    let s = Shared.new(1)
}

async fn fetch() -> Int {
    1
}

async fn twice() -> Int {
    let v = await fetch()
    v * 2
}

let n = 1
`)
	if !r.Fn(findFn(t, r, "holdsShared")).Ops.Facts[sem.FactRelease].Maybe {
		t.Fatal("dropping a Shared cell must set FactRelease")
	}
	if !r.Fn(findFn(t, r, "twice")).Ops.Facts[sem.FactSuspend].Maybe {
		t.Fatal("a function with an `await` point must set FactSuspend")
	}
}

// TestOpSummaryArtifactDeterministic stands in for cross-package determinism
// until an artifact cache exists: two checks of the same source must agree.
func TestOpSummaryArtifactDeterministic(t *testing.T) {
	src := `
type Box = { n String }

pub fn allocating() {
    let b = Box { n: "x" }
}

pub fn quiet() {}

let n = 1
`
	a, b := checkOp(t, src), checkOp(t, src)
	pa, pb := a.PublicOpSummaries(), b.PublicOpSummaries()
	if len(pa) != len(pb) || len(pa) == 0 {
		t.Fatalf("expected a matching, non-empty public surface, got %d vs %d", len(pa), len(pb))
	}
	for k, va := range pa {
		vb, ok := pb[k]
		if !ok {
			t.Fatalf("%s missing from the second check's artifact", k)
		}
		if va.HasOpFact("allocate") != vb.HasOpFact("allocate") || va.Unresolved != vb.Unresolved {
			t.Fatalf("%s disagrees between two checks of the same source: %+v vs %+v", k, va, vb)
		}
	}
	allocName := a.Packages[a.Entities[findFn(t, a, "allocating")].Pkg].Path + ".allocating"
	if !pa[allocName].HasOpFact("allocate") {
		t.Fatalf("%s's own artifact entry must report FactAlloc", allocName)
	}
	quietName := a.Packages[a.Entities[findFn(t, a, "quiet")].Pkg].Path + ".quiet"
	if pa[quietName].HasOpFact("allocate") {
		t.Fatalf("%s must not report FactAlloc", quietName)
	}
}
