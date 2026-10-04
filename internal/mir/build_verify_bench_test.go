package mir_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

// benchMIRUnit mirrors the parser/checker benchmarks' unit.
const benchMIRUnit = `
pub type Point%d = {
    pub x Int
    pub y Int
}

pub fn Point%d.len(self) -> Int {
    self.x * self.x + self.y * self.y
}

fn classify%d(n: Int) -> String {
    match n {
        0 -> "zero"
        _ -> if n < 0 { "negative" } else { "positive" }
    }
}

fn sum%d(xs: Array[Int]) -> Int {
    let mut total = 0
    for x in xs {
        total = total + x
    }
    total
}

fn makeAdder%d(n: Int) -> fn(Int) -> Int {
    (m: Int) => m + n
}
`

func genMIRBenchModule(b *testing.B, count int) *sem.Result {
	b.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range testkit.LoadStd(b, "../../std") {
		pkgs[p.Path] = p
	}
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(strings.ReplaceAll(benchMIRUnit, "%d", strconv.Itoa(i)))
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(sb.String())))
	if tree.HasErrors() {
		b.Fatalf("parse errors: %d diagnostics", len(tree.Diags))
	}
	pkgs["main"] = &sem.Package{Path: "main", Files: []*syntax.Tree{tree}, Entry: tree}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	mod := &sem.Module{}
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	res := sem.Check(mod)
	if res.HasErrors() {
		b.Fatalf("check errors:\n%s", res.Render())
	}
	return res
}

// BenchmarkBuildVerify lowers a representative module (20 repeats of
// benchMIRUnit) to MIR and runs its ownership verifier.
func BenchmarkBuildVerify(b *testing.B) {
	res := genMIRBenchModule(b, 20)
	b.ReportAllocs()
	for b.Loop() {
		prog := mir.Build(res)
		if prog.HasErrors() {
			b.Fatal("mir.Build reported errors on a module that already type-checked")
		}
		if err := prog.Verify(); err != nil {
			b.Fatalf("verify: %v", err)
		}
	}
}
