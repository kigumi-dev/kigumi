package fmt_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestFmtWriteRefusesSymlink checks that fmt -w refuses a symlink by default instead of replacing its directory entry with a regular file.
func TestFmtWriteRefusesSymlink(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.kg")
	src := "let  x = 1\nprint \"v=${x}\"\n"
	if err := os.WriteFile(real, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.kg")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symlinks unsupported: %v", err)
	}

	if code, _, errOut := clitest.Execute(t, "fmt", "-w", link); code == 0 {
		t.Fatalf("fmt -w on a symlink must fail, not silently succeed; stderr %q", errOut)
	}

	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatal("link.kg must still be a symlink after the refused write")
	}
	target, err := os.Readlink(link)
	if err != nil || target != real {
		t.Fatalf("link.kg must still point at real.kg, got %q err=%v", target, err)
	}
	after, err := os.ReadFile(real)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != src {
		t.Fatalf("real.kg must be untouched (never formatted through the link), got %q", after)
	}
}
