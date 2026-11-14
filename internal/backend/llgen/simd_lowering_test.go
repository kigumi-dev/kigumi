package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/target"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
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

const simdGenericSrc = `
import simd from std/simd

fn addV[const N: usize](a: simd.Vec[i32, N], b: simd.Vec[i32, N]) -> simd.Vec[i32, N] {
    a.add(b)
}

fn makeV[const N: usize](v: u8) -> simd.Vec[u8, N] {
    simd.Vec.splat[u8, N](v)
}

fn main() {
    let a = simd.Vec.splat[i32, 4](1)
    let b = simd.Vec.splat[i32, 4](2)
    print "${addV[4](a, b).lane(0)} ${makeV[16](7).lane(3)}"
}
`

// plannedProgramSpecialized runs the checker and the whole planning pipeline,
// including the specializer a build runs before plan-direct.
func plannedProgramSpecialized(t *testing.T, src string) *mir.Program {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../std"), main), IsrContracts: opdef.IsrContracts}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if err := mirplan.Apply(prog,
		mirplan.Transform{Name: "plan-rc", Run: mirplan.RC},
		mirplan.Transform{Name: "specialize", Run: mirplan.Specialize},
		mirplan.Transform{Name: "elide-rc", Run: mirplan.ElideRC},
		mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
		t.Fatal(err)
	}
	return prog
}

// TestSimdGenericWrapperIsInstantiated pins the fix for a const-generic
// wrapper: the shape is only concrete at the call that supplies N, so the
// specializer must clone the body and llgen must lower that clone to a vector.
// The shared body, which still names Vec[T, N], is unreachable and never
// emitted, so it cannot trip the "no vector shape" check.
func TestSimdGenericWrapperIsInstantiated(t *testing.T) {
	prog := plannedProgramSpecialized(t, simdGenericSrc)
	instantiated := map[string]bool{}
	for _, f := range prog.Funcs {
		instantiated[f.Name] = true
	}
	for _, want := range []string{"main.addV[4]", "main.makeV[16]"} {
		if !instantiated[want] {
			t.Fatalf("no instantiation %s in %v", want, len(instantiated))
		}
	}
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)
	ir := llgen.EmitWith(prog, llgen.EmitOptions{Reachable: rs})

	add := fnBody(t, ir, "main.addV[4]")
	for _, want := range []string{"load <4 x i32>, ptr", "add <4 x i32>", "store <4 x i32>"} {
		if !strings.Contains(add, want) {
			t.Errorf("instantiated addV[4] lacks %q:\n%s", want, add)
		}
	}
	make := fnBody(t, ir, "main.makeV[16]")
	if !strings.Contains(make, "insertelement <16 x i8>") {
		t.Errorf("instantiated makeV[16] does not splat a 128-bit vector:\n%s", make)
	}
	for _, bad := range []string{`@"main.addV"(`} {
		if strings.Contains(ir, bad) {
			t.Errorf("the shared generic body was emitted:\n%s", ir)
		}
	}
	for _, bad := range []string{"@rt_std_id(", `@"rt_std"(`} {
		if strings.Contains(add, bad) {
			t.Errorf("instantiated addV[4] went through %s:\n%s", bad, add)
		}
	}
}
