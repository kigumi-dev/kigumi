package repl_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

func stdRoot(t *testing.T) string {
	t.Helper()
	p, _ := filepath.Abs("../../../../std")
	return p
}

func withStdin(t *testing.T, src string) {
	t.Helper()
	old := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(src); err != nil {
		t.Fatal(err)
	}
	w.Close()
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; r.Close() })
}

func TestReplStdinCells(t *testing.T) {
	withStdin(t, "let x = 1\nprint \"${x + 1}\"\n")
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "repl", "-s")
	if code != 0 || out != "2\n" || strings.Contains(errOut, "error[") {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestReplCommand(t *testing.T) {
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "repl", "-c", `print "ok"`)
	if code != 0 || out != "ok\n" {
		t.Fatalf("code=%d out=%q err=%q", code, out, errOut)
	}
}

func TestReplStdinFailureStops(t *testing.T) {
	withStdin(t, "let x = 1\nprint \"${x}\"\nexit 7\n")
	code, out, _ := clitest.Execute(t, "--std", stdRoot(t), "repl", "-s")
	if code != 7 || out != "1\n" {
		t.Fatalf("code=%d out=%q", code, out)
	}
}
