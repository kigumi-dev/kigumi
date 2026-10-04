package driver_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// fakeZig records each invocation's subcommand to countFile, then execs realZig.
func fakeZig(t *testing.T, realZig, countFile string) {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho \"$1\" >> " + shQuote(countFile) + "\nexec " + shQuote(realZig) + ` "$@"` + "\n"
	path := filepath.Join(dir, "zig")
	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func shQuote(s string) string { return "'" + s + "'" }

// countSubcommand counts countFile's lines equal to subcommand.
func countSubcommand(t *testing.T, path, subcommand string) int {
	t.Helper()
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, l := range strings.Split(string(b), "\n") {
		if l == subcommand {
			n++
		}
	}
	return n
}

// A cache hit still re-runs the cheap `zig cc -M` closure check, to catch a changed #include.
func TestHeaderImportCache(t *testing.T) {
	realZig, err := exec.LookPath("zig")
	if err != nil {
		t.Skip("no zig")
	}
	countFile := filepath.Join(t.TempDir(), "count")
	fakeZig(t, realZig, countFile)

	root := t.TempDir()
	// The nonce keeps this header's content hash unique, so no earlier run's cache entry can false-hit it.
	header := fmt.Sprintf("#define CACHE_TEST_NONCE %d\nint cache_test_fn(int x);\n", os.Getpid())
	os.WriteFile(filepath.Join(root, "n.h"), []byte(header), 0o644)
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"cache-test\"\n    kigumi: \"0.1\"\n}\n\nHeader {\n    path: \"n.h\"\n    package: \"n\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("print \"ok\"\n"), 0o644)
	std, err := filepath.Abs("../../std")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std}); err != nil {
		t.Fatalf("first load: %v", err)
	}
	first := countSubcommand(t, countFile, "translate-c")
	if first == 0 {
		t.Fatal("first import should have invoked translate-c")
	}

	if _, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: std}); err != nil {
		t.Fatalf("second load: %v", err)
	}
	second := countSubcommand(t, countFile, "translate-c")
	if second != first {
		t.Errorf("second import ran translate-c again: %d calls before, %d after", first, second)
	}
	if countSubcommand(t, countFile, "cc") == 0 {
		t.Error("even a cache hit should run the cheap include-closure check")
	}
}
