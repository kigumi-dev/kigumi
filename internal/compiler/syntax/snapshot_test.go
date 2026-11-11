package syntax_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

func file(name, src string) *token.File { return token.NewFile(name, []byte(src)) }

func TestParseSnapshotIDsIgnoreOrder(t *testing.T) {
	t.Parallel()
	a := file("a.kg", "fn a() {}")
	b := file("b.kg", "fn b() {}")
	first := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo", Files: []*token.File{a, b}},
		{Path: "demo/sub", Files: []*token.File{file("z.kg", "fn z() {}")}},
	})
	second := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo/sub", Files: []*token.File{file("z.kg", "fn z() {}")}},
		{Path: "demo", Files: []*token.File{b, a}},
	})
	if first.ModuleID() != second.ModuleID() {
		t.Fatalf("module id moved with enumeration order: %q != %q", first.ModuleID(), second.ModuleID())
	}
	for _, path := range []string{"demo", "demo/sub"} {
		want, _ := first.PackageID(path)
		got, ok := second.PackageID(path)
		if !ok || got != want {
			t.Errorf("package %q id moved: %q != %q", path, got, want)
		}
	}
	fid1, ok1 := first.FileID("demo", "a.kg")
	fid2, ok2 := second.FileID("demo", "a.kg")
	if !ok1 || !ok2 || fid1 != fid2 {
		t.Errorf("file id moved: %q != %q", fid1, fid2)
	}
}

func TestParseSnapshotIDsFollowContent(t *testing.T) {
	t.Parallel()
	base := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo", Files: []*token.File{file("a.kg", "fn a() {}")}},
		{Path: "demo/other", Files: []*token.File{file("b.kg", "fn b() {}")}},
	})
	edited := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo", Files: []*token.File{file("a.kg", "fn a() { 1 }")}},
		{Path: "demo/other", Files: []*token.File{file("b.kg", "fn b() {}")}},
	})
	if base.ModuleID() == edited.ModuleID() {
		t.Fatal("a content edit must change the module id")
	}
	baseID, _ := base.PackageID("demo")
	editID, _ := edited.PackageID("demo")
	otherBase, _ := base.PackageID("demo/other")
	otherEdit, _ := edited.PackageID("demo/other")
	if baseID == editID {
		t.Error("the edited package id must change")
	}
	if otherBase != otherEdit {
		t.Error("a sibling package id must not change")
	}
	added := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo", Files: []*token.File{file("a.kg", "fn a() {}")}},
		{Path: "demo/other", Files: []*token.File{file("b.kg", "fn b() {}")}},
		{Path: "demo/third", Files: []*token.File{file("c.kg", "fn c() {}")}},
	})
	if base.ModuleID() == added.ModuleID() {
		t.Error("adding a package must change the module id")
	}
	if id, _ := added.PackageID("demo"); id != baseID {
		t.Error("adding a sibling package must not change an existing package id")
	}
}

func TestParseSnapshotAccessorsAreCopies(t *testing.T) {
	t.Parallel()
	snap := syntax.NewParseSnapshot("demo", []syntax.ParsePackage{
		{Path: "demo", Files: []*token.File{file("a.kg", "fn a() {}")}},
	})
	pkgs := snap.Packages()
	pkgs[0].Path = "mutated"
	pkgs[0].Files[0] = nil
	if got := snap.Packages(); got[0].Path != "demo" || got[0].Files[0] == nil {
		t.Fatalf("a caller mutated the snapshot: %+v", got)
	}
	if got := snap.Packages(); !slices.IsSortedFunc(got, func(a, b syntax.ParsePackage) int {
		if a.Module < b.Module {
			return -1
		}
		if a.Module > b.Module {
			return 1
		}
		if a.Path < b.Path {
			return -1
		}
		if a.Path > b.Path {
			return 1
		}
		return 0
	}) {
		t.Fatalf("packages are not in a stable order: %+v", got)
	}
}
