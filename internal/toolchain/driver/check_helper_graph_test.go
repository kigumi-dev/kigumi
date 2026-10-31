package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/toolchain/driver"
)

func TestCheckDuplicatedHelperGraph(t *testing.T) {
	out, _, cpu := probedUsage(t, func() string { return checkDuplicatedHelperGraph(t) })
	if out != "checked" {
		t.Fatalf("output = %q", out)
	}
	if cpu > 3*time.Second {
		t.Fatalf("cpu time %v, limit 3s", cpu)
	}
	t.Logf("check used %v of CPU time", cpu)
}

func checkDuplicatedHelperGraph(t *testing.T) string {
	t.Helper()
	const depth = 20
	var src strings.Builder
	src.WriteString("type R = { pub x Int }\nfn h0(r: &mut R, f: escaping fn() -> Int) -> Int { r.x + f() }\n")
	for i := 1; i <= depth; i++ {
		fmt.Fprintf(&src, "fn h%d(r: &mut R, f: escaping fn() -> Int) -> Int { h%d(r, f) + h%d(r, f) }\n", i, i-1, i-1)
	}
	fmt.Fprintf(&src, "fn main() { let mut r = R { x: 1 }; print \"${h%d(&mut r, () => 2)}\" }\n", depth)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
	if err != nil {
		t.Fatal(err)
	}
	r, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if r.HasErrors() {
		t.Fatal(r.Render())
	}
	return "checked"
}
