package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func TestFreestandingEntryHostNoteDoesNotBlameTheEntryFile(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fshost\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let args = host.args()\nprint \"x\"\n"), 0o644)
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if !strings.Contains(out, "`host` is not available on a freestanding target") || strings.Contains(out, "exists only in the entry file") {
		t.Fatalf("freestanding note:\n%s", out)
	}
}
