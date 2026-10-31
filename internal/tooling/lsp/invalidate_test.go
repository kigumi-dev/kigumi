package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.lsp.dev/uri"
)

// TestInvalidateFileIsRootGrained: editing a file in one module root must
// not drop an unrelated root's cached analysis in the same workspace.
func TestInvalidateFileIsRootGrained(t *testing.T) {
	a, b := t.TempDir(), t.TempDir()
	os.MkdirAll(filepath.Join(a, "main"), 0o755)
	os.MkdirAll(filepath.Join(b, "main"), 0o755)
	os.WriteFile(filepath.Join(a, "main", "main.kg"), []byte("fn f() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(b, "main", "main.kg"), []byte("fn g() -> Int {\n    2\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../std")
	s := newServer(std)
	s.roots = []string{a, b}
	ctx := context.Background()

	aFile := filepath.Join(a, "main", "main.kg")
	bFile := filepath.Join(b, "main", "main.kg")
	s.load(ctx, s.moduleRoot(aFile), uri.File(aFile))
	s.load(ctx, s.moduleRoot(bFile), uri.File(bFile))
	if len(s.snaps) != 2 {
		t.Fatalf("expected one cached snapshot per root: %d", len(s.snaps))
	}

	roots := s.invalidateFile(aFile)
	if len(roots) != 1 || roots[0] != s.moduleRoot(aFile) {
		t.Fatalf("expected only a's root invalidated: %v", roots)
	}
	if len(s.snaps) != 1 {
		t.Fatalf("b's snapshot must survive invalidating a file in a: %d left", len(s.snaps))
	}
	if _, ok := s.snaps[s.moduleRoot(bFile)+"\x00"+bFile]; !ok {
		t.Fatalf("b's snapshot was dropped")
	}
}

// TestInvalidateFileFollowsLocalReplace: a change in a local-replacement
// target must also invalidate the module that replaced it in.
func TestInvalidateFileFollowsLocalReplace(t *testing.T) {
	base := t.TempDir()
	lib := filepath.Join(base, "the-lib")
	app := filepath.Join(base, "app")
	os.MkdirAll(lib, 0o755)
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(lib, "mod.kg"), []byte("Module {\n    name: \"the-lib\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	libFile := filepath.Join(lib, "lib.kg")
	os.WriteFile(libFile, []byte("pub fn one() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"the-lib\", version: \"v1.0.0\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"), []byte("Replace { name: \"the-lib\", path: \"../the-lib\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {one} from the-lib\nprint \"${one()}\"\n"), 0o644)

	std, _ := filepath.Abs("../../../std")
	s := newServer(std)
	s.roots = []string{lib, app}
	ctx := context.Background()
	s.load(ctx, app, "")

	roots := s.invalidateFile(libFile)
	found := false
	for _, r := range roots {
		if r == app {
			found = true
		}
	}
	if !found {
		t.Fatalf("a change in the replaced library must invalidate app too: %v", roots)
	}
}
