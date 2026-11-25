package fmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// `kigumi fmt -w` writes atomically, preserving the source's mode and leaving it untouched on failure.
func TestFmtWriteIsAtomicAndPreservesMode(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root ignores file permissions")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "tool.kg")
	if err := os.WriteFile(file, []byte("#!/usr/bin/env kigumi\nlet  x = 1\nprint \"v=${x}\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	if code, _, errOut := clitest.Execute(t, "fmt", "-w", file); code != 0 {
		t.Fatalf("fmt -w: exit %d, stderr %q", code, errOut)
	}
	fi, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o755 {
		t.Fatalf("executable bit lost: mode is %v", fi.Mode().Perm())
	}
	formatted, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o755)
	if code, _, _ := clitest.Execute(t, "fmt", "-w", file); code == 0 {
		t.Fatal("fmt -w into a read-only directory should fail, not silently succeed")
	}
	after, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(formatted) {
		t.Fatalf("a failed write must leave the original content untouched, got %q", after)
	}
}

// A write must reach a fixed point: after `fmt -w`, `fmt --check` stays clean
// and the comments in a fn head, a shell literal prefix, and an import alias
// are still there.
func TestFmtWriteLeavesCheckCleanWithComments(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"fn.kg":    "fn /* why */ g(a: Int, b: Int) {}\n",
		"shell.kg": "fn f() {\n    print $ /* c */ \"cmd\"\n}\n",
		"alias.kg": "import {a /* why */ as b} from std/math\n",
	}
	for name, src := range files {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		if code, _, errOut := clitest.Execute(t, "fmt", "-w", path); code != 0 {
			t.Fatalf("%s: fmt -w: exit %d, stderr %q", name, code, errOut)
		}
		if code, _, _ := clitest.Execute(t, "fmt", "--check", path); code != 0 {
			t.Fatalf("%s: fmt --check after fmt -w: exit %d", name, code)
		}
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(got), "/*") != 1 {
			t.Fatalf("%s: comment lost or duplicated:\n%s", name, got)
		}
	}
}
