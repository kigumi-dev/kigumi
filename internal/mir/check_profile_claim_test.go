package mir_test

import (
	"testing"

	"kigumi/internal/llgen"
	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
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
	mod.Packages = append(testkit.LoadStd(t, "../../std"), main)
	mod.IsrContracts = mir.IsrContracts
	res := sem.CheckWith(&mod, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}, mir.Transform{Name: "plan-direct", Run: mir.PlanDirect}); err != nil {
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
			return llgen.ScalarBoxSites(p, llgen.EmitOptions{Reachable: mir.Reachable(p)})
		}
		var codes []string
		for _, claim := range mir.CheckAllocClaims(prog) {
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
	mir.CheckAllocClaims(prog)
}
