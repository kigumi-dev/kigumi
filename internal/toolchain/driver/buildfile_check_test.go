package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// CheckBuildFile parses and checks build.kg the way `kigumi build` would,
// without running configure, and reports only diagnostics bound to build.kg.
func TestCheckBuildFileDiagnostics(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"bf\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let x = 1\nprint \"${x}\"\n"), 0o644)

	path := filepath.Join(root, driver.BuildFileName)
	write := func(src string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	check := func() *driver.BuildFile {
		t.Helper()
		bf, ok, err := driver.CheckBuildFile(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
		if err != nil || !ok {
			t.Fatalf("check build.kg: ok=%v err=%v", ok, err)
		}
		return bf
	}

	write("import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let app = b.executable(\"app\", \"\", b.request().target())?\n}\n")
	if bf := check(); bf.HasErrors() || bf.Render() != "" {
		t.Fatalf("valid build.kg: %v\n%s", bf.HasErrors(), bf.Render())
	}

	write("import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n    let n: Int = \"x\"\n}\n")
	bf := check()
	if !bf.HasErrors() || bf.Res == nil {
		t.Fatalf("a type error must be a checked diagnostic, got res=%v err=%v", bf.Res, bf.Err)
	}
	rendered := bf.Render()
	if !strings.Contains(rendered, "build.kg:4") || !strings.Contains(rendered, "Int") {
		t.Fatalf("diagnostic should point into build.kg:\n%s", rendered)
	}
	for _, d := range bf.Diagnostics() {
		if d.Loc.Source != 0 && d.Loc.Source != bf.Tree.File.ID {
			t.Fatalf("diagnostic leaked from another source file: %+v", d)
		}
	}

	write("import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n")
	if bf := check(); bf.Res != nil || !bf.HasErrors() || !strings.Contains(bf.Render(), "build.kg") {
		t.Fatalf("parse error: res=%v\n%s", bf.Res, bf.Render())
	}

	write("import mine from bf/lib\n\nfn configure(b: build.Builder) -> Void! {}\n")
	os.MkdirAll(filepath.Join(root, "lib"), 0o755)
	os.WriteFile(filepath.Join(root, "lib", "lib.kg"), []byte("pub fn helper() -> Int {\n    1\n}\n"), 0o644)
	if bf := check(); !bf.HasErrors() || bf.Err == nil || !strings.Contains(bf.Err.Msg, "cannot import bf/lib") {
		t.Fatalf("import of the built module: %+v", bf.Err)
	}
}

// A module without build.kg reports ok=false and no diagnostics.
func TestCheckBuildFileAbsent(t *testing.T) {
	std, _ := filepath.Abs("../../../std")
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"nobf\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	bf, ok, err := driver.CheckBuildFile(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil || ok || bf != nil {
		t.Fatalf("no build.kg: bf=%v ok=%v err=%v", bf, ok, err)
	}
}
