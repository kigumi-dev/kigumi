package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/target"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// plannedProgramFor is plannedProgram checked for a non-host target: the
// checker's pointer width and 64-bit alignment come from l, and llgen gets
// the same layout.
func plannedProgramFor(t *testing.T, l target.Layout, src string) *mir.Program {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	mod := &sem.Module{
		Packages:     append(semfixture.LoadStd(t, "../../../std"), main),
		IsrContracts: opdef.IsrContracts,
		PtrBits:      l.PtrBits,
		Align64:      l.CAlign(target.CLongLong),
	}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
		t.Fatal(err)
	}
	return prog
}

const aggLimitSrc = `
type Pair = {
    pub a u64
    pub b u64
}

fn take(p: Pair) -> u64 { p.a + p.b }

fn main() -> Void! {
    let p = Pair { a: 1, b: 2 }
    print "${take(p)}"
}
`

// TestDirectAggregateLimitFollowsTarget checks the direct ABI's by-value
// limit comes from the layout: a 16-byte record is two 64-bit machine words
// on x86-64 and stays first-class, while on a 32-bit target (two 32-bit
// words) or under the Microsoft x64 convention (8-byte aggregates) it
// travels by pointer.
func TestDirectAggregateLimitFollowsTarget(t *testing.T) {
	for _, c := range []struct {
		name    string
		layout  target.Layout
		byValue bool
	}{
		{"x86_64-linux", target.ForArch("linux", "amd64"), true},
		{"aarch64-linux", target.ForArch("linux", "arm64"), true},
		{"riscv64-linux", target.ForArch("linux", "riscv64"), true},
		{"x86-linux", target.ForArch("linux", "386"), false},
		{"wasm32-wasi", target.ForArch("wasi", "wasm32"), false},
		{"x86_64-windows", target.ForArch("windows", "amd64"), false},
	} {
		t.Run(c.name, func(t *testing.T) {
			prog := plannedProgramFor(t, c.layout, aggLimitSrc)
			ir := llgen.EmitWith(prog, llgen.EmitOptions{Layout: c.layout})
			body := bodyOf(t, ir, `@"main.take"(`)
			param := body[strings.Index(body, `@"main.take"(`)+len(`@"main.take"(`):]
			param = param[:strings.Index(param, "\n")]
			if c.byValue {
				if !strings.Contains(param, "kg.agg.main.Pair") {
					t.Errorf("a 16-byte aggregate should pass by value:\n%s", param)
				}
			} else if !strings.HasPrefix(param, "ptr ") {
				t.Errorf("a 16-byte aggregate should pass by pointer on this target:\n%s", param)
			}
		})
	}
}
