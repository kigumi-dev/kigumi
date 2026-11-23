package driver_test

import (
	"testing"
	"time"
)

const contextNonstandardCallsSrc = `context base: Int

@provides(base)
type Cfg = { pub base Int }

fn add(x: Int, y: Int = base) -> Int {
    x + y
}

type Count = { pub limit Int }
type State = { pub cur Int, pub limit Int }

fn Count.iter(self) -> State {
    State { cur: if base > 0 { 0 } else { 100 }, limit: self.limit }
}

fn State.next(mut self) -> Int? {
    if base < 0 || self.cur >= self.limit {
        None
    } else {
        let v = self.cur
        self = State { cur: v + 1, limit: self.limit }
        Some(v)
    }
}

type Boxed = { pub v Int }

fn Boxed.branch(move self) -> Branch[Int, Void] {
    Branch.Hit(self.v + base)
}

type Val = { pub n Int }

pure fn Val.equals(self, other: &Val) -> Bool {
    self.n + base == other.n
}

type Table = { pub n Int }

fn Table.index(self, i: usize) -> Int {
    if i == 0 { self.n + base } else { self.n }
}

type Vec = { pub x Int }

fn Vec.+(a: Vec, b: Vec) -> Vec {
    Vec { x: a.x + b.x + base }
}

type Flag = { pub n Int }

fn Flag.!(a: Flag) -> Int {
    a.n + base
}

fn main() {
    context (Cfg { base: 100 }) {
        print "${add(5)}"
        let mut total = 0
        for v in (Count { limit: 3 }) { total = total + v }
        print "${total}"
        let b = Boxed { v: 1 }
        let hit: Int = b || 0
        print "${hit}"
        let a = Val { n: 1 }
        let c = Val { n: 101 }
        print "${a == c}"
        let t = Table { n: 5 }
        print "${t[0]}"
        let v1 = Vec { x: 1 }
        let v2 = Vec { x: 2 }
        print "${(v1 + v2).x}"
        let mut v3 = Vec { x: 3 }
        v3 += Vec { x: 4 }
        print "${v3.x}"
        print "${!Flag { n: 1 }}"
    }
}
`

const replacingIteratorSrc = `type State = {
    pub cur Int
    pub limit Int
}

type Range2 = {
    pub limit Int
}

fn Range2.iter(self) -> State {
    State { cur: 0, limit: self.limit }
}

fn State.next(mut self) -> Int? {
    if self.cur >= self.limit {
        None
    } else {
        let v = self.cur
        self = State { cur: v + 1, limit: self.limit }
        Some(v)
    }
}

fn main() {
    for x in (Range2 { limit: 3 }) {
        print "${x}"
    }
}
`

// Implicit `context` provider arguments must reach every nonstandard call
// path: default thunks, the custom `for` protocol, `||` fallbacks, protocol
// comparisons, custom index and user-defined operators.
func TestImplicitContextReachesNonstandardCalls(t *testing.T) {
	t.Parallel()
	want := "105\n3\n101\ntrue\n105\n103\n107\n101\n"
	if got := runGeneratedVM(t, contextNonstandardCallsSrc, 20*time.Second); got != want {
		t.Fatalf("vm output:\n%s\nwant:\n%s", got, want)
	}
	if testCompiler().CCompiler() != nil {
		if got := runGeneratedAOT(t, contextNonstandardCallsSrc, 20*time.Second); got != want {
			t.Fatalf("native output:\n%s\nwant:\n%s", got, want)
		}
	}
}

// A `next(mut self)` body that assigns `self = ...` must write back into the
// loop-owned iterator, not panic on the VM or spin forever natively.
func TestProtocolIteratorReplacingSelf(t *testing.T) {
	t.Parallel()
	want := "0\n1\n2\n"
	if got := runGeneratedVM(t, replacingIteratorSrc, 20*time.Second); got != want {
		t.Fatalf("vm output:\n%s\nwant:\n%s", got, want)
	}
	if testCompiler().CCompiler() != nil {
		if got := runGeneratedAOT(t, replacingIteratorSrc, 20*time.Second); got != want {
			t.Fatalf("native output:\n%s\nwant:\n%s", got, want)
		}
	}
}

// The protocol records `iter` and `next` at the same head node; a provider
// needed by only one of them must not leak into the other call.
func TestProtocolIteratorContextOneSided(t *testing.T) {
	t.Parallel()
	const iterOnly = `context base: Int

@provides(base)
type Cfg = { pub base Int }

type Count = { pub limit Int }
type State = { pub cur Int, pub limit Int }

fn Count.iter(self) -> State {
    State { cur: if base > 0 { 0 } else { 100 }, limit: 2 }
}

fn State.next(mut self) -> Int? {
    if self.cur >= self.limit {
        None
    } else {
        let v = self.cur
        self = State { cur: v + 1, limit: self.limit }
        Some(v)
    }
}

fn main() {
    context (Cfg { base: 7 }) {
        for x in (Count { limit: 2 }) { print "${x}" }
    }
}
`
	const nextOnly = `context base: Int

@provides(base)
type Cfg = { pub base Int }

type Count = { pub limit Int }
type State = { pub cur Int, pub limit Int }

fn Count.iter(self) -> State {
    State { cur: 0, limit: 2 }
}

fn State.next(mut self) -> Int? {
    if base < 0 || self.cur >= self.limit {
        None
    } else {
        let v = self.cur
        self = State { cur: v + 1, limit: self.limit }
        Some(v)
    }
}

fn main() {
    context (Cfg { base: 7 }) {
        for x in (Count { limit: 2 }) { print "${x}" }
    }
}
`
	want := "0\n1\n"
	for _, tc := range []struct{ name, src string }{{"iter only", iterOnly}, {"next only", nextOnly}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := runGeneratedVM(t, tc.src, 20*time.Second); got != want {
				t.Fatalf("vm output:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// A user-defined operator on a generic owner passes its type-parameter
// witness like any other associated call.
func TestUserOperatorWitnessArgument(t *testing.T) {
	t.Parallel()
	const src = `interface Addable {
    pure fn add(self, other: &Self) -> Self
}

type Num = { pub n Int }

pure fn Num.add(self, other: &Num) -> Num {
    Num { n: self.n + other.n }
}

type Box[T] = { pub v T }

fn Box[T].+[U: Addable](a: Box[T], b: U) -> U {
    b
}

fn main() {
    let a = Box { v: 1 }
    let r = a + (Num { n: 2 })
    print "${r.n}"
}
`
	if got := runGeneratedVM(t, src, 20*time.Second); got != "2\n" {
		t.Fatalf("vm output:\n%s\nwant:\n2", got)
	}
	if testCompiler().CCompiler() != nil {
		if got := runGeneratedAOT(t, src, 20*time.Second); got != "2\n" {
			t.Fatalf("native output:\n%s\nwant:\n2", got)
		}
	}
}
