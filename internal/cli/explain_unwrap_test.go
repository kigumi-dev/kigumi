package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainFnReportsPanicThroughUnwrap(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "main"), 0o755)
	file := filepath.Join(dir, "main", "main.kg")
	os.WriteFile(file, []byte(`
fn firstOf(xs: Array[Int]) -> Int {
    xs.get(0).unwrap()
}

fn named(o: Option[Int]) -> Int {
    o.expect("present")
}

fn quiet(n: Int) -> Int {
    n
}
`), 0o644)
	for _, fn := range []string{"main.firstOf", "main.named"} {
		code, out, errOut := execute(t, "--std", std, "explain", "--fn", fn, file)
		if code != 0 || !strings.Contains(out, "may panic") {
			t.Fatalf("explain --fn %s must report the panic behind unwrap/expect: exit %d\n%s%s", fn, code, out, errOut)
		}
	}
	code, out, errOut := execute(t, "--std", std, "explain", "--fn", "main.quiet", file)
	if code != 0 || strings.Contains(out, "may panic") {
		t.Fatalf("explain --fn main.quiet must not report a panic: exit %d\n%s%s", code, out, errOut)
	}
}
