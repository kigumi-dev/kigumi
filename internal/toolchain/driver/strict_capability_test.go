package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// ffi.CString.new has a target-specific, OS-free rt_core.c implementation, unlike os.Host.shell.
func TestStrictProfileAllowsFreestandingSafeProviderOp(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"strict2\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	src := "import ffi from std/ffi\nimport {Arena} from std/alloc\n\n" +
		"test \"cstring under a strict profile\" {\n    let h = Arena.create().unwrap()\n    allocator h.scope() {\n        let s = \"hi\"\n        let _ = ffi.CString.new(&s).unwrap()\n    }\n}\n"
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := driver.ParseTarget(testCompiler(), "x86_64-freestanding", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if _, ok, err := driver.Compile(m, &out); err != nil || !ok {
		t.Fatalf("expected Compile to accept CString.new under a strict profile, ok=%v err=%v\n%s", ok, err, out.String())
	}
	if strings.Contains(out.String(), "E1370") {
		t.Fatalf("CString.new is freestanding-safe and should not raise E1370:\n%s", out.String())
	}
}
