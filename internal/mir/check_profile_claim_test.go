package mir_test

import (
	"testing"

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

// The profile accepts the generic call on the promise an instance takes the Option unboxed; without one, the box is reported.
func TestProfileClaimsReportACrossingTheProfileTrusted(t *testing.T) {
	for _, c := range []struct {
		mod  sem.Module
		code string
	}{{sem.Module{RecoverableOnly: true}, "E2313"}, {sem.Module{NoDefaultAllocator: true}, "E2314"}} {
		main := &sem.Package{Path: "main"}
		main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(profileCrossSrc)))
		main.Files = []*syntax.Tree{main.Entry}
		mod := c.mod
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
		var codes []string
		for _, claim := range mir.CheckAllocClaims(prog) {
			codes = append(codes, claim.Diag.Code)
		}
		if len(codes) != 1 || codes[0] != c.code {
			t.Errorf("claims %v, want one %s for the box the unspecialized call needs", codes, c.code)
		}
	}
}
