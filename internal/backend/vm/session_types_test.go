package vm_test

import "testing"

const sessionTypesSrc = `type Pair[T] = { pub a T, pub b T }

fn id[T](x: T) -> T { x }

fn apply(f: fn(Int) -> Int, x: Int) -> Int { f(x) }

type Buf[const N: usize] = { pub n Int }

fn fixed(b: Buf[4]) -> Int { b.n }

fn wide[const N: usize](b: Buf[N]) -> Int { b.n }

fn pick(a: &Int, b: &Int) -> Int { 1 }

fn swap[T](p: Pair[T]) -> Pair[T] { p }

print "types"
`

func TestSessionAcceptsUnchangedStructuredTypes(t *testing.T) {
	base := buildSessionProgram(t, sessionTypesSrc)
	s := newSession(t, base)
	defer s.Close()
	next := buildSessionProgram(t, sessionTypesSrc+"\nfn extra() -> Int { 1 }\n")
	if err := s.UseProgram(next); err != nil {
		t.Fatalf("unchanged structured types were refused: %v", err)
	}
}
