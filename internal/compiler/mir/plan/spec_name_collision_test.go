package plan

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

func nameSpecs(res *sem.Result, order []string) []string {
	pkg := res.PackageByPath("main")
	s := &specializer{p: &planner{Program: &mir.Program{R: res}}, names: map[string]*nameHolder{}}
	var funcs []*midir.Func
	for _, name := range order {
		wit := []sem.WitnessArg{{Kind: sem.WitnessFn, Ent: res.PackageMember(pkg, name)}}
		n, h := s.instanceName("main.id", nil, wit)
		h.fn = &midir.Func{Name: n}
		funcs = append(funcs, h.fn)
	}
	var out []string
	for _, f := range funcs {
		out = append(out, f.Name)
	}
	slices.Sort(out)
	return out
}

func TestInstanceNamesDifferingInWitnessesDoNotDependOnOrder(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte("fn f() -> Int { 1 }\nfn g() -> Int { 2 }\n")))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	a, b := nameSpecs(res, []string{"f", "g"}), nameSpecs(res, []string{"g", "f"})
	want := []string{"main.id[]<main.f>", "main.id[]<main.g>"}
	if !slices.Equal(a, want) || !slices.Equal(b, want) {
		t.Errorf("names = %v and %v, want %v both ways", a, b, want)
	}
}
