package mir_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"kigumi/internal/compiler/mir"
)

var builtinEmit = regexp.MustCompile(`Op:\s*OpBuiltin[^}]*Str:\s*"([^"]+)"`)

var builtinFamily = regexp.MustCompile(`"((?:atomic|volatile)\.[A-Za-z]+)"`)

func TestEveryEmittedBuiltinIsRegistered(t *testing.T) {
	files, _ := filepath.Glob("build*.go")
	more, _ := filepath.Glob("spec_proto*.go")
	seen := map[string]bool{}
	for _, path := range append(files, more...) {
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range builtinEmit.FindAllStringSubmatch(string(src), -1) {
			seen[m[1]] = true
		}
		for _, m := range builtinFamily.FindAllStringSubmatch(string(src), -1) {
			seen[m[1]] = true
		}
	}
	for _, name := range []string{"print", "eprint", "panic", "host", "direct.eq", "direct.hash"} {
		seen[name] = true
	}
	if len(seen) < 8 {
		t.Fatalf("found only %d emitted builtins; the scan lost its pattern", len(seen))
	}
	for name := range seen {
		if _, ok := mir.BuiltinFor(name); !ok {
			t.Errorf("the builder emits builtin %q, which the registry lacks", name)
		}
	}
}

func TestRegistryNamesAreUnique(t *testing.T) {
	names := mir.BuiltinNames()
	for i, n := range names {
		if slices.Index(names, n) != i {
			t.Errorf("builtin %q registered twice", n)
		}
	}
}
