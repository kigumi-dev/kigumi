package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A plain fn(T) -> T callback may receive a place read through &mut, but it
// must not be modeled as returning a fresh own borrow of the caller's place.
func TestCallbackPlaceReadDoesNotPinBorrow(t *testing.T) {
	root := t.TempDir()
	const src = `type Ops[T: Copy] = {
    g fn(T) -> T,
    idf T,
}
type L[T: Copy] = {
    mut size usize,
    mut d Array[T],
}

fn allApply[T: Copy](t: &mut L[T], k: usize, f: T, ops: &Ops[T]) {
    t.d[k] = (ops.g)(t.d[k])
}

fn push[T: Copy](t: &mut L[T], k: usize, ops: &Ops[T]) {
    let f = t.d[0]
    allApply(t, k, f, ops)
    t.d[1] = ops.idf
}

fn main() {
    let ops = Ops[Int] { g: (v) => v + 1, idf: 7 }
    let mut l = L[Int] { size: 2, d: Array.of(1, 2) }
    push(&mut l, 0, &ops)
    print "${l.d[0]} ${l.d[1]}"
}
`
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"alg1\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	a, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("callback place read was rejected:\n%s", a.Res.Render())
	}
	if strings.Contains(a.Res.Render(), "E2198") {
		t.Fatalf("unexpected borrow conflict:\n%s", a.Res.Render())
	}
}
