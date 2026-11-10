package driver

import (
	"os"
	"path/filepath"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
)

// TestResolveProgram covers the rule a bare tool name follows: it is searched
// in the compiler options' path directories, never the driver's environment,
// and an absolute or relative path naming a file is used as given.
func TestResolveProgram(t *testing.T) {
	t.Parallel()
	bin := t.TempDir()
	tool := filepath.Join(bin, "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	o := buildconfig.CompilerOptions{PathDirs: []string{bin}}
	if got, ok := resolveProgram(o, "tool"); !ok || got != tool {
		t.Fatalf("bare name resolved to %q, %v; want %q", got, ok, tool)
	}
	if got, ok := resolveProgram(o, tool); !ok || got != tool {
		t.Fatalf("an absolute path changed: %q, %v", got, ok)
	}
	rel := "." + string(filepath.Separator) + "tool"
	if got, ok := resolveProgram(o, rel); !ok || got != rel {
		t.Fatalf("a path with a separator changed: %q, %v", got, ok)
	}
	if _, ok := resolveProgram(o, "no-such-tool"); ok {
		t.Fatal("a missing bare name must not resolve")
	}
}
