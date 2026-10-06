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
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
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

// Handing an unboxed aggregate to a type parameter's reference slot
// allocates a box, which a declared `noalloc` does not cover.
func TestNoallocRejectsABoxingSite(t *testing.T) {
	t.Parallel()
	out := checkAllocClaim(t, directCallbackSrc)
	if !strings.Contains(out, "E1570") {
		t.Fatalf("expected E1570 direct-box-in-noalloc, got:\n%s", out)
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
