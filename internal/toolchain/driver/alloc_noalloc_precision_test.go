package driver_test

import (
	"fmt"
	"testing"
)

const precisionProgram = `type Named = {
    pub name String
    pub n Int
}

type Mixed = {
    pub left Named
    pub f Float
    pub o Int?
    pub t (Int, Bool)
}

type Tag = Text(s: String) | Num(n: Int) | Boxed(m: Mixed) | Empty

noalloc fn sameMixed(a: &Mixed, b: &Mixed) -> Bool { a == b }

noalloc fn sameTag(a: &Tag, b: &Tag) -> Bool { a == b }

noalloc fn constBool(a: &Mixed, b: &Mixed) -> Bool { true }

noalloc fn constTag(a: &Tag, b: &Tag) -> Bool { true }

noalloc fn constInt(xs: &Array[Int]) -> Int { 5 }

fn run(rounds: Int) -> Int {
    let a = Mixed { left: Named { name: "a", n: 1 }, f: 1.5, o: Some(3), t: (1, true) }
    let b = Mixed { left: Named { name: "a", n: 1 }, f: 1.5, o: Some(3), t: (1, true) }
    let t = Boxed(a)
    let u = Text("x")
    let xs = Array.of(3, 4, 5)
    let mut s = 0
    for i in 0..rounds {
        %s
    }
    s
}

print "ok=${run(%d)}"
`

var precisionCases = []struct{ name, body, baseline string }{
	{"derived record equals", "if sameMixed(&a, &b) { s = s + 1 }", "if constBool(&a, &b) { s = s + 1 }"},
	{"derived variant equals", "if sameTag(&t, &u) { s = s + 1 }", "if constTag(&t, &u) { s = s + 1 }"},
	{"first", "if (xs.first() || 0) == 7 { s = s + 1 }", "if constInt(&xs) == 7 { s = s + 1 }"},
	{"last", "if (xs.last() || 0) == 7 { s = s + 1 }", "if constInt(&xs) == 7 { s = s + 1 }"},
}

func precisionSrc(body string, rounds int) string {
	return fmt.Sprintf(precisionProgram, body, rounds)
}

func TestNoallocPrecisionCasesAllocateLikeAConstant(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, c := range precisionCases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			growth := func(body string) int {
				return nativeAllocs(t, precisionSrc(body, 8)) - nativeAllocs(t, precisionSrc(body, 2))
			}
			if got, want := growth(c.body), growth(c.baseline); got != want {
				t.Errorf("%s allocates %d more times over 6 rounds, a constant-returning noalloc call %d", c.name, got, want)
			}
		})
	}
}
