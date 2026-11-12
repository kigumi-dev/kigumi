package config

import (
	"os"
	"path/filepath"
	"testing"
)

// The hermetic CI job must run with no C toolchain even though the runner
// image ships clang; the no-CC gate has to beat a compiler on the search path.
func TestNoCCompilerEnvHidesToolchain(t *testing.T) {
	t.Setenv("KIGUMI_TEST_NO_CC", "")
	dir := t.TempDir()
	for _, name := range []string{"zig", "clang"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	o := CompilerOptions{PathDirs: []string{dir}}
	if got := o.CCompiler(); got == nil {
		t.Fatal("the fake compilers were not found before the gate")
	}
	t.Setenv("KIGUMI_TEST_NO_CC", "1")
	if got := o.CCompiler(); got != nil {
		t.Fatalf("KIGUMI_TEST_NO_CC still found %v", got)
	}
}
