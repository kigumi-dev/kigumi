package plan_test

import (
	"fmt"
	"strings"
	"testing"

	mirverify "kigumi/internal/compiler/mir/verify"
)

func shapeFanoutSource(levels int) string {
	var src strings.Builder
	src.WriteString("import simd from std/simd\n")
	for i := 0; i < levels-1; i++ {
		fmt.Fprintf(&src, "fn shapeG%d[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int { shapeG%d((x, 1), v) + shapeG%d((2, x), v) }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&src, "fn shapeG%d[T: Copy, const N: usize](x: T, v: simd.Vec[i32, N]) -> Int { v.add(v).lane(0).toInt() }\n", levels-1)
	src.WriteString("fn main() { print \"${shapeG0((1, 2), simd.Vec.splat[i32, 4](1))}\" }\n")
	return src.String()
}

func TestSpecializeSimdSharesUnrelatedTypeArguments(t *testing.T) {
	p := specialized(t, shapeFanoutSource(10))
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics: %+v", p.Diags)
	}
	if n := mintedFor(p.R, "shapeG"); n != 10 {
		t.Fatalf("instantiations=%d, want 10", n)
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}

func TestSpecializeSimdChainPastOptimizationBudget(t *testing.T) {
	var src strings.Builder
	src.WriteString("import simd from std/simd\n")
	for i := 0; i < 299; i++ {
		fmt.Fprintf(&src, "fn shapeG%d[const N: usize](v: simd.Vec[i32, N]) -> Int { shapeG%d(v) }\n", i, i+1)
	}
	src.WriteString("fn shapeG299[const N: usize](v: simd.Vec[i32, N]) -> Int { v.add(v).lane(0).toInt() }\n")
	src.WriteString("fn main() { print \"${shapeG0(simd.Vec.splat[i32, 4](1))}\" }\n")
	p := specialized(t, src.String())
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics: %+v", p.Diags)
	}
	if n := mintedFor(p.R, "shapeG"); n != 300 {
		t.Fatalf("instantiations=%d, want 300", n)
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}
