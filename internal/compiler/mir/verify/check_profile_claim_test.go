package verify_test

import (
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirreach "kigumi/internal/compiler/mir/reach"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const profileCrossSrc = `
fn id[T](x: T) -> T {
    x
}

fn run(s: String) {
    let q: String? = s
    let r = id(q)
}

fn main() -> Void! {
    run("x")
}
`

func profiledProgram(t *testing.T, mod sem.Module, src string) *mir.Program {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	mod.Packages = append(semfixture.LoadStd(t, "../../../../std"), main)
	mod.IsrContracts = opdef.IsrContracts
	res := sem.CheckWith(&mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
		t.Fatal(err)
	}
	return prog
}

// The profile accepts the generic call on the promise an instance takes the Option unboxed; without one, the box is reported.
func TestProfileClaimsReportACrossingTheProfileTrusted(t *testing.T) {
	for _, c := range []struct {
		mod  sem.Module
		code string
	}{{sem.Module{RecoverableOnly: true}, "E2313"}, {sem.Module{NoDefaultAllocator: true}, "E2314"}} {
		prog := profiledProgram(t, c.mod, profileCrossSrc)
		prog.NativeBoxes = func(p *mir.Program) mir.NativeBoxSites {
			return llgen.ScalarBoxSites(p, llgen.EmitOptions{Reachable: mirreach.Reachable(p.R, p.Entry, p.Funcs, p.ByEnt)})
		}
		var codes []string
		for _, claim := range mirverify.CheckAllocClaims(prog, mir.DemoteBoxedLends(prog)) {
			codes = append(codes, claim.Diag.Code)
		}
		if len(codes) != 1 || codes[0] != c.code {
			t.Errorf("claims %v, want one %s for the box the unspecialized call needs", codes, c.code)
		}
	}
}

// Without the native lowering's boxes a profile check would pass a computed Int handed to a boxed parameter.
func TestProfileClaimsRefuseToRunWithoutTheNativeBoxes(t *testing.T) {
	prog := profiledProgram(t, sem.Module{RecoverableOnly: true}, profileCrossSrc)
	defer func() {
		if recover() == nil {
			t.Error("CheckAllocClaims ran a profile check with no NativeBoxes")
		}
	}()
	mirverify.CheckAllocClaims(prog, mir.DemoteBoxedLends(prog))
}
