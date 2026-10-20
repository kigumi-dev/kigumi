package driver_test

import (
	"fmt"
	"testing"
)

const fixedCopySrc = `type Pt = {
    pub mut x Int
    pub mut y Int
}

type Cell = {
    pub mut at Pt
    pub mut on Bool
    pub mut w Float
}

noalloc fn op(xs: &mut FixedArray[%s, 2]) -> Unit {
    %s
}

fn run(rounds: Int) -> Int {
    let mut a = FixedArray.filled[%s, 2](%s)
    let mut s = 0
    for i in 0..rounds {
        op(&mut a)
        s = s + i
    }
    s
}

print "ok=${run(%d)}"
`

// Filling a slot from another element reuses the slot's box: the call costs what a body without the copy does.
func TestFixedElementCopyAllocatesNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cases := []struct{ name, elem, init, copy, base string }{
		{"record", "Pt", "Pt { x: 1, y: 2 }", "xs[0] = xs[1]", "let k = 0"},
		{"nested", "Cell", "Cell { at: Pt { x: 1, y: 2 }, on: false, w: 0.5 }", "xs[0] = xs[1]", "let k = 0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := func(body string, rounds int) string {
				return fmt.Sprintf(fixedCopySrc, c.elem, body, c.elem, c.init, rounds)
			}
			slope := func(count func(*testing.T, string) int, body string) int {
				return count(t, src(body, 6)) - count(t, src(body, 2))
			}
			if got, want := slope(nativeAllocs, c.copy), slope(nativeAllocs, c.base); got != want {
				t.Errorf("native: the element copy allocates %d over 4 rounds, a body without it %d", got, want)
			}
			if got := slope(vmAllocs, c.copy); got != 0 {
				t.Errorf("vm: the element copy allocates %d over 4 rounds", got)
			}
		})
	}
}
