package verify_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirreach "kigumi/internal/compiler/mir/reach"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const simdSharedSrc = `import simd from std/simd
fn twice[const N: usize](v: simd.Vec[i32, N]) -> simd.Vec[i32, N] { v.add(v) }
fn relay[const N: usize](v: simd.Vec[i32, N]) -> simd.Vec[i32, N] { twice(v) }
fn main() { print "${relay(simd.Vec.splat[i32, 4](3)).lane(0)}" }
`

func simdClaims(prog *mir.Program) []mirverify.AllocClaim {
	return mirverify.CheckSimdShapes(prog, mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt))
}

func TestCheckSimdShapesNamesTheSharedBody(t *testing.T) {
	claims := simdClaims(mirfixture.Planned(t, simdSharedSrc))
	if len(claims) != 1 {
		t.Fatalf("claims = %+v, want one", claims)
	}
	d := claims[0].Diag
	if d.Code != "E1713" || !strings.Contains(d.Msg, "`twice` is reached as one shared body") || !strings.Contains(d.Msg, "`Vec[i32, N]`") {
		t.Fatalf("diagnostic %s %q", d.Code, d.Msg)
	}
	var notes []string
	for _, n := range d.Notes {
		notes = append(notes, n.Msg)
	}
	if got := strings.Join(notes, "; "); got != "`relay` reaches it; `main` reaches it" {
		t.Fatalf("notes %q", got)
	}
}

func TestCheckSimdShapesAcceptsTheSpecializedProgram(t *testing.T) {
	prog := mirbuild.Program(mirfixture.CheckSourceMir(t, simdSharedSrc))
	passes := []mirplan.Transform{{Name: "plan-rc", Run: mirplan.RC}, {Name: "specialize", Run: mirplan.Specialize}, {Name: "plan-direct", Run: mirplan.PlanDirect}}
	if err := mirplan.Apply(prog, passes...); err != nil {
		t.Fatal(err)
	}
	if claims := simdClaims(prog); len(claims) != 0 {
		t.Fatalf("claims = %+v, want none", claims)
	}
}
