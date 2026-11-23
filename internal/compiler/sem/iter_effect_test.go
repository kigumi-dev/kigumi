package sem_test

import (
	"strings"
	"testing"
)

const iterStateDropSrc = `
type Count = { pub limit Int }
type Cursor = resource { mut i Int, limit Int }

fn Cursor.drop(move self) { print "IMPURE CURSOR DROP" }
fn Count.iter(self) -> Cursor { Cursor { i: 0, limit: self.limit } }
fn Cursor.next(mut self) -> Int? { None }

pure fn total(c: Count) -> Int {
    let mut s = 0
    for v in c { s = s + v }
    s
}

fn main() { print "total=${total(Count { limit: 1 })}" }
`

const iterWildcardDropSrc = `
type Count = { pub limit Int }
type State = { pub i Int }
type Res = resource { pub n Int }

fn Res.drop(move self) { print "IMPURE RES DROP" }
fn Count.iter(self) -> State { State { i: 0 } }
fn State.next(mut self) -> Res? { None }

pure fn discard(c: Count) {
    for _ in c {}
}

fn main() { discard(Count { limit: 1 }) }
`

// A custom `for` iterator owns its state, and a wildcard discards a non-Copy
// item; both drops must reach the effects pass, so a `pure fn` cannot run a
// side-effecting destructor silently.
func TestProtocolIteratorDropEffects(t *testing.T) {
	for _, tc := range []struct {
		name, src, typ string
	}{
		{"state", iterStateDropSrc, "Cursor"},
		{"wildcard item", iterWildcardDropSrc, "Res"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res := checkBlocking(t, "", tc.src)
			out := res.Render()
			if !res.HasErrors() || !strings.Contains(out, "E906") || !strings.Contains(out, "dropping `"+tc.typ+"` is not `pure`") {
				t.Fatalf("expected E906 for dropping %s:\n%s", tc.typ, out)
			}
		})
	}
}
