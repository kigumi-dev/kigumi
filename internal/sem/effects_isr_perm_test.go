package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/sem"
)

const (
	isrNotify = "sync queue.notify width=32 impl=cas"
	isrAck    = "mmio uart.ack"
)

var isrDeviceContracts = isrTestContracts{contracts: map[string]sem.IsrContract{
	"buflib.notify": {Kind: sem.IsrSync, ID: isrNotify},
	"buflib.ack":    {Kind: sem.IsrMMIO, ID: isrAck},
	"buflib.quiet":  {Kind: sem.IsrInert},
	"buflib.port":   {Kind: sem.IsrInert},
}}

const isrDeviceLib = `
pub noalloc fn notify()

pub noalloc fn ack()

pub noalloc fn quiet()

pub type Port = resource {
    pub id Int
}

fn Port.drop(move self) {
    ack()
}

pub noalloc fn port() -> Port
`

const isrTwoHandlers = `
import {notify, quiet} from buflib

fn wake() {
    quiet()
    notify()
}

export(interrupt) {
    pub fn uart() {
        wake()
    }
    pub fn timer() {
        wake()
    }
}
`

func TestInterruptPermissionPerHandler(t *testing.T) {
	cfg := sem.InterruptConfig{Root: []string{isrNotify}, Handlers: map[string][]string{"main.timer": nil}}
	res := checkIsrWith(t, isrDeviceLib, isrTwoHandlers, isrDeviceContracts, cfg)
	out := res.Render()
	if strings.Contains(out, "handler `uart`") {
		t.Fatalf("uart takes the root common configuration, which permits the helper's sync:\n%s", out)
	}
	for _, want := range []string{"E1088", "handler `timer` reaches `" + isrNotify + "`", "calls `wake`", "calls `notify`, which performs `" + isrNotify + "`", "`timer` uses its own interrupt configuration, which permits no MMIO or synchronization operation within the root limit of `" + isrNotify + "`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("timer narrowed the root limit away, so the same helper must be rejected there (missing %q):\n%s", want, out)
		}
	}
}

func TestInterruptPermissionKeepsFacts(t *testing.T) {
	res := checkIsrWith(t, isrDeviceLib, isrTwoHandlers, isrDeviceContracts, sem.InterruptConfig{Root: []string{isrNotify}})
	if res.HasErrors() {
		t.Fatalf("both handlers inherit the permitted sync:\n%s", res.Render())
	}
	uart := lookupFn(t, res, "uart")
	if ex := res.ExplainOpFact(uart, sem.FactSync); !ex.Maybe || len(ex.Chain) == 0 {
		t.Fatalf("a permission must not erase the sync fact or its chain: %+v", ex)
	}
	perms, ok := res.InterruptPermissionsOf(uart)
	if !ok || perms.Own || len(perms.Permits) != 1 || perms.Permits[0] != isrNotify {
		t.Fatalf("uart's resolved permissions = %+v, %v", perms, ok)
	}
}

func TestInterruptPermissionRootLimit(t *testing.T) {
	cfg := sem.InterruptConfig{Handlers: map[string][]string{"main.uart": {isrNotify}, "main.gone": nil}}
	out := checkIsrWith(t, isrDeviceLib, isrTwoHandlers, isrDeviceContracts, cfg).Render()
	for _, want := range []string{"E1091", "the `Interrupt` record for handler `main.uart` names `" + isrNotify + "`, which is outside the root limit", "E1092", "names handler `main.gone`", "handler `uart` reaches `" + isrNotify + "`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a handler record must not widen the root limit (missing %q):\n%s", want, out)
		}
	}
	cfg = sem.InterruptConfig{Root: []string{"sync queue.notify width=64 impl=cas"}}
	out = checkIsrWith(t, isrDeviceLib, isrTwoHandlers, isrDeviceContracts, cfg).Render()
	for _, want := range []string{"E1091", "matches no reviewed contract", "E1088"} {
		if !strings.Contains(out, want) {
			t.Fatalf("an unknown permission must be rejected and grant nothing (missing %q):\n%s", want, out)
		}
	}
}

func TestInterruptPermissionNearMiss(t *testing.T) {
	contracts := isrDeviceContracts
	contracts.contracts = map[string]sem.IsrContract{"buflib.notify": {Kind: sem.IsrSync, ID: isrNotify}, "buflib.quiet": {Kind: sem.IsrInert}, "buflib.wide": {Kind: sem.IsrSync, ID: "sync queue.notify width=64 impl=cas"}}
	cfg := sem.InterruptConfig{Root: []string{"sync queue.notify width=64 impl=cas"}}
	out := checkIsrWith(t, isrDeviceLib, isrTwoHandlers, contracts, cfg).Render()
	if !strings.Contains(out, "permits `sync queue.notify width=64 impl=cas`, which differs in width, implementation or nesting condition") {
		t.Fatalf("a width mismatch must be rejected with the near miss shown:\n%s", out)
	}
}

func TestInterruptPermissionDropMMIO(t *testing.T) {
	const src = `
import {port} from buflib

export(interrupt) {
    pub fn tick() {
        let p = port()
        _ = p.id
    }
}
`
	out := checkIsrWith(t, isrDeviceLib, src, isrDeviceContracts, sem.InterruptConfig{}).Render()
	for _, want := range []string{"E1088", "reaches `" + isrAck + "`", "drops `Port`", "calls `drop`", "calls `ack`, which performs `" + isrAck + "`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("a drop doing unpermitted MMIO must be rejected with its path (missing %q):\n%s", want, out)
		}
	}
	if res := checkIsrWith(t, isrDeviceLib, src, isrDeviceContracts, sem.InterruptConfig{Root: []string{isrAck}}); res.HasErrors() {
		t.Fatalf("a drop doing permitted MMIO must pass:\n%s", res.Render())
	}
}

func TestInterruptPermissionCallback(t *testing.T) {
	const src = `
import {notify} from buflib

fn run(f: noalloc fn() -> Void) {
    f()
}

export(interrupt) {
    pub fn tick() {
        run(notify)
    }
}
`
	out := checkIsrWith(t, isrDeviceLib, src, isrDeviceContracts, sem.InterruptConfig{}).Render()
	if !strings.Contains(out, "export(interrupt) handler `tick`") {
		t.Fatalf("a callback reaching unpermitted sync must not pass:\n%s", out)
	}
}

func TestInterruptWrappingAndCheckedOps(t *testing.T) {
	ok := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let a: i64 = 7
        _ = a.wrappingAdd(1).rotateLeft(3)
        _ = a.toU8()
        _ = Char.fromInt(65)
    }
}
`)
	if ok.HasErrors() {
		t.Fatalf("wrapping and checked-returning primitives carry an inert contract:\n%s", ok.Render())
	}
	bad := checkIsr(t, `
export(interrupt) {
    pub fn tick() {
        let a: i64 = 7
        _ = a + 1
    }
}
`)
	if out := bad.Render(); !strings.Contains(out, "E1081") {
		t.Fatalf("checked arithmetic may panic:\n%s", out)
	}
}

func lookupFn(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntFn && e.Name == name {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no fn %s", name)
	return 0
}
