package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func checkAllocClaim(t *testing.T, main string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"allocclaim\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(main), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res.Render()
}

const directCallbackSrc = `type Point = {
    pub x Int
    pub y Int
}

noalloc fn keep[T](v: T) -> T {
    v
}

noalloc fn span(p: Point) -> Int {
    let q = keep(p)
    q.y - q.x
}

fn main() -> Unit! {
    print "${span(Point { x: 1, y: 2 })}"
}
`

func TestNoallocAcceptsAnInstantiatedCall(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, directCallbackSrc)
	if strings.Contains(out, "error") {
		t.Fatalf("expected the instantiated call to pass, got:\n%s", out)
	}
}

func TestNoallocRejectsABoxingSite(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Point = {
    pub x Int
    pub y Int
}

fn size[T](v: &T) -> Int { 1 }

noalloc fn lend[T](v: T) -> Int { size(&v) }

noalloc fn span(p: Point) -> Int {
    lend(p)
}

fn main() -> Unit! {
    print "${span(Point { x: 1, y: 2 })}"
}
`)
	if !strings.Contains(out, "E1570") || !strings.Contains(out, "`span` is declared `noalloc`") || strings.Contains(out, "E1610") {
		t.Fatalf("expected the box to be rejected by the built program's check, got:\n%s", out)
	}
}

// The same body without the promise compiles: the box is allowed, it is
// only the claim that was wrong.
func TestBoxingSiteWithoutTheClaimIsFine(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, strings.ReplaceAll(directCallbackSrc, "noalloc fn", "fn"))
	if strings.Contains(out, "error") {
		t.Fatalf("expected the same body without `noalloc` to pass, got:\n%s", out)
	}
}

// A `noalloc` body that builds an aggregate and reads it back never reaches
// a reference slot, so nothing is rejected.
func TestNoallocKeepsALocalAggregate(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Point = {
    pub x Int
    pub y Int
}

noalloc fn span(p: Point) -> Int {
    p.y - p.x
}

fn main() -> Unit! {
    print "${span(Point { x: 1, y: 2 })}"
}
`)
	if strings.Contains(out, "error") {
		t.Fatalf("expected a local aggregate to stay `noalloc`, got:\n%s", out)
	}
}

func TestNoallocAcceptsInstantiatedLiteralsAndClosures(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Point = {
    pub x Int
    pub y Int
}

fn ident[T](v: T) -> Int { 0 }

fn Point.keep[T](self, v: T) -> T { v }

noalloc fn applyGen[F: Fn[noalloc fn(Int) -> Bool]](f: F) -> Bool { f(1) }

noalloc fn literals() -> Int {
    ident(Some(1)) + ident((1, 2))
}

noalloc fn method(p: Point) -> Point {
    p.keep(p)
}

noalloc fn closure(n: Int) -> Bool {
    applyGen((x: Int) => x + n > 0)
}

fn main() -> Unit! {
    print "${literals()} ${method(Point { x: 1, y: 2 }).y} ${closure(2)}"
}
`)
	if strings.Contains(out, "error") {
		t.Fatalf("expected every instantiated call to pass, got:\n%s", out)
	}
}

func TestNoallocKeepsAWideRecursionInstantiated(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, `type Wide = {
    pub a Int
    pub b Int
    pub c Int
}

fn down[T](x: T, n: Int) -> T {
    if n == 0 { x } else { down(x, n - 1) }
}

fn outer[T](x: T, n: Int) -> T {
    down(x, n)
}

fn isEven[T](x: T, n: Int) -> Bool {
    if n == 0 { true } else { isOdd(x, n - 1) }
}

fn isOdd[T](x: T, n: Int) -> Bool {
    if n == 0 { false } else { isEven(x, n - 1) }
}

noalloc fn run(w: Wide, n: Int) -> Int {
    down(w, n).c
}

noalloc fn nested(w: Wide, n: Int) -> Int {
    outer(w, n).c
}

noalloc fn parity(w: Wide, n: Int) -> Bool {
    isEven(w, n)
}

fn main() -> Unit! {
    let w = Wide { a: 1, b: 2, c: 3 }
    print "${run(w, 4)} ${nested(w, 5)} ${parity(w, 7)}"
}
`)
	if out != "" {
		t.Fatalf("expected the noalloc callers to keep their instantiations, got:\n%s", out)
	}
}
