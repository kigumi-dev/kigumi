package driver_test

import (
	"fmt"
	"strings"
	"testing"
)

const protocolRounds = 4

const protocolDirect = `type Point = {
    pub x Int
    pub y Int
}

type Tag = On(n: Int, ok: Bool) | Off

fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        let a = Point { x: i, y: 1 }
        let b = Point { x: i, y: 2 }
        let t = On(i, true)
        let u = if i > 0 { On(i, true) } else { Off }
        if a == b { s = s + 1 }
        if a != b { s = s + 2 }
        if t == u { s = s + 3 }
        if hash(&a) == hash(&b) { s = s + 4 }
        if hash(&t) == hash(&u) { s = s + 5 }
        if a.equals(&b) { s = s + 6 }
    }
    s
}

print "ok=${run(%d)}"
`

const protocolBaseline = `fn run(n: Int) -> Int {
    let mut s = 0
    for i in 0..n {
        let a = i
        let b = i + 1
        let c = i > 0
        let d = i > 0
        if a == b { s = s + 1 }
        if a != b { s = s + 2 }
        if c == d { s = s + 3 }
        if a == b { s = s + 4 }
        if c == d { s = s + 5 }
        if a == b { s = s + 6 }
    }
    s
}

print "ok=${run(%d)}"
`

func TestDirectEqualsAndHashAllocateNothingOnTheVM(t *testing.T) {
	t.Parallel()
	direct, base := fmt.Sprintf(protocolDirect, protocolRounds), fmt.Sprintf(protocolBaseline, protocolRounds)
	if got, want := vmAllocs(t, direct), vmAllocs(t, base); got != want {
		t.Errorf("comparing and hashing direct values allocates %d times on the VM, the same loop over scalars %d", got, want)
	}
}

func TestDirectEqualsAndHashAllocateNothingNatively(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	direct, base := fmt.Sprintf(protocolDirect, protocolRounds), fmt.Sprintf(protocolBaseline, protocolRounds)
	if got, want := nativeAllocs(t, direct), nativeAllocs(t, base); got != want {
		t.Errorf("comparing and hashing direct values allocates %d times natively, the same loop over scalars %d", got, want)
	}
}

func TestDeclaredNoallocAcceptsDirectEqualsAndHash(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, strings.Replace(`type Point = {
    pub x Int
    pub y Int
}

type Tag = On(n: Int) | Off

noalloc fn same(a: Point, b: Point) -> Bool { a == b }

noalloc fn differ(a: Tag, b: Tag) -> Bool { a != b }

noalloc fn viaTuple(a: (Int, Bool), b: (Int, Bool)) -> Bool { a == b }

noalloc fn fingerprint(a: Point) -> u64 { hash(&a) }

noalloc fn call(a: Point, b: Point) -> u64 {
    if same(a, b) { fingerprint(a) } else { fingerprint(b) }
}

fn main() -> Unit! {
    print "${call(Point { x: 1, y: 2 }, Point { x: 1, y: 2 })} ${differ(On(1), Off)} ${viaTuple((1, true), (1, false))}"
}
`, "\t", "", 0))
	if out != "" {
		t.Fatalf("expected the comparisons and the hash to pass the build's check, got:\n%s", out)
	}
}
