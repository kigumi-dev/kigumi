package sem_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

// benchModuleUnit mirrors the parser benchmark's unit.
const benchModuleUnit = `
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

func genModuleSource(count int) string {
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(strings.ReplaceAll(benchModuleUnit, "%d", strconv.Itoa(i)))
	}
	return sb.String()
}

// buildBenchModule is buildModule without the fixture file: a synthetic "main" package over the real std.
func buildBenchModule(b *testing.B, size int) *sem.Module {
	b.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range semfixture.LoadStd(b, "../../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(genModuleSource(size))))
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
	return mod
}

// BenchmarkCheckModule type-checks a representative module over std (20
// repeats of benchModuleUnit, a few KiB).
func BenchmarkCheckModule(b *testing.B) {
	mod := buildBenchModule(b, 20)
	b.ReportAllocs()
	for b.Loop() {
		res := sem.CheckWith(mod, schema.GenerateDerive)
		if res.HasErrors() {
			b.Fatalf("check errors:\n%s", res.Render())
		}
	}
}
