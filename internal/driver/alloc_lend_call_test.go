package driver_test

import (
	"fmt"
	"testing"
)

const lendCallSrc = `type Point = {
    pub x Int
    pub y Int
}

type Prio = Low | Mid(n: Int) | High

noalloc fn peek(p: &Point) -> Int { p.x * 10 + p.y }

noalloc fn forward(p: &Point) -> Int { peek(p) + 1 }

pure noalloc fn Prio.rank(self) -> Int {
    match self {
        Low -> 0
        Mid(n) -> 1 + n
        High -> 100
    }
}

pure noalloc fn Prio.compareTo(self, other: &Prio) -> Ordering {
    if self.rank() < other.rank() { Ordering.Less } else { Ordering.Greater }
}

fn lendsInferred(i: Int) -> Int {
    let p = Point { x: i, y: 2 }
    peek(&p) + forward(&p)
}

fn lendsDeeper(i: Int) -> Int { lendsInferred(i) + 1 }

noalloc fn call(i: Int) -> Int {
    %s
}

fn run(rounds: Int) -> Int {
    let mut s = 0
    for i in 0..rounds {
        s = s + call(i)
    }
    s
}

print "ok=${run(%d)}"
`

var lendCallBodies = map[string]string{
	"written":  "let p = Point { x: i, y: 2 }\n    peek(&p) + forward(&p)",
	"coerced":  "let p = Point { x: i, y: 2 }\n    peek(p)",
	"inferred": "lendsInferred(i)",
	"deeper":   "lendsDeeper(i)",
	"operator": "let a = Mid(i)\n    let b = Mid(2)\n    if a < b { 1 } else { 0 }",
	"equals":   "let a = Point { x: i, y: 2 }\n    let b = Point { x: 1, y: 2 }\n    if a.equals(&b) && a == b { 1 } else { 0 }",
}

// A borrow the callee only reads is passed as the value, so a noalloc caller that lends allocates nothing.
func TestLentCallsInNoallocCodeAllocateNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for name, body := range lendCallBodies {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src := func(rounds int) string { return fmt.Sprintf(lendCallSrc, body, rounds) }
			if got := nativeAllocs(t, src(6)) - nativeAllocs(t, src(2)); got != 0 {
				t.Errorf("native: %q allocates %d over 4 rounds", body, got)
			}
			if got := vmAllocs(t, src(6)) - vmAllocs(t, src(2)); got != 0 {
				t.Errorf("vm: %q allocates %d over 4 rounds", body, got)
			}
		})
	}
}
