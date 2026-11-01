package cheader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfineIncludes(t *testing.T) {
	top := t.TempDir()
	root := filepath.Join(top, "mod")
	os.MkdirAll(filepath.Join(root, "inc"), 0o755)
	os.WriteFile(filepath.Join(root, "inc", "a.h"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(top, "outside.h"), []byte("x"), 0o644)
	if os.Symlink(filepath.Join(top, "outside.h"), filepath.Join(root, "inc", "link.h")) != nil {
		t.Skip("no symlinks")
	}
	header := filepath.Join(root, "inc", "a.h")
	opts := func(deps ...string) Options {
		return Options{Root: root, HeaderPath: header, UserDepRun: func(string, string, string) ([]string, error) { return deps, nil }}
	}
	if err := confineIncludes(opts(header, filepath.Join(root, "inc", "..", "inc", "a.h"))); err != nil {
		t.Errorf("includes inside the module: %v", err)
	}
	if err := confineIncludes(Options{HeaderPath: header}); err != nil {
		t.Errorf("no Root means no check: %v", err)
	}
	for name, dep := range map[string]string{
		"dotdot":  filepath.Join(root, "..", "outside.h"),
		"symlink": filepath.Join(root, "inc", "link.h"),
	} {
		if err := confineIncludes(opts(dep)); err == nil || !strings.Contains(err.Error(), "outside the module") {
			t.Errorf("%s: want a refusal, got %v", name, err)
		}
	}
}

func TestConfineIncludesRelativeRoot(t *testing.T) {
	top := t.TempDir()
	os.MkdirAll(filepath.Join(top, "mod", "inc"), 0o755)
	os.WriteFile(filepath.Join(top, "mod", "inc", "a.h"), []byte("x"), 0o644)
	os.WriteFile(filepath.Join(top, "outside.h"), []byte("x"), 0o644)
	t.Chdir(top)
	deps := func(d ...string) Options {
		return Options{Abs: filepath.Abs, Root: "mod", HeaderPath: "mod/inc/a.h", UserDepRun: func(string, string, string) ([]string, error) { return d, nil }}
	}
	if err := confineIncludes(deps("mod/inc/a.h")); err != nil {
		t.Errorf("relative root, header inside: %v", err)
	}
	t.Chdir(filepath.Join(top, "mod"))
	in := Options{Abs: filepath.Abs, Root: ".", HeaderPath: "inc/a.h", UserDepRun: func(string, string, string) ([]string, error) { return []string{"inc/a.h"}, nil }}
	if err := confineIncludes(in); err != nil {
		t.Errorf("root \".\": %v", err)
	}
	in.UserDepRun = func(string, string, string) ([]string, error) { return []string{"../outside.h"}, nil }
	if err := confineIncludes(in); err == nil || !strings.Contains(err.Error(), "outside the module") {
		t.Errorf("relative escape: want a refusal, got %v", err)
	}
}
