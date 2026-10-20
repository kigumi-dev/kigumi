package arch_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// envAllowed lists where a process environment may be read: the CLI
// boundary, which turns KIGUMI_* into explicit options, and the host
// intrinsic (the VM) that exposes the environment to Kigumi programs.
var envAllowed = []string{"internal/cli/", "internal/vm/std_host.go"}

// processAllowed lists the files that may resolve a path against the process cwd, search PATH or ask for the home directory.
// The rule: a main package, or the CLI's one reader (shared/compiler_options.go), reads the process once and passes the facts down; no library file falls back to the process.
var processAllowed = map[string]bool{
	"internal/cli/shared/compiler_options.go": true,
	"cmd/testaudit/main.go":                   true,
	"cmd/e2ebench/main.go":                    true,
}

var processCalls = []string{"filepath.Abs", "os.Getwd", "exec.LookPath", "os.UserHomeDir"}

func TestNoEnvReads(t *testing.T) {
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, "../../"))
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || !(strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/")) {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if !processAllowed[rel] && rel != "internal/vm/std_host.go" {
			for _, call := range processCalls {
				if strings.Contains(string(b), call) {
					t.Errorf("%s: %s を参照している (関数値も含む)。cwd・PATH・home は CompilerOptions 経由で受け取れ", rel, call)
				}
			}
		}
		if envExempt(rel) {
			return nil
		}
		for _, call := range []string{"os.Getenv(", "os.LookupEnv(", "os.Environ(", "os.ExpandEnv("} {
			if strings.Contains(string(b), call) {
				t.Errorf("%s: %s を読んでいる。環境変数は internal/cli で Options に変換せよ", rel, call)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func envExempt(rel string) bool {
	for _, ok := range envAllowed {
		if strings.HasPrefix(rel, ok) {
			return true
		}
	}
	return false
}
