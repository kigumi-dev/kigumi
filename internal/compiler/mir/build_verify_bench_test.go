package mir_test

import (
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
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
	for _, p := range testkit.LoadStd(b, "../../../std") {
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
	res := sem.CheckWith(mod, schema.GenerateDerive)
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

// benchMIRBorrowUnit stresses unownedRefs: a `&` parameter and a field read
// into a named local are the shapes its fixed point must walk every block for.
const benchMIRBorrowUnit = `
pub type Box%d = {
    pub mut n Int
}

pub fn Box%d.bump(mut self, k: Int) {
    self.n = self.n + k
}

pub type Holder%d['a] = {
    pub mut r &'a mut Box%d
}

fn bumpLent%d(r: &mut Box%d) {
    r.bump(3)
}

fn useField%d['a](h: Holder%d['a]) {
    let r = h.r
    r.bump(2)
    bumpLent%d(r)
}
`

func genMIRBorrowBenchModule(b *testing.B, count int) *sem.Result {
	b.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range testkit.LoadStd(b, "../../../std") {
		pkgs[p.Path] = p
	}
	var sb strings.Builder
	for i := 0; i < count; i++ {
		sb.WriteString(strings.ReplaceAll(benchMIRBorrowUnit, "%d", strconv.Itoa(i)))
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
	return res
}

// BenchmarkBuildVerifyBorrows is BenchmarkBuildVerify over a module that
// actually exercises unownedRefs, unlike benchMIRUnit (no `&T` anywhere).
func BenchmarkBuildVerifyBorrows(b *testing.B) {
	res := genMIRBorrowBenchModule(b, 20)
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
