package sem_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

func checkPlanned(t *testing.T, fx fixture) *sem.Result {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fixture\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../std")
	for _, s := range fx.sections {
		if s.name == "module" || s.name == "expect" {
			continue
		}
		if tree := syntax.Parse(token.NewFile(s.name, []byte(s.body))); tree.HasErrors() {
			t.Fatalf("%s: section %s has parse errors (parse errors belong in testdata/parser):\n%s", fx.path, s.name, tree.Dump())
		}
		switch {
		case s.name == "src":
			if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(s.body), 0o644); err != nil {
				t.Fatal(err)
			}
		case strings.HasPrefix(s.name, "std/"):
			std = plannedStd(t, std, s.name, s.body)
		default:
			t.Fatalf("%s: a planned fixture holds a src section and std packages only", fx.path)
		}
	}
	target := driver.HostTarget()
	target.Allocator = fx.opt("allocator", "")
	target.Profile = fx.opt("profile", "")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// plannedStd adds a fixture's std file to a copy of std, made on the first one.
func plannedStd(t *testing.T, std, name, body string) string {
	t.Helper()
	if base, _ := filepath.Abs("../../std"); std == base {
		std = filepath.Join(t.TempDir(), "std")
		if err := os.CopyFS(std, os.DirFS(base)); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(std, filepath.FromSlash(strings.TrimPrefix(name, "std/")))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return std
}
