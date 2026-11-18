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

func TestSessionAcceptsOverloadsOfOneName(t *testing.T) {
	cases := map[string][2]string{
		"parameter types": {"fn f(x: Int) -> Int { x }\n", "fn f(x: Float) -> Float { x }\n"},
		"arity":           {"fn f() -> Int { 0 }\n", "fn f(x: Int) -> Int { x }\n"},
		"generic":         {"fn f(x: Int) -> Int { x }\n", "fn f[T](x: T, y: T) -> T { x }\n"},
		"methods": {"type A = { n Int }\nfn A.m(self, x: Int) -> Int { x }\n",
			"fn A.m(self, x: Float) -> Float { x }\n"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			s := newSession(t, buildSessionProgram(t, c[0]+"print \"a\"\n"))
			defer s.Close()
			both := c[0] + c[1] + "print \"a\"\n"
			if err := s.UseProgram(buildSessionProgram(t, both)); err != nil {
				t.Fatalf("adding an overload was refused: %v", err)
			}
			if err := s.UseProgram(buildSessionProgram(t, both+"fn extra() -> Int { 1 }\n")); err != nil {
				t.Fatalf("a later cell was refused: %v", err)
			}
		})
	}
}
