package fmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func writeFmtFiles(t *testing.T) (a, bad, c string) {
	t.Helper()
	dir := t.TempDir()
	a, bad, c = filepath.Join(dir, "a.kg"), filepath.Join(dir, "bad.kg"), filepath.Join(dir, "c.kg")
	for path, src := range map[string]string{a: "fn a()   {\n1\n}\n", bad: "fn (((\n", c: "fn c()   {\n2\n}\n"} {
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return a, bad, c
}

func TestFmtWriteProcessesEveryFileAndFailsOnce(t *testing.T) {
	a, bad, c := writeFmtFiles(t)
	code, _, errOut := clitest.Execute(t, "fmt", "-w", a, bad, c)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if !strings.Contains(errOut, "bad.kg") {
		t.Errorf("the failing file is not named:\n%s", errOut)
	}
	for _, path := range []string{a, c} {
		got, _ := os.ReadFile(path)
		if !strings.Contains(string(got), "\n    ") {
			t.Errorf("%s was not rewritten after a failure: %q", path, got)
		}
	}
}

func TestFmtCheckReportsEveryFile(t *testing.T) {
	a, bad, c := writeFmtFiles(t)
	code, _, errOut := clitest.Execute(t, "fmt", "--check", a, bad, c)
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	for _, name := range []string{"a.kg: not formatted", "bad.kg", "c.kg: not formatted"} {
		if !strings.Contains(errOut, name) {
			t.Errorf("missing %q in:\n%s", name, errOut)
		}
	}
}

func TestFmtPrintSeparatesFiles(t *testing.T) {
	a, _, c := writeFmtFiles(t)
	code, out, errOut := clitest.Execute(t, "fmt", a, c)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	want := "==> " + a + " <==\nfn a() {\n    1\n}\n==> " + c + " <==\nfn c() {\n    2\n}\n"
	if out != want {
		t.Errorf("stdout:\n%q\nwant:\n%q", out, want)
	}
}
