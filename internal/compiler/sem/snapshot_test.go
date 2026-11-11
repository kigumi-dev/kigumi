package sem_test

import (
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func snapshotModule(t *testing.T, srcs map[string]string, order []string) *sem.Result {
	t.Helper()
	std := semfixture.LoadStd(t, "../../../std")
	files := make([]*syntax.Tree, 0, len(order))
	for _, name := range order {
		files = append(files, syntax.Parse(token.NewFile("main/"+name, []byte(srcs[name]))))
	}
	main := &sem.Package{Path: "main", Files: files}
	return sem.CheckWith(&sem.Module{Packages: append(append([]*sem.Package(nil), std...), main), Name: "demo"}, schema.GenerateDerive)
}

var twoFiles = map[string]string{
	"alpha.kg": "fn alpha(n: Int) -> Int { n + 1 }\n",
	"beta.kg":  "fn beta(n: Int) -> Int { n + 2 }\n",
}

// The module and package ids follow content, not file enumeration order.
func TestAnalysisIDsIgnoreFileOrder(t *testing.T) {
	t.Parallel()
	first := snapshotModule(t, twoFiles, []string{"alpha.kg", "beta.kg"})
	second := snapshotModule(t, twoFiles, []string{"beta.kg", "alpha.kg"})
	if first.HasErrors() || second.HasErrors() {
		t.Fatalf("unexpected errors:\n%s\n%s", first.Render(), second.Render())
	}
	if first.ModuleID() != second.ModuleID() {
		t.Fatalf("module id moved with file order: %q != %q", first.ModuleID(), second.ModuleID())
	}
	if first.PackageKey(first.PackageByPath("main")) != second.PackageKey(second.PackageByPath("main")) {
		t.Fatal("package id moved with file order")
	}
	if first.PackageKey(first.PackageByPath("main")) == "" {
		t.Fatal("package id is empty")
	}
}

// An edit changes the module and its package, but no sibling package.
func TestAnalysisIDsFollowContent(t *testing.T) {
	t.Parallel()
	base := snapshotModule(t, map[string]string{"main.kg": "fn a(n: Int) -> Int { n + 1 }\n"}, []string{"main.kg"})
	edited := snapshotModule(t, map[string]string{"main.kg": "fn a(n: Int) -> Int { n + 2 }\n"}, []string{"main.kg"})
	if base.ModuleID() == edited.ModuleID() {
		t.Fatal("an edit must change the module id")
	}
	if base.PackageKey(base.PackageByPath("main")) == edited.PackageKey(edited.PackageByPath("main")) {
		t.Fatal("an edit must change the package id")
	}
	if base.PackageKey(base.PackageByPath("std/mem")) != edited.PackageKey(edited.PackageByPath("std/mem")) {
		t.Fatal("an unrelated standard package id must not change")
	}
}

// A published snapshot owns its diagnostics and refuses builder appends.
func TestSnapshotIsImmutable(t *testing.T) {
	t.Parallel()
	res := snapshotModule(t, map[string]string{"main.kg": "fn a() -> Int { \"x\" }\n"}, []string{"main.kg"})
	if !res.HasErrors() {
		t.Fatal("fixture must fail to check")
	}
	snap := res.Snapshot()
	if !res.Sealed() {
		t.Fatal("Snapshot must seal the result")
	}
	diags := snap.Diagnostics()
	before := len(diags)
	if before == 0 {
		t.Fatal("fixture must have a diagnostic")
	}
	diags[0].Msg = "mutated"
	if got := snap.Diagnostics(); got[0].Msg == "mutated" {
		t.Fatal("a caller's diagnostic mutation reached the snapshot")
	}
	snap.Packages()[0].Path = "mutated"
	if snap.Packages()[0].Path == "mutated" {
		t.Fatal("a caller mutated the published package list")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("AppendDiag after publish must panic")
		}
	}()
	res.AppendDiag(1, diag.Diagnostic{Severity: diag.Error, Msg: "late"})
}
