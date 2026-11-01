package llgen

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// benchEmitUnit mirrors the parser/checker/MIR benchmarks' unit: a record, method, match, loop and closure.
const benchEmitUnit = `
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

func buildEmitBenchProgram(b *testing.B, count int) *mir.Program {
	b.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range semfixture.LoadStd(b, "../../../std") {
		pkgs[p.Path] = p
	}
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(strings.ReplaceAll(benchEmitUnit, "%d", strconv.Itoa(i)))
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
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if res.HasErrors() {
		b.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
		b.Fatalf("plan-rc: %v", err)
	}
	return prog
}

func BenchmarkEmitProgram(b *testing.B) {
	prog := buildEmitBenchProgram(b, 20)
	b.ReportAllocs()
	for b.Loop() {
		out := EmitProgram(prog, EmitOptions{})
		if out.IR == "" {
			b.Fatal("EmitProgram produced no IR")
		}
	}
}
