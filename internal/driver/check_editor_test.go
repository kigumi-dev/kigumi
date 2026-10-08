package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
)

const deferredBoxSrc = `type Point = {
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
`

func editorModule(t *testing.T, src string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"editor\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestCheckForEditorReportsADeferredBox(t *testing.T) {
	t.Parallel()
	only, err := driver.CheckOnly(editorModule(t, deferredBoxSrc))
	if err != nil || only.HasErrors() || !only.DefersAllocClaims() {
		t.Fatalf("the checker alone should leave the crossing pending: %v\n%s", err, only.Render())
	}
	res, err := driver.CheckForEditor(editorModule(t, deferredBoxSrc))
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if strings.Count(out, "error[E1570]") != 1 || !strings.Contains(out, "`span` is declared `noalloc`") {
		t.Fatalf("expected the built program's E1570, got:\n%s", out)
	}
	if want := checkAllocClaim(t, deferredBoxSrc); out != want {
		t.Errorf("the editor says\n%s\nbut `kigumi check` says\n%s", out, want)
	}
	for id := range res.Entities {
		if _, minted := res.InstanceOf(sem.EntityID(id)); minted {
			t.Fatalf("the editor's result holds the minted instantiation %s", res.Entity(sem.EntityID(id)).Name)
		}
	}
}

func TestCheckForEditorAcceptsAnInstantiatedCall(t *testing.T) {
	t.Parallel()
	res, err := driver.CheckForEditor(editorModule(t, directCallbackSrc))
	if err != nil || res.HasErrors() {
		t.Fatalf("expected the instantiated call to pass: %v\n%s", err, res.Render())
	}
}
