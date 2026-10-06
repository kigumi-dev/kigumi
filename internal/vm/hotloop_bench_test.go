package vm_test

import (
	"io"
	"sort"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
	"kigumi/internal/vm"
)

// hotLoopSrc pushes onto an Array repeatedly: std/array.push has a Go accelerator, toggled off by NoAccel.
const hotLoopSrc = `
let mut total = 0
for i in 0..2000 {
    let mut xs = Array.of(0)
    for j in 0..50 {
        xs.push(j)
    }
    total = total + (xs.len().toInt() || 0)
}
print "${total}"
`

// buildRunnableProgramTB is buildRunnableProgram over testing.TB, so a *testing.B can share it.
func buildRunnableProgramTB(tb testing.TB, src string) *mir.Program {
	tb.Helper()
	pkgs := map[string]*sem.Package{}
	for _, p := range testkit.LoadStd(tb, "../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	if tree.HasErrors() {
		for _, d := range tree.Diags {
			tb.Logf("%s", d.Msg)
		}
		tb.Fatalf("parse errors: %d diagnostics", len(tree.Diags))
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
		tb.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		tb.Fatalf("plan-rc: %v", err)
	}
	return prog
}

// BenchmarkVMHotLoopAccelerated and BenchmarkVMHotLoopReference run the same hot loop with std/array.push's accelerator on and off.
func BenchmarkVMHotLoopAccelerated(b *testing.B) { benchmarkHotLoop(b, false) }
func BenchmarkVMHotLoopReference(b *testing.B)   { benchmarkHotLoop(b, true) }

func benchmarkHotLoop(b *testing.B, noAccel bool) {
	prog := buildRunnableProgramTB(b, hotLoopSrc)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := vm.Run(prog, vm.Options{NoAccel: noAccel}, io.Discard, io.Discard); err != nil {
			b.Fatalf("run: %v", err)
		}
	}
}
