package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func checkIsr(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../std"), main), IsrContracts: mir.IsrContracts}, schema.GenerateDerive)
}

// checkIsrWithSource is checkIsr plus a bodiless Std package whose functions carry an inert contract, isolating a drop-only path.
func checkIsrWithSource(t *testing.T, source, src string) *sem.Result {
	t.Helper()
	return checkIsrWith(t, source, src, isrTestContracts{inert: true}, sem.InterruptConfig{})
}

// isrTestContracts trusts buflib's bodiless functions: inert when inert is set, else by the contracts map.
type isrTestContracts struct {
	inert     bool
	contracts map[string]sem.IsrContract
}

func (c isrTestContracts) IsrContract(key string) (sem.IsrContract, bool) {
	if !strings.HasPrefix(key, "buflib.") {
		return mir.IsrContracts.IsrContract(key)
	}
	if got, ok := c.contracts[key]; ok {
		return got, true
	}
	return sem.IsrContract{Kind: sem.IsrInert}, c.inert
}

func (c isrTestContracts) IsrKnown(id string) bool {
	for _, got := range c.contracts {
		if got.ID == id {
			return true
		}
	}
	return mir.IsrContracts.IsrKnown(id)
}

func (isrTestContracts) IsrVersion() string { return "test" }

func checkIsrWith(t *testing.T, source, src string, contracts sem.IsrContracts, cfg sem.InterruptConfig) *sem.Result {
	t.Helper()
	lib := &sem.Package{Path: "buflib", Std: true}
	lib.Entry = syntax.Parse(token.NewFile("buflib/buflib.kg", []byte(source)))
	lib.Files = []*syntax.Tree{lib.Entry}
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../std"), lib, main), IsrContracts: contracts, Interrupt: cfg}, schema.GenerateDerive)
}

func TestInterruptHandlerOK(t *testing.T) {
	ok := checkIsr(t, `
noalloc fn poke() {}

export(interrupt) {
    pub fn tick() {
        poke()
    }
}
`)
	if ok.HasErrors() {
		t.Fatalf("a noalloc, panic-free handler must pass:\n%s", ok.Render())
	}
}

func TestInterruptHandlerRecordConstruct(t *testing.T) {
	res := checkIsr(t, `
type Pair = { pub x String, pub y Int }

export(interrupt) {
    pub fn tick() {
        let p = Pair { x: "a", y: 2 }
        _ = p
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "allocates or deallocates here") {
		t.Fatalf("a handler constructing a record must be rejected:\n%s", out)
	}
}

// A record of register-wide fields never reaches the allocator, so a
// handler may build one.
func TestInterruptHandlerDirectRecordConstruct(t *testing.T) {
	res := checkIsr(t, `
type Pair = { pub x Int, pub y Int }

export(interrupt) {
    pub fn tick() {
        let p = Pair { x: 1, y: 2 }
        _ = p.y
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler building a direct record must pass:\n%s", res.Render())
	}
}

func TestInterruptHandlerVariantConstruct(t *testing.T) {
	res := checkIsr(t, `
type Pair = A(x: String) | B(y: Int)

fn level() -> Int { 3 }

export(interrupt) {
    pub fn tick() {
        let p = B(level())
        _ = p
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "allocates or deallocates here") {
		t.Fatalf("a handler constructing a sum-type variant must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerDirectVariantConstruct(t *testing.T) {
	res := checkIsr(t, `
type Pair = A(x: Int) | B(y: Int)

export(interrupt) {
    pub fn tick() {
        let p = A(1)
        _ = p is A(n) && n == 1
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler building a direct variant must pass:\n%s", res.Render())
	}
}

// A payload-free variant holds nothing and is built in its descriptor, so
// a handler may construct one.
func TestInterruptHandlerNullaryVariantAccepted(t *testing.T) {
	res := checkIsr(t, `
type Mark = Cold | Hot

export(interrupt) {
    pub fn tick() {
        let x = Cold
        _ = x
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler constructing a nullary variant must be accepted:\n%s", res.Render())
	}
}

func TestInterruptHandlerOptionNoneAccepted(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let x: Int? = None
        _ = x
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler constructing `None` must be accepted:\n%s", res.Render())
	}
}

// `self = ...` writes through the receiver cell, not the value handed in, so a payload-free variant stays shareable.
func TestInterruptHandlerWholeAssignedVariantConstruct(t *testing.T) {
	res := checkIsr(t, `
type Slot = Free | Busy(id: Int)

fn Slot.release(mut self) {
    self = Free
}

export(interrupt) {
    pub fn tick() {
        let s = Free
        _ = s
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler constructing a variant some method whole-assigns must be accepted:\n%s", res.Render())
	}
}

func TestInterruptHandlerQualifiedNullaryVariantAccepted(t *testing.T) {
	res := checkIsr(t, `
type Box = Empty | Full(x: Int)

export(interrupt) {
    pub fn tick() {
        let b: Box = Box.Empty
        _ = b
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler constructing a qualified nullary variant must be accepted:\n%s", res.Render())
	}
}

func TestInterruptHandlerTupleConstruct(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let p = (1, "a")
        _ = p
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "allocates or deallocates here") {
		t.Fatalf("a handler constructing a tuple must be rejected:\n%s", out)
	}
}

// A literal's type is known only once inference settles, so the tuple is judged there.
func TestInterruptHandlerDirectTupleAccepted(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let p = (1, 2)
        _ = p._0 == p._1
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler building a tuple of scalars must pass:\n%s", res.Render())
	}
}

// A capture-free closure has an empty environment and is built in place;
// capturing anything still needs one.
func TestInterruptHandlerCapturelessClosureAccepted(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let f = () => 1
        _ = f
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler creating a captureless closure must be accepted:\n%s", res.Render())
	}
}

func TestInterruptHandlerCapturingClosureConstruct(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let n = "x"
        let f = () => n.len()
        _ = f
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "allocates or deallocates here") {
		t.Fatalf("a handler constructing a closure over a String must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerDirectClosureConstruct(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let n = 1
        let f = (x: Int) => x == n
        _ = f(2)
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("a handler building and calling a direct closure must be accepted:\n%s", res.Render())
	}
}

func TestInterruptHandlerAlloc(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let s = "${1}"
        _ = s
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "may allocate or deallocate") {
		t.Fatalf("an allocating handler must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerPanic(t *testing.T) {
	res := checkIsr(t, `
fn divideBy(d: Int) -> Int {
    10 / d
}

export(interrupt) {
    pub fn tick() {
        _ = divideBy(0)
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1081") || !strings.Contains(out, "may panic") {
		t.Fatalf("a handler reaching panic() must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "calls `divideBy`") {
		t.Fatalf("the diagnostic must show the path through `divideBy`:\n%s", out)
	}
}

func TestInterruptHandlerBlocking(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        print("tick")
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1082") {
		t.Fatalf("a handler performing I/O must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerSuspend(t *testing.T) {
	res := checkIsr(t, `
async fn fetch() -> Int {
    1
}

async fn twice() -> Int {
    await fetch()
}

export(interrupt) {
    pub fn tick() {
        let _ = twice()
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1082") || !strings.Contains(out, "may suspend here") {
		t.Fatalf("a handler reaching an await point must be rejected with a precise note:\n%s", out)
	}
	if !strings.Contains(out, "calls `twice`") {
		t.Fatalf("the diagnostic must show the path through `twice`:\n%s", out)
	}
}

func TestInterruptHandlerUnvettedExtern(t *testing.T) {
	res := checkIsr(t, `
extern(C) {
    fn raw_op()
}

export(interrupt) {
    pub fn tick() {
        unsafe {
            raw_op()
        }
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1082") || !strings.Contains(out, "no reviewed interrupt contract") {
		t.Fatalf("a handler calling an unmarked extern fn must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerDrop(t *testing.T) {
	res := checkIsr(t, `
type Buf = resource {
    pub tag String
}

fn Buf.drop(move self) {
    let s = "${self.tag}"
    _ = s
}

fn make() -> Buf {
    Buf { tag: "x" }
}

export(interrupt) {
    pub fn tick() {
        let b = make()
        _ = b
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "calls `make`") || !strings.Contains(out, "allocates or deallocates here") {
		t.Fatalf("a handler reaching a heap-owning value's construction must be rejected:\n%s", out)
	}
}

// bufLibSrc is a heap-owning Buf behind a trusted bodiless source, isolating drop checks from construction checks.
const bufLibSrc = `
pub type Buf = resource {
    pub tag String
}

fn Buf.drop(move self) {
    let s = "${self.tag}"
    _ = s
}

pub noalloc fn option() -> Buf?

pub noalloc fn buf() -> Buf

pub type Box = Empty | Full(buf: Buf)

pub noalloc fn box() -> Box

pub fn consumeResource(x: Buf?) {}

pub fn identity(x: Buf) -> Buf { x }

pub fn Buf.release(move self) {
    let s = "${self.tag}"
    _ = s
}
`

// TestInterruptHandlerDropOnlyArgs checks Option[Buf]'s drop glue recurses into the generic argument to catch the handler.
func TestInterruptHandlerDropOnlyArgs(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let x = source.option()
        _ = x
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("dropping an Option wrapping a heap-owning value must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "drops `Buf`") {
		t.Fatalf("the trace must recurse into the generic argument down to `Buf`, not stop at `Buf?`:\n%s", out)
	}
}

// TestInterruptHandlerDropOnlyVariants is TestInterruptHandlerDropOnlyArgs for a sum type's variant payload instead of a generic arg.
func TestInterruptHandlerDropOnlyVariants(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let b = source.box()
        _ = b
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("dropping a sum type whose variant owns a heap-owning value must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "drops `Buf`") {
		t.Fatalf("the trace must recurse into the variant payload down to `Buf`, not stop at `Box`:\n%s", out)
	}
}

// TestInterruptHandlerImplicitLocalDrop checks an unused `let` binding's implicit scope-exit drop is rejected too, not just an explicit discard.
func TestInterruptHandlerImplicitLocalDrop(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let x = source.option()
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler letting a heap-owning value fall out of scope unused must be rejected just like an explicit `_ = x`:\n%s", out)
	}
}

// TestInterruptHandlerImplicitParamDrop checks an unread parameter still drops at the callee's return, rejecting the handler reaching it.
func TestInterruptHandlerImplicitParamDrop(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        source.consumeResource(source.option())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler reaching a function with an unused heap-owning parameter must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "calls `consumeResource`") {
		t.Fatalf("the diagnostic must show the path through `consumeResource`:\n%s", out)
	}
}

// TestInterruptHandlerWildcardParamDrop checks a parameter named `_` is still a real by-value binding, not a no-op placeholder.
func TestInterruptHandlerWildcardParamDrop(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

fn peek(_: source.Buf) {}

export(interrupt) {
    pub fn tick() {
        peek(source.buf())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "calls `peek`") {
		t.Fatalf("a handler calling a function with a heap-owning parameter named `_` must still be rejected:\n%s", out)
	}
}

// TestInterruptHandlerLetWildcardDrop checks `let _ = ...`: a wildcard declares no entity, so the drop edge must come from the let itself.
func TestInterruptHandlerLetWildcardDrop(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let _ = source.buf()
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler discarding a heap-owning value with `let _ =` must be rejected like an explicit `_ = x`:\n%s", out)
	}
}

// TestInterruptHandlerTypeTestStillDrops checks a bare `is` tag test doesn't account for x's drop: it leaves x's value, and its drop, in place.
func TestInterruptHandlerTypeTestStillDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let x = source.option()
        if x is None {
        }
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler reading a heap-owning local with `is` before it falls out of scope must still be rejected:\n%s", out)
	}
	if !strings.Contains(out, "drops `Buf`") {
		t.Fatalf("the trace must still recurse into the generic argument down to `Buf`:\n%s", out)
	}
}

// TestInterruptHandlerIsBindingConsumes checks `is Some(v)` moves the payload into v, so x itself is not left to drop (unlike a bare tag test).
func TestInterruptHandlerIsBindingConsumes(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let x = source.option()
        if x is Some(v) {
            _ = v
        }
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler explicitly dropping the bound payload must still be rejected:\n%s", out)
	}
}

// TestInterruptHandlerPassThroughParamNotDropped checks a fn returning its own by-value param isn't blamed for dropping it.
func TestInterruptHandlerPassThroughParamNotDropped(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

fn identity(x: source.Buf) -> source.Buf { x }

export(interrupt) {
    pub fn tick() {
        _ = identity(source.buf())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("the handler must still be rejected for the real drop at its own discard:\n%s", out)
	}
	if strings.Contains(out, "calls `identity`") {
		t.Fatalf("identity's pass-through parameter must not itself be blamed for a drop:\n%s", out)
	}
}

// TestInterruptHandlerConsumedParamStillDropsInCallee checks consuming a param into a callee that drops it still rejects the handler.
func TestInterruptHandlerConsumedParamStillDropsInCallee(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

fn sink(x: source.Buf) { _ = x }

export(interrupt) {
    pub fn tick() {
        sink(source.buf())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "calls `sink`") {
		t.Fatalf("a handler passing a heap value by value into a callee that drops it must still be rejected:\n%s", out)
	}
}

// TestInterruptHandlerSelfAssignStillDrops checks `b = b` doesn't erase b's own drop: b still owns the value afterward.
func TestInterruptHandlerSelfAssignStillDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let mut b = source.buf()
        b = b
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("self-assignment must not erase the implicit drop:\n%s", out)
	}
}

// TestInterruptHandlerReassignFromHelperStillDrops checks `b = identity(b)` leaves b owning a fresh value that still needs dropping.
func TestInterruptHandlerReassignFromHelperStillDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let mut b = source.buf()
        b = source.identity(b)
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("reassigning a pass-through helper's result back into b must not erase the implicit drop:\n%s", out)
	}
}

// TestInterruptHandlerMoveSelfReceiverConsumes checks a move-self method call consumes its receiver, so the drop is blamed on the callee, not duplicated at the caller.
func TestInterruptHandlerMoveSelfReceiverConsumes(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        source.buf().release()
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "calls `release`") {
		t.Fatalf("a handler calling a move-self method that drops its receiver must be rejected through the callee:\n%s", out)
	}
	if n := strings.Count(out, "error["); n != 1 {
		t.Fatalf("the receiver's own consumption must not also report a duplicate violation at the call site (%d errors):\n%s", n, out)
	}
}

func TestInterruptHandlerRecursiveAdtDrop(t *testing.T) {
	res := checkIsr(t, `
type Node = Text(s: String) | Wrap(kid: Node)

export(interrupt) {
    pub fn tick() {
        let n = Wrap(Text("x"))
        _ = n
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("constructing a self-referential sum type must be rejected, not crash the checker:\n%s", out)
	}
}

func TestInterruptHandlerRecursiveGenericAdtDrop(t *testing.T) {
	res := checkIsr(t, `
type Boxed[M] = Leaf(v: M) | Nest(kid: Boxed[M])

export(interrupt) {
    pub fn tick() {
        let b: Boxed[Int] = Nest(Leaf(7))
        _ = b
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("constructing a self-referential generic sum type must be rejected, not crash the checker:\n%s", out)
	}
}

// TestInterruptHandlerGenericParamDrop checks an unconstrained generic param isn't assumed drop-safe just because it's a bare type parameter.
func TestInterruptHandlerGenericParamDrop(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

fn consume[T](x: T) {}

export(interrupt) {
    pub fn tick() {
        consume(source.option())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1087") || !strings.Contains(out, "keep heap-owning values out of interrupt handlers") {
		t.Fatalf("a handler reaching an unconstrained generic parameter's implicit drop must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "calls `consume`") {
		t.Fatalf("the diagnostic must show the path through `consume`:\n%s", out)
	}
}

// TestInterruptHandlerFieldReadDrops checks `x.tag` only inspects a field, not a use that discharges x's own drop.
func TestInterruptHandlerFieldReadDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let x = source.buf()
        let tag = x.tag
        _ = tag
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler that only field-reads a heap-owning local must still be rejected for its own implicit drop:\n%s", out)
	}
}

// TestInterruptHandlerParamFieldReadDrops is TestInterruptHandlerFieldReadDrops through a parameter instead of a local.
func TestInterruptHandlerParamFieldReadDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

fn peek(b: source.Buf) {
    let tag = b.tag
    _ = tag
}

export(interrupt) {
    pub fn tick() {
        peek(source.buf())
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") || !strings.Contains(out, "calls `peek`") {
		t.Fatalf("a handler calling a function that only field-reads its heap-owning parameter must still be rejected:\n%s", out)
	}
}

// TestInterruptHandlerMatchBindingFieldReadDrops is TestInterruptHandlerFieldReadDrops through a match-arm binding.
func TestInterruptHandlerMatchBindingFieldReadDrops(t *testing.T) {
	res := checkIsrWithSource(t, bufLibSrc, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        match source.box() {
            source.Box.Empty -> {}
            source.Box.Full(buf) -> {
                let tag = buf.tag
                _ = tag
            }
        }
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1080") {
		t.Fatalf("a handler that only field-reads a match-arm binding must still be rejected for its implicit drop:\n%s", out)
	}
}

// TestInterruptHandlerIfaceDrop is TestInterruptHandlerGenericParamDrop for a dynamic interface value instead.
func TestInterruptHandlerIfaceDrop(t *testing.T) {
	res := checkIsrWithSource(t, `
pub interface Greeter {
    fn hello(self)
}

pub noalloc fn greeter() -> Greeter
`, `
import source from buflib

export(interrupt) {
    pub fn tick() {
        let g = source.greeter()
        _ = g
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1087") || !strings.Contains(out, "keep heap-owning values out of interrupt handlers") {
		t.Fatalf("a handler reaching an interface value's implicit drop must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerFnValue(t *testing.T) {
	res := checkIsr(t, `
fn other() {}

export(interrupt) {
    pub fn tick() {
        let f: fn() -> Void = other
        f()
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1083") || !strings.Contains(out, "not known until run time") {
		t.Fatalf("a handler calling through a function value must be rejected:\n%s", out)
	}
}

// TestInterruptHandlerRecursionBeforePanic checks a self-call's cycle doesn't pre-empt a panic finding sitting right after it in the edge list.
func TestInterruptHandlerRecursionBeforePanic(t *testing.T) {
	res := checkIsr(t, `
noalloc fn countdown(n: Int) {
    if n <= 0 {
        return
    }
    countdown(n - 1)
}

export(interrupt) {
    pub fn tick() {
        countdown(5)
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1081") || !strings.Contains(out, "may panic") {
		t.Fatalf("the trap in `n - 1` must be reported, not the recursion it sits next to:\n%s", out)
	}
	if strings.Contains(out, "E1083") {
		t.Fatalf("the less specific recursion diagnostic must not pre-empt the panic:\n%s", out)
	}
}

func TestInterruptHandlerArgsAndReturn(t *testing.T) {
	res := checkIsr(t, `
export(interrupt) {
    pub fn tick(x: Int) -> Int {
        x
    }
}
`)
	out := res.Render()
	if !strings.Contains(out, "E1084") || !strings.Contains(out, "takes parameters") {
		t.Fatalf("a handler with parameters must be rejected:\n%s", out)
	}
	if !strings.Contains(out, "E1085") || !strings.Contains(out, "must return `Void`") {
		t.Fatalf("a non-Void handler must be rejected:\n%s", out)
	}
}

func TestInterruptHandlerSelfReceiver(t *testing.T) {
	res := checkIsr(t, `
type Board = {
    pub id Int
}

export(interrupt) {
    pub fn Board.tick(self) {}
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1084") || !strings.Contains(out, "takes parameters") {
		t.Fatalf("a method handler must be rejected like one taking arguments:\n%s", out)
	}
}

func TestInterruptHandlerNaked(t *testing.T) {
	ok := checkIsr(t, `
export(interrupt, naked) {
    pub fn isr() -> Never {
        asm {
            "ud2"
        }
    }
}
`)
	out := ok.Render()
	if !strings.Contains(out, "E1089") || !strings.Contains(out, "`ud2` cannot be classified") {
		t.Fatalf("a naked interrupt handler must not bypass the check:\n%s", out)
	}
}

func TestInterruptExtern(t *testing.T) {
	res := checkIsr(t, `
extern(interrupt) {
    fn tick()
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E1086") {
		t.Fatalf("`interrupt` on an extern block must be rejected:\n%s", out)
	}
}

// A bodiless noalloc declaration is no interrupt contract on its own; only a reviewed one makes it callable.
func TestInterruptHandlerBodilessNoallocCallee(t *testing.T) {
	const lib = `
pub noalloc fn raw_op()
`
	const src = `
import {raw_op} from buflib

export(interrupt) {
    pub fn tick() {
        raw_op()
    }
}
`
	res := checkIsrWith(t, lib, src, isrTestContracts{}, sem.InterruptConfig{})
	if out := res.Render(); !strings.Contains(out, "E1082") || !strings.Contains(out, "calls `raw_op`, which has no body the compiler can check and no reviewed interrupt contract") {
		t.Fatalf("a bodiless noalloc callee without a contract must be rejected:\n%s", out)
	}
	if res := checkIsrWithSource(t, lib, src); res.HasErrors() {
		t.Fatalf("a bodiless callee with an inert contract must pass:\n%s", res.Render())
	}
}

func TestInterruptHandlerBodilessPureNoallocCallee(t *testing.T) {
	res := checkIsrWith(t, `
pub pure noalloc fn raw_op()
`, `
import {raw_op} from buflib

export(interrupt) {
    pub fn tick() {
        raw_op()
    }
}
`, isrTestContracts{}, sem.InterruptConfig{})
	if out := res.Render(); !strings.Contains(out, "E1082") {
		t.Fatalf("`pure noalloc` alone is no interrupt contract either:\n%s", out)
	}
}
