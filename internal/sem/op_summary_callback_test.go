package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/sem"
)

const callbackWrapperSrc = `
import {Box, Shared} from std/alloc
import ffi from std/ffi
import fs from std/fs

fn boxPure(b: &Box[Int]) -> Bool { b.with((x: &Int) => x == 1) }
fn boxMutPure(b: &mut Box[Int]) -> Unit { b.withMut((x: &mut Int) => { x = 2 }) }
fn sharedPure(s: &Shared[Int]) -> Bool { s.with((x: &Int) => x == 1) }
fn sharedMutPure(s: &Shared[Int]) -> Unit { s.withMut((x: &mut Int) => { x = 2 }) }
fn arrayPure(a: &Array[Int]) -> Bool? { a.with(0, (x: &Int) => x == 1) }
fn arrayMutPure(a: &mut Array[Int]) -> Unit? { a.withMut(0, (x: &mut Int) => { x = 2 }) }
fn mapPure(a: &Array[Int]) -> Result[Array[Bool], AllocError] { a.tryMap((x: &Int) => x == 1) }
fn tablePure(t: &ffi.HandleTable[Int], h: &ffi.FfiHandle[Int]) -> Bool { t.with(h, (x: &Int) => x == 1) }
fn pinPure(p: *mut u8) -> Bool { unsafe { ffi.Pin.with(p, (x: &Int) => x == 1) } }

fn boxAlloc(b: &Box[Int]) -> Bool { b.with((x: &Int) => { let c = Box.new(x); true }) }
fn sharedAlloc(s: &Shared[Int]) -> Bool { s.with((x: &Int) => { let c = Box.new(x); true }) }
fn arrayAlloc(a: &Array[Int]) -> Bool? { a.with(0, (x: &Int) => { let c = Box.new(x); true }) }
fn pinAlloc(p: *mut u8) -> Bool { unsafe { ffi.Pin.with(p, (x: &Int) => { let c = Box.new(x); true }) } }

fn boxPanic(b: &Box[Int]) -> Int { b.with((x: &Int) => Some(1).expect("none") + x) }
fn sharedPanic(s: &Shared[Int]) -> Int { s.with((x: &Int) => Some(1).expect("none") + x) }
fn arrayPanic(a: &Array[Int]) -> Int? { a.with(0, (x: &Int) => Some(1).expect("none") + x) }

fn slurp() -> Bool! {
    let _ = host.files().read(fs.Path.fromString("x")?)?
    true
}

fn sharedBlock(s: &Shared[Int]) -> Bool {
    s.with((x: &Int) => slurp() is Ok(_))
}
`

func opFact(r *sem.Result, name string, f sem.OpFact) bool {
	return r.Fn(findFn2(r, name)).Ops.Facts[f].Maybe
}

func findFn2(r *sem.Result, name string) sem.EntityID {
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entities[id]; e.Kind == sem.EntFn && e.Name == name && r.Fn(sem.EntityID(id)).Body != 0 {
			return sem.EntityID(id)
		}
	}
	return 0
}

func TestCallbackPrimitivesComposeCallbackFacts(t *testing.T) {
	r := checkOp(t, callbackWrapperSrc)
	clean := []string{"boxPure", "boxMutPure"}
	for _, name := range clean {
		s := r.Fn(findFn2(r, name)).Ops
		for f := sem.OpFact(0); f <= sem.FactSpawn; f++ {
			if s.Facts[f].Maybe {
				t.Errorf("%s: a pure callback must leave a noalloc wrapper without %v", name, f)
			}
		}
	}
	for _, name := range []string{"sharedPure", "sharedMutPure", "arrayPure", "arrayMutPure", "mapPure", "tablePure", "pinPure"} {
		s := r.Fn(findFn2(r, name)).Ops
		for _, f := range []sem.OpFact{sem.FactRelease, sem.FactBlock, sem.FactSuspend, sem.FactIO, sem.FactSync, sem.FactSpawn} {
			if s.Facts[f].Maybe {
				t.Errorf("%s: a pure callback must not leave an unverified %v fact", name, f)
			}
		}
	}
	for _, name := range []string{"boxAlloc", "sharedAlloc", "arrayAlloc", "pinAlloc"} {
		if !opFact(r, name, sem.FactAlloc) {
			t.Errorf("%s: an allocating callback must make the call allocate", name)
		}
	}
	for _, name := range []string{"boxPanic", "sharedPanic", "arrayPanic"} {
		if !opFact(r, name, sem.FactPanic) {
			t.Errorf("%s: a panicking callback must make the call may-panic", name)
		}
	}
	if !opFact(r, "sharedBlock", sem.FactBlock) {
		t.Error("sharedBlock: a blocking callback must make Shared.with block")
	}
}

func TestCallbackPrimitiveOwnPanic(t *testing.T) {
	r := checkOp(t, callbackWrapperSrc)
	for _, name := range []string{"sharedPure", "sharedMutPure", "tablePure"} {
		if !opFact(r, name, sem.FactPanic) {
			t.Errorf("%s: reentrant or stale access panics, so the wrapper itself may panic", name)
		}
	}
	for _, name := range []string{"boxPure", "boxMutPure", "arrayPure", "arrayMutPure", "mapPure", "pinPure"} {
		if opFact(r, name, sem.FactPanic) {
			t.Errorf("%s: this primitive never panics on its own", name)
		}
	}
}

func TestInterruptHandlerSharedWithBlockingCallback(t *testing.T) {
	res := checkIsr(t, `
import {Shared} from std/alloc
import fs from std/fs

fn slurp() -> Bool! {
    let _ = host.files().read(fs.Path.fromString("x")?)?
    true
}

fn drain(s: &Shared[Int]) -> Bool {
    s.with((x: &Int) => slurp() is Ok(_))
}

let cell = Shared.new(1)

export(interrupt) {
    pub fn tick() -> Unit {
        _ = drain(&cell)
    }
}
`)
	out := res.Render()
	if !res.HasErrors() || !strings.Contains(out, "E108") || !strings.Contains(out, "calls `with`") {
		t.Fatalf("a handler reaching Shared.with with a blocking callback must be rejected:\n%s", out)
	}
}
