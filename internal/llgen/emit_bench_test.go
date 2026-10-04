package llgen

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
	for _, p := range testkit.LoadStd(b, "../../std") {
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
	res := sem.Check(mod)
	if res.HasErrors() {
		b.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
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
