package driver_test

import (
	"fmt"
	"testing"
)

const boxAccessorSrc = `import {Box} from std/alloc

type Pt = {
    pub mut x Int
    pub mut y Int
}

noalloc fn op(b: &mut Box[%s]) -> Int {
    %s
}

fn run(rounds: Int) -> Int {
    let mut b = Box.new(%s)
    let mut s = 0
    for i in 0..rounds {
        s = s + op(&mut b)
    }
    s
}

print "ok=${run(%d)}"
`

const boxIntoSrc = `import {Box} from std/alloc

type Pt = {
    pub mut x Int
    pub mut y Int
}

noalloc fn op(b: Box[%s]) -> Int {
    %s
}

fn run(rounds: Int) -> Int {
    let mut s = 0
    for i in 0..rounds {
        s = s + op(Box.new(%s))
    }
    s
}

print "ok=${run(%d)}"
`

// get, with, withMut and into add no allocation over a body that leaves the box alone.
func TestBoxAccessorsAllocateNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cases := []struct{ name, tmpl, elem, init, body, base string }{
		{"get", boxAccessorSrc, "Int", "7", "b.get()", "0"},
		{"getFloat", boxAccessorSrc, "Float", "1.5", "if b.get() > 1.0 { 1 } else { 0 }", "0"},
		{"with", boxAccessorSrc, "Pt", "Pt { x: 1, y: 2 }", "if b.with((p: &Pt) => p.x > 0) { 1 } else { 0 }", "0"},
		{"withMut", boxAccessorSrc, "Pt", "Pt { x: 1, y: 2 }", "b.withMut((p: &mut Pt) => {\n        p.x = 5\n    })\n    0", "0"},
		{"into", boxIntoSrc, "Int", "7", "b.into()", "0"},
		{"intoRecord", boxIntoSrc, "Pt", "Pt { x: 1, y: 2 }", "let p = b.into()\n    p.x", "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			src := func(body string, rounds int) string {
				return fmt.Sprintf(c.tmpl, c.elem, body, c.init, rounds)
			}
			slope := func(count func(*testing.T, string) int, body string) int {
				return count(t, src(body, 6)) - count(t, src(body, 2))
			}
			if got, want := slope(nativeAllocs, c.body), slope(nativeAllocs, c.base); got != want {
				t.Errorf("native: %q allocates %d over 4 rounds, a body without it %d", c.body, got, want)
			}
			if got, want := slope(vmAllocs, c.body), slope(vmAllocs, c.base); got != want {
				t.Errorf("vm: %q allocates %d over 4 rounds, a body without it %d", c.body, got, want)
			}
		})
	}
}
