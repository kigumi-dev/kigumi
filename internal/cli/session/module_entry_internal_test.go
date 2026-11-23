package session

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A package directory inside a module names its entry file, so `kigumi build cmd/app` works next to `cmd/tool`;
// module roots and plain library directories pass through unchanged.
func TestPackageEntry(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"cmd/app/main.kg", "cmd/tool/tool.kg", "lib/a.kg", "lib/b.kg"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("print 1\n"), 0o644)
	}
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	for dir, want := range map[string]string{
		"cmd/app":  "cmd/app/main.kg",
		"cmd/tool": "cmd/tool/tool.kg",
		"lib":      "lib",
		".":        ".",
	} {
		got := packageEntry(filepath.Join(root, dir))
		if got != filepath.Join(root, want) {
			t.Errorf("packageEntry(%s) = %s, want %s", dir, got, filepath.Join(root, want))
		}
	}
}

// An ancestor that is only a module through its cmd/ directory (a checkout's Go tooling, say) must not re-root a
// directory passed by absolute path: outside a mod.kg that directory selects its own script. Inside a mod.kg it
// names the package entry exactly as it does when spelled relatively.
func TestResolveDirTargetIgnoresCmdOnlyAncestor(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"cmd/tool/main.go", "work/pkg/main.kg", "work/pkg/other.kg"} {
		os.MkdirAll(filepath.Dir(filepath.Join(root, p)), 0o755)
		os.WriteFile(filepath.Join(root, p), []byte("print 1\n"), 0o644)
	}
	dir := filepath.Join(root, "work", "pkg")
	gotRoot, entry, scriptOnly, err := resolveDirTarget(dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != dir || entry != "main.kg" || !scriptOnly {
		t.Errorf("resolveDirTarget(%s) = %s, %s, scriptOnly=%v; want its own main.kg", dir, gotRoot, entry, scriptOnly)
	}
	// The same tree with a manifest does declare the module, so the package names its entry file in that module.
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	gotRoot, entry, scriptOnly, err = resolveDirTarget(dir)
	if err != nil {
		t.Fatal(err)
	}
	if gotRoot != root || entry != "work/pkg/main.kg" || scriptOnly {
		t.Errorf("resolveDirTarget(%s) = %s, %s, scriptOnly=%v; want the module's work/pkg/main.kg", dir, gotRoot, entry, scriptOnly)
	}
}

// A manifest-less directory with no main.kg but several scripts is ambiguous: naming it must ask for one file (or
// a mod.kg) instead of silently compiling them all into one package.
func TestResolveDirTargetAmbiguousScripts(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.kg"), []byte("print 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.kg"), []byte("print 2\n"), 0o644)
	_, _, _, err := resolveDirTarget(dir)
	if err == nil || !strings.Contains(err.Error(), "several .kg files") || !strings.Contains(err.Error(), "mod.kg") {
		t.Fatalf("resolveDirTarget(%s) error = %v, want an ambiguity asking for a file or mod.kg", dir, err)
	}
	// main.kg resolves the same directory without a manifest, and so does a lone script.
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte("print 3\n"), 0o644)
	if _, entry, scriptOnly, err := resolveDirTarget(dir); err != nil || entry != "main.kg" || !scriptOnly {
		t.Fatalf("main.kg resolution = %q scriptOnly=%v err=%v", entry, scriptOnly, err)
	}
	if err := os.Remove(filepath.Join(dir, "main.kg")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "b.kg")); err != nil {
		t.Fatal(err)
	}
	if _, entry, scriptOnly, err := resolveDirTarget(dir); err != nil || entry != "a.kg" || !scriptOnly {
		t.Fatalf("lone script resolution = %q scriptOnly=%v err=%v", entry, scriptOnly, err)
	}
}

// A mod.kg directory keeps package=directory: the target stays the module root, and all its .kg files stay in the load.
func TestResolveDirTargetModuleStaysDirectory(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.kg"), []byte("print 1\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "other.kg"), []byte("pub fn two() -> Int { 2 }\n"), 0o644)
	root, entry, scriptOnly, err := resolveDirTarget(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir || entry != "" || scriptOnly {
		t.Fatalf("resolveDirTarget(%s) = %s, %q, scriptOnly=%v; want the module directory itself", dir, root, entry, scriptOnly)
	}
}

// An entry file with no mod.kg above it is a script (ScriptOnly); the same file in a module uses package=directory.
func TestResolveFileTargetScriptOnly(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.kg")
	os.WriteFile(file, []byte("print 1\n"), 0o644)
	root, entry, scriptOnly, err := resolveFileTarget(file)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir || entry != "a.kg" || !scriptOnly {
		t.Fatalf("manifest-less file = %s, %q, scriptOnly=%v; want its own directory and a script", root, entry, scriptOnly)
	}
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	root, entry, scriptOnly, err = resolveFileTarget(file)
	if err != nil {
		t.Fatal(err)
	}
	if root != dir || entry != "a.kg" || scriptOnly {
		t.Fatalf("module file = %s, %q, scriptOnly=%v; want the module's package file", root, entry, scriptOnly)
	}
}

func TestLoadRejectsNonDirectoryTarget(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.txt")
	if err := os.WriteFile(file, []byte("not a module"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := (Session{}).Load(file, false, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "is not a .kg file or a directory") {
		t.Fatalf("Load(%s) error = %v", file, err)
	}
}
