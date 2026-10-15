package sem_test

import (
	"slices"
	"strings"
	"testing"

	"kigumi/internal/sem"
)

const declSrcA = `
type Point = { x: Int, y: Int }
type Shape = Circle(r: Int) | Dot
interface Sized {
    fn size(self) -> Int
}
fn Point.size(self) -> Int { self.x + self.y }
fn show(x: Int) -> String { "${x}" }
fn show(x: String) -> String { x }
fn pair[T](a: T, b: T) -> T { a }
let k = 3
let f = (v: Int) => v + k
let g = (v: Int) => v * k
`

const declSrcB = `
let k = 3
let g = (v: Int) => v * k
let f = (v: Int) => v + k
fn pair[T](a: T, b: T) -> T { a }
fn show(x: String) -> String { x }
fn show(x: Int) -> String { "${x}" }
fn Point.size(self) -> Int { self.x + self.y }
interface Sized {
    fn size(self) -> Int
}
type Shape = Circle(r: Int) | Dot
type Point = { x: Int, y: Int }
`

func declIDs(t *testing.T, src string) []string {
	t.Helper()
	res := checkRepr(t, src)
	pkg := res.PackageByPath("main")
	var out []string
	for id := 1; id < len(res.Entities); id++ {
		if res.Entity(sem.EntityID(id)).Pkg != pkg {
			continue
		}
		if d := res.DeclID(sem.EntityID(id)); !d.IsZero() {
			out = append(out, d.String())
		}
	}
	slices.Sort(out)
	return out
}

func TestDeclIDIgnoresDeclarationOrder(t *testing.T) {
	a, b := declIDs(t, declSrcA), declIDs(t, declSrcB)
	if !slices.Equal(a, b) {
		t.Fatalf("DeclIDs depend on declaration order:\n%v\n%v", a, b)
	}
	for _, want := range []string{
		"main.Point", "main.Point.x", "main.Point.size", "main.Shape.Circle", "main.Sized.size",
		"main.show#(Int)", "main.show#(String)", "main.pair", "main.main.main/main.kg.closure1", "main.main.main/main.kg.closure2",
	} {
		if !slices.Contains(a, want) {
			t.Errorf("no DeclID %q in %v", want, a)
		}
	}
}

func TestClosureOrdinalFollowsSourceOrder(t *testing.T) {
	res := checkRepr(t, declSrcB)
	pkg := res.PackageByPath("main")
	tok := map[string]uint32{}
	for id := 1; id < len(res.Entities); id++ {
		e := res.Entity(sem.EntityID(id))
		if e.Pkg == pkg && e.Kind == sem.EntClosure {
			tok[res.DeclID(sem.EntityID(id)).String()] = e.Tok
		}
	}
	if len(tok) != 2 || tok["main.main.main/main.kg.closure1"] >= tok["main.main.main/main.kg.closure2"] {
		t.Errorf("closure ordinals do not follow source order: %v", tok)
	}
}

func TestTypeKeyAndWitnessID(t *testing.T) {
	res := checkRepr(t, declSrcA)
	point := mainType(t, res, "Point")
	arr := res.Types.Named(res.EntityOfName("Array"), []sem.TypeID{point})
	if got := res.TypeKey(arr); got != "Array[main.Point]" {
		t.Errorf("TypeKey(Array[Point]) = %q", got)
	}
	fn := res.Types.Fn([]sem.TypeID{point}, sem.TyI64, sem.EffPure, false)
	if got := res.TypeKey(fn); got != "pure fn(main.Point)->Int" {
		t.Errorf("TypeKey(fn) = %q", got)
	}
	sized := res.Types.Iface(res.PackageMember(res.PackageByPath("main"), "Sized"), nil)
	if got := res.WitnessID(point, sized); got != "main.Point : main.Sized" {
		t.Errorf("WitnessID = %q", got)
	}
}

const declSrcOverloads = `
interface A {
    fn a(self) -> Int
}
interface B {
    fn b(self) -> Int
}
fn f[T: A](x: T) -> Int { x.a() }
fn f[T: B](x: T) -> Int { x.b() }
fn h(x: Int) -> Int {
    let g = (y: Int) => y + x
    g(1)
}
fn h(x: String) -> Int {
    let g = (y: Int) => y
    g(1)
}
`

func TestDeclIDSeparatesOverloadsByBoundsAndClosureParent(t *testing.T) {
	ids := declIDs(t, declSrcOverloads)
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			t.Errorf("duplicate DeclID %q in %v", id, ids)
		}
		seen[id] = true
	}
	var fs, closures int
	for _, id := range ids {
		switch {
		case strings.HasPrefix(id, "main.f#"):
			fs++
		case strings.Contains(id, ".closure1#"):
			closures++
		}
	}
	if fs != 2 || closures != 2 {
		t.Errorf("want 2 constrained overloads and 2 overload-qualified closures, got %d and %d in %v", fs, closures, ids)
	}
}
