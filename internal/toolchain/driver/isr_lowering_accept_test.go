package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

const isrLoweringAcceptSrc = `type Mode = | Idle | Busy(Int)

fn pick(m: Mode) -> Int {
    match m {
        Idle -> 0
        Busy(n) -> n
    }
}

fn measure(s: String) {
    let r = s.len()
    _ = r
}

fn forward(s: String) {
    measure(s)
}

export(interrupt) {
    pub fn tick() {
        let b = true
        let r = match b {
            true -> 1
            false -> 2
        }
        let o: Option[Int] = None
        let v = match o {
            Some(x) -> x
            None -> 0
        }
        let mut n = 0
        for i in 0..4 {
            n = n.wrappingAdd(i)
        }
        for i in 0..=r {
            n = n.wrappingAdd(i)
        }
        _ = n.wrappingAdd(v).wrappingAdd(pick(Idle))
        measure("abc")
        forward("abc")
    }
}

fn main() -> Void! {}
`

// The fall-through panic of an exhaustive match, a guarded range counter and the drop of an immortal argument never trap or free.
func TestInterruptLoweringAcceptsInertShapes(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "", isrLoweringAcceptSrc))
	if err != nil || res.HasErrors() {
		t.Fatalf("tick must pass the final lowering check: %v\n%s", err, res.Render())
	}
}

const isrLoweringOwnedSrc = `fn measure(s: String) {
    let r = s.len()
    _ = r
}

fn relay(s: String) {
    measure(s)
}

export(interrupt) {
    pub fn tick() {
        relay("abc")
    }
}

pub fn other(s: String) {
    measure(s)
}

fn main() -> Void! {
    other("x".repeat(3))
}
`

// A helper reached from a handler with a literal stays accepted even when another caller passes it an owned string.
func TestInterruptLoweringParamsPerPath(t *testing.T) {
	t.Parallel()
	res, err := driver.Check(isrModule(t, "", isrLoweringOwnedSrc))
	if err != nil || strings.Contains(res.Render(), "E1090") {
		t.Fatalf("only the handler's own call path decides its verdict: %v\n%s", err, res.Render())
	}
}
