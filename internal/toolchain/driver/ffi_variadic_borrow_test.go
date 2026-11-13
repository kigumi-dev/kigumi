package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A C variadic slot has no fixed parameter type to coerce a borrow through.
// Passing &x as ... must not silently turn into the referent or a C string.
func TestCVariadicRejectsBorrow(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"va\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(`extern(C) {
    fn printf(fmt: *const u8, ...) -> i32
}

fn main() {
    let x: i64 = 12345
    unsafe { _ = printf(null, &x) }
}
`), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	got := res.Render()
	if !res.HasErrors() || !strings.Contains(got, "E924") || !strings.Contains(got, "&Int") {
		t.Fatalf("a variadic borrow must be rejected as a C ABI type, got:\n%s", got)
	}
}
