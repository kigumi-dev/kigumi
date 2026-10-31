package driver_test

import (
	"fmt"
	"testing"
)

const lendDecls = `type Point = {
    pub x Int
    pub y Int
}

type Tick = Up(n: Int) | Flat

pure fn Tick.rank(self) -> Int {
    match self {
        Up(n) -> 1 + n
        Flat -> 0
    }
}

pure fn Tick.compareTo(self, other: &Tick) -> Ordering {
    let a = self.rank()
    let b = other.rank()
    if a < b { Ordering.Less } else if a > b { Ordering.Greater } else { Ordering.Equal }
}

pure fn Point.compareTo(self, other: &Point) -> Ordering {
    if self.x < other.x { Ordering.Less } else if self.x > other.x { Ordering.Greater } else { Ordering.Equal }
}

fn peek(p: &Point) -> Int { p.x * 10 + p.y }

fn forward(p: &Point) -> Int { peek(p) }

fn fingerprint[T: Hash](a: &T) -> u64 { hash(a) }
`

var lendCases = []struct{ name, body string }{
	{"record compareTo", "let p = Point { x: i, y: 1 }\n        let q = Point { x: 3, y: 1 }\n        if p.compareTo(&q) == Ordering.Less { s = s + 1 }"},
	{"read-only chain", "let p = Point { x: i, y: 1 }\n        s = s + forward(&p) + peek(&p)"},
	{"generic helper over a reference", "let p = Point { x: i, y: 1 }\n        if fingerprint(&p) == 7 { s = s + 1 }"},
	{"adt right operand of <", "let a = Up(i)\n        let b = Up(i + 1)\n        if a < b { s = s + 1 }"},
	{"borrowed temporary", "let a = Up(i)\n        if a.compareTo(&Up(i + 2)) == Ordering.Less { s = s + 1 }"},
}

func lendSource(body string, rounds int) string {
	return fmt.Sprintf("%s\nfn run(n: Int) -> Int {\n    let mut s = 0\n    for i in 0..n {\n        %s\n    }\n    s\n}\n\nprint \"ok=${run(%d)}\"\n", lendDecls, body, rounds)
}

func TestFaultInjectionLentDirectValuesAllocateNothingPerRound(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range lendCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			short, long := lendSource(c.body, 4), lendSource(c.body, 12)
			vmPer := vmAllocs(t, long) - vmAllocs(t, short)
			nativePer := nativeAllocs(t, long) - nativeAllocs(t, short)
			if vmPer != 0 || nativePer != 0 {
				t.Errorf("8 more rounds cost %d allocations on the VM and %d on native, want none", vmPer, nativePer)
			}
		})
	}
}
