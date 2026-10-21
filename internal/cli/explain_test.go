package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplainCode(t *testing.T) {
	code, out, _ := execute(t, "explain")
	if code != 0 || !strings.Contains(out, "E100") {
		t.Fatalf("explain with no argument must list the codes: exit %d\n%s", code, out)
	}
	code, out, _ = execute(t, "explain", "E100")
	if code != 0 || out == "" {
		t.Fatalf("explain E100: exit %d\n%s", code, out)
	}
	code, errOut := executeReal(t, "explain", "E9999999")
	if code == 0 || !strings.Contains(errOut, "unknown diagnostic code") {
		t.Fatalf("explain with an unknown code must fail: exit %d\n%s", code, errOut)
	}
}

func TestExplainFnOpSummary(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "main"), 0o755)
	file := filepath.Join(dir, "main", "main.kg")
	os.WriteFile(file, []byte(`
type Box = { n String }

fn allocating() {
    let b = Box { n: "x" }
}

fn quiet() {}
`), 0o644)

	code, out, errOut := execute(t, "--std", std, "explain", "--fn", "main.allocating", file)
	if code != 0 || !strings.Contains(out, "may allocate") {
		t.Fatalf("explain --fn on an allocating function: exit %d\n%s%s", code, out, errOut)
	}

	for _, fn := range []string{"std/array.join", "std/text.lines"} {
		code, out, errOut = execute(t, "--std", std, "explain", "--fn", fn, file)
		if code != 0 || !strings.Contains(out, "may allocate") {
			t.Fatalf("explain --fn on %s, which the module never names: exit %d\n%s%s", fn, code, out, errOut)
		}
	}

	code, out, errOut = execute(t, "--std", std, "explain", "--fn", "main.quiet", file)
	if code != 0 || !strings.Contains(out, "no operation in this closed vocabulary") {
		t.Fatalf("explain --fn on a quiet function: exit %d\n%s%s", code, out, errOut)
	}

	isr := filepath.Join(dir, "isr", "main.kg")
	os.MkdirAll(filepath.Dir(isr), 0o755)
	os.WriteFile(isr, []byte("export(interrupt) {\n    pub fn tick() {}\n}\n"), 0o644)
	code, out, errOut = execute(t, "--std", std, "explain", "--fn", "main.tick", isr)
	if code != 0 || !strings.Contains(out, "interrupt handler, checked under the root common configuration") || !strings.Contains(out, "permits: no MMIO or synchronization operation") || !strings.Contains(out, "root limit: no MMIO") {
		t.Fatalf("explain --fn on a handler must show its resolved permissions: exit %d\n%s%s", code, out, errOut)
	}

	code, errOut = executeReal(t, "--std", std, "explain", "--fn", "main.missing", file)
	if code == 0 || !strings.Contains(errOut, "no function") {
		t.Fatalf("explain --fn on an unknown name must fail: exit %d\n%s", code, errOut)
	}

	code, errOut = executeReal(t, "--std", std, "explain", "--fn", "main.quiet")
	if code == 0 || !strings.Contains(errOut, "--fn needs exactly one") {
		t.Fatalf("explain --fn without a module argument must fail: exit %d\n%s", code, errOut)
	}
}
