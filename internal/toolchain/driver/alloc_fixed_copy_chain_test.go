package driver_test

import (
	"fmt"
	"testing"
)

const chainPrelude = `type Pt = {
    pub mut x Int
    pub mut y Int
}

type Wrap = {
    pub mut a FixedArray[Pt, 3]
}

fn mk() -> Wrap {
    Wrap { a: FixedArray.filled[Pt, 3](Pt { x: 1, y: 2 }) }
}
`

var chainShapes = []struct{ name, src string }{
	{"field", chainPrelude + `
noalloc fn op(w: &mut Wrap) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut w = mk()
    let mut s = 0
    for i in 0..rounds {
        op(&mut w)
        s = s + i
    }
    s
}

print "ok=${run(%d)}"
`},
	{"method", chainPrelude + `
noalloc fn Wrap.op(mut self) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut w = mk()
    let mut s = 0
    for i in 0..rounds {
        w.op()
        s = s + i
    }
    s
}

print "ok=${run(%d)}"
`},
	{"undeclared", chainPrelude + `
fn Wrap.op(mut self) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut w = mk()
    let mut s = 0
    for i in 0..rounds {
        w.op()
        s = s + i
    }
    s
}

print "ok=${run(%d)}"
`},
	{"closure", chainPrelude + `
fn run(rounds: Int) -> Int {
    let mut w = mk()
    let mut s = 0
    let f = () => {
        %s
    }
    for i in 0..rounds {
        f()
        s = s + i
    }
    s + w.a[0].x
}

print "ok=${run(%d)}"
`},
}

// The element copy through a field, a receiver or a captured cell fills the slot's own box like the plain array form.
func TestFixedElementCopyThroughPlaceAllocatesNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, sh := range chainShapes {
		t.Run(sh.name, func(t *testing.T) {
			t.Parallel()
			target := "w.a"
			if sh.name == "method" || sh.name == "undeclared" {
				target = "self.a"
			}
			copyBody := fmt.Sprintf("%s[0] = %s[1]", target, target)
			src := func(body string, rounds int) string { return fmt.Sprintf(sh.src, body, rounds) }
			slope := func(count func(*testing.T, string) int, body string) int {
				return count(t, src(body, 6)) - count(t, src(body, 2))
			}
			if got, want := slope(nativeAllocs, copyBody), slope(nativeAllocs, "let k = 0"); got != want {
				t.Errorf("native: the element copy allocates %d over 4 rounds, a body without it %d", got, want)
			}
			if got := slope(vmAllocs, copyBody); got != 0 {
				t.Errorf("vm: the element copy allocates %d over 4 rounds", got)
			}
		})
	}
}

const nestedCopySrc = `type Pt = {
    pub mut x Int
    pub mut y Int
}

type Grid = {
    pub mut rows FixedArray[FixedArray[Pt, 2], 2]
}

noalloc fn op(g: &mut Grid) {
    %s
}

fn run(rounds: Int) -> Int {
    let mut g = Grid { rows: FixedArray.filled(FixedArray.filled(Pt { x: 1, y: 2 })) }
    let mut s = 0
    for i in 0..rounds {
        op(&mut g)
        s = s + i
    }
    s
}

print "ok=${run(%d)}"
`

// A copy between slots of nested fixed arrays fills the slot's own box too.
func TestFixedNestedElementCopyAllocatesNothing(t *testing.T) {
	t.Parallel()
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	src := func(body string, rounds int) string { return fmt.Sprintf(nestedCopySrc, body, rounds) }
	slope := func(count func(*testing.T, string) int, body string) int {
		return count(t, src(body, 6)) - count(t, src(body, 2))
	}
	const copyBody = "g.rows[0][1] = g.rows[1][0]"
	if got, want := slope(nativeAllocs, copyBody), slope(nativeAllocs, "let k = 0"); got != want {
		t.Errorf("native: the nested copy allocates %d over 4 rounds, a body without it %d", got, want)
	}
	if got := slope(vmAllocs, copyBody); got != 0 {
		t.Errorf("vm: the nested copy allocates %d over 4 rounds", got)
	}
}
