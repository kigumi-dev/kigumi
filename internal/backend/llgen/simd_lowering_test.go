package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/lang/target"
)

const simdSrc = `
import simd from std/simd

fn lanes(a: simd.Vec[i32, 4], b: simd.Vec[i32, 4]) -> simd.Vec[i32, 4] {
    a.add(b).mul(b).sub(a)
}

fn pairs(a: simd.Vec[f64, 2], b: simd.Vec[f64, 2]) -> simd.Vec[f64, 2] {
    a.add(b).mul(b)
}

fn main() {
    let a = simd.Vec.splat[i32, 4](3)
    let b = simd.Vec.splat[i32, 4](4)
    let c = lanes(a, b)
    let d = pairs(simd.Vec.splat[f64, 2](1.5), simd.Vec.splat[f64, 2](2.0))
    let f = simd.Vec.fromArray(FixedArray.filled[u32, 4](5)).mul(simd.Vec.splat[u32, 4](2))
    print "${c.lane(0)} ${d.lane(1)} ${f.toArray()[0]}"
}
`

// TestSimdLowersToVectorInstructions pins the 128-bit path: one insertelement
// per splat lane, vector loads and stores, and one add/mul/sub per operation.
func TestSimdLowersToVectorInstructions(t *testing.T) {
	body := bodyOf(t, emitIR(t, simdSrc), `@"main.lanes"(`)
	for _, want := range []string{
		"load <4 x i32>, ptr", "add <4 x i32>", "mul <4 x i32>", "sub <4 x i32>",
		"store <4 x i32>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("i32x4 body lacks %q:\n%s", want, body)
		}
	}
	pairs := bodyOf(t, emitIR(t, simdSrc), `@"main.pairs"(`)
	for _, want := range []string{"load <2 x double>, ptr", "fadd <2 x double>", "fmul <2 x double>", "store <2 x double>"} {
		if !strings.Contains(pairs, want) {
			t.Errorf("f64x2 body lacks %q:\n%s", want, pairs)
		}
	}
	splat := bodyOf(t, emitIR(t, simdSrc), `@"main.main"(`)
	if !strings.Contains(splat, "insertelement <4 x i32>") || !strings.Contains(splat, "insertelement <2 x double>") {
		t.Errorf("splat does not build a 128-bit vector:\n%s", splat)
	}
	for _, bad := range []string{"@rt_std_id(", `@"rt_std"(`} {
		if strings.Contains(splat, bad) {
			t.Errorf("simd went through %s:\n%s", bad, splat)
		}
	}
}

// TestSimdScalarFallback keeps the same program correct on a target without
// 128-bit vectors: the lanes are touched one at a time, never as <N x T>.
func TestSimdScalarFallback(t *testing.T) {
	ir := llgen.EmitWith(plannedProgram(t, simdSrc), llgen.EmitOptions{Layout: target.ForArch("linux", "riscv64")})
	body := bodyOf(t, ir, `@"main.lanes"(`)
	if strings.Contains(body, "<4 x i32>") {
		t.Errorf("riscv64 uses a 128-bit vector:\n%s", body)
	}
	for _, want := range []string{"add i32", "mul i32", "sub i32"} {
		if !strings.Contains(body, want) {
			t.Errorf("scalar fallback lacks %q:\n%s", want, body)
		}
	}
	main := bodyOf(t, ir, `@"main.main"(`)
	if strings.Contains(main, "<4 x i32>") || strings.Contains(main, "<2 x double>") {
		t.Errorf("riscv64 main uses a 128-bit vector:\n%s", main)
	}
	if !strings.Contains(main, "getelementptr") {
		t.Errorf("fromArray does not copy lanes one at a time:\n%s", main)
	}
}
