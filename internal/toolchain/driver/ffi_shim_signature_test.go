package driver_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFFIShimsKeepLongSignaturesDistinct(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	var params, types, args []string
	for i := range 419 {
		params = append(params, fmt.Sprintf("p%d: Int", i))
		types = append(types, "Int")
		args = append(args, fmt.Sprint(i))
	}
	src := fmt.Sprintf(`pub type Pair layout(C) = { pub a i32, pub b f64 }
export(C) {
    pub fn sumA(pr: Pair, %s) -> Int { pr.a.toInt() + p418 }
    pub fn sumB(pr: Pair, %s, q: Int) -> Int { pr.a.toInt() + q }
}
fn viaA(f: extern(C) fn(Pair, %s) -> Int) -> Int {
    unsafe { f(Pair { a: 1, b: 0.5 }, %s) }
}
fn viaB(f: extern(C) fn(Pair, %s, Int) -> Int) -> Int {
    unsafe { f(Pair { a: 1, b: 0.5 }, %s, 777) }
}
print "A ${viaA(sumA)}"
print "B ${viaB(sumB)}"
`, strings.Join(params, ", "), strings.Join(params, ", "), strings.Join(types, ", "), strings.Join(args, ", "), strings.Join(types, ", "), strings.Join(args, ", "))
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "main"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main", "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, run := range []func(*testing.T, string) (string, string, int){runFixtureVM, runFixtureNative} {
		out, errOut, code := run(t, root)
		if out != "A 419\nB 778\n" || errOut != "" || code != 0 {
			t.Fatalf("stdout=%q stderr=%q exit=%d", out, errOut, code)
		}
	}
}
