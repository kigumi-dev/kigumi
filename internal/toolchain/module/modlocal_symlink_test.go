package module_test

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/module"
)

// TestReadLocalFollowsASymlinkedModuleRoot checks that opening a module through
// a symlink resolves the same mod.local.kg as its real path, and that a
// mod.local.kg in the symlink's lexical parent never applies.
func TestReadLocalFollowsASymlinkedModuleRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	realParent := filepath.Join(root, "workspace", "real")
	app := filepath.Join(realParent, "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLocal := func(dir, target string) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "Replace {\n    name: \"example.com/dep\"\n    path: \"" + target + "\"\n}\n"
		if err := os.WriteFile(filepath.Join(dir, module.LocalName), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	realTarget := filepath.Join(root, "repos", "mid")
	lexicalTarget := filepath.Join(root, "repos", "dep")
	writeLocal(realParent, realTarget)
	writeLocal(filepath.Join(root, "workspace"), lexicalTarget)
	link := filepath.Join(root, "workspace", "link")
	if err := os.Symlink(app, link); err != nil {
		t.Fatal(err)
	}

	o := buildconfig.CompilerOptions{Cwd: root}
	viaReal, err := module.ReadLocal(o, app)
	if err != nil {
		t.Fatalf("read real path: %v", err)
	}
	viaLink, err := module.ReadLocal(o, link)
	if err != nil {
		t.Fatalf("read symlink: %v", err)
	}
	if viaReal.Replaces["example.com/dep"] != realTarget {
		t.Fatalf("real path resolved %q, want %q", viaReal.Replaces["example.com/dep"], realTarget)
	}
	if viaLink.Replaces["example.com/dep"] != realTarget {
		t.Fatalf("symlink resolved %q, want the real parent's %q", viaLink.Replaces["example.com/dep"], realTarget)
	}
	if viaLink.File != viaReal.File {
		t.Fatalf("symlink read %q, real path read %q", viaLink.File, viaReal.File)
	}
}
