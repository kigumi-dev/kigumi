package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func checkSimd(t *testing.T, src string) *sem.Result {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	return sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main)}, schema.GenerateDerive)
}

// A Vec is one 128-bit register: a shape that is not 128 bits or a lane type
// no target vector holds is rejected where it is instantiated, not by a
// backend failing later.
func TestSimdRejectsUnsupportedVectorShapes(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"too many lanes", "simd.Vec.splat[i32, 5](1)", "160 bits"},
		{"too few lanes", "simd.Vec.splat[i32, 3](1)", "96 bits"},
		{"i128 lane", "simd.Vec.splat[i128, 1](1)", "not a lane type"},
		{"bool lane", "simd.Vec.splat[Bool, 16](true)", "not a lane type"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := checkSimd(t, "import simd from std/simd\n\nfn main() {\n    let _ = "+c.src+"\n}\n")
			out := res.Render()
			if !res.HasErrors() || !strings.Contains(out, "E976") || !strings.Contains(out, c.want) {
				t.Fatalf("check accepted %s:\n%s", c.src, out)
			}
		})
	}
}

// The four widths and both float lanes a 128-bit register holds are accepted.
func TestSimdAccepts128BitVectors(t *testing.T) {
	for _, src := range []string{
		"simd.Vec.splat[i8, 16](1)",
		"simd.Vec.splat[u16, 8](1)",
		"simd.Vec.splat[i32, 4](1)",
		"simd.Vec.splat[u64, 2](1)",
		"simd.Vec.splat[f32, 4](1.0)",
		"simd.Vec.splat[f64, 2](1.0)",
	} {
		t.Run(src, func(t *testing.T) {
			res := checkSimd(t, "import simd from std/simd\n\nfn main() {\n    let _ = "+src+"\n}\n")
			if res.HasErrors() {
				t.Fatalf("check rejected %s:\n%s", src, res.Render())
			}
		})
	}
}
