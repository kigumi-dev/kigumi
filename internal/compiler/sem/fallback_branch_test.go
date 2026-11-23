package sem_test

import (
	"strings"
	"testing"
)

// Custom `||` must validate the branch method it lowers, not leave the
// mismatch to the MIR verifier or a wrong-arity OpCall.
func TestFallbackBranchValidation(t *testing.T) {
	const header = `
type Boxed = { pub v Int }
`
	cases := []struct {
		name, src, code string
	}{
		{
			"arity",
			header + `
fn Boxed.branch(move self, extra: Int) -> Branch[Int, Void] { Branch.Hit(self.v + extra) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E625",
		},
		{
			"receiver",
			header + `
fn Boxed.branch(self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E626",
		},
		{
			"async",
			header + `
async fn Boxed.branch(move self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E627",
		},
		{
			"hidden witness",
			header + `
fn Boxed.branch[T: Eq](move self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E628",
		},
		{
			"hidden const",
			header + `
fn Boxed.branch[const N: usize](move self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E628",
		},
		{
			"ambiguous overload",
			header + `
fn Boxed.branch(move self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn Boxed.branch(move self, extra: Int) -> Branch[Int, Void] { Branch.Hit(self.v + extra) }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E624",
		},
		{
			"result",
			header + `
fn Boxed.branch(move self) -> Int { self.v }
fn use(b: Boxed) -> Int { b || 0 }
`,
			"E629",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := checkBlocking(t, "", tc.src)
			out := res.Render()
			if !res.HasErrors() || !strings.Contains(out, tc.code) {
				t.Fatalf("expected %s:\n%s", tc.code, out)
			}
		})
	}
}

func TestFallbackBranchAcceptsBranchAndOption(t *testing.T) {
	for _, src := range []string{
		`
type Boxed = { pub v Int }
fn Boxed.branch(move self) -> Branch[Int, Void] { Branch.Hit(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
		`
type Boxed = { pub v Int }
fn Boxed.branch(move self) -> Option[Int] { Option.Some(self.v) }
fn use(b: Boxed) -> Int { b || 0 }
`,
	} {
		if res := checkBlocking(t, "", src); res.HasErrors() {
			t.Fatalf("unexpected errors:\n%s", res.Render())
		}
	}
}
