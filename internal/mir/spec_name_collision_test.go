package mir

import (
	"slices"
	"testing"

	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

func nameSpecs(res *sem.Result, order []string) []string {
	pkg := res.PackageByPath("main")
	s := &specializer{p: &Program{R: res}, names: map[string]*nameHolder{}}
	var funcs []*Func
	for _, name := range order {
		wit := []sem.WitnessArg{{Kind: sem.WitnessFn, Ent: res.PackageMember(pkg, name)}}
		n, h := s.instanceName("main.id", nil, wit)
		h.fn = &Func{Name: n}
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
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatal(res.Render())
	}
	a, b := nameSpecs(res, []string{"f", "g"}), nameSpecs(res, []string{"g", "f"})
	want := []string{"main.id[]<main.f>", "main.id[]<main.g>"}
	if !slices.Equal(a, want) || !slices.Equal(b, want) {
		t.Errorf("names = %v and %v, want %v both ways", a, b, want)
	}
}
