package cli_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoStaleEngineGuidance(t *testing.T) {
	for _, args := range [][]string{{"--help"}, {"run", "--help"}, {"test", "--help"}, {"build", "--help"}} {
		_, out, errOut := execute(t, args...)
		if s := out + errOut; strings.Contains(s, "--engine") || strings.Contains(s, "interp") {
			t.Errorf("%v mentions a removed engine:\n%s", args, s)
		}
	}
	for _, dir := range []string{"../driver", ".", "../../cmd"} {
		filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			b, _ := os.ReadFile(path)
			if strings.Contains(string(b), "--engine") {
				t.Errorf("%s still names --engine", path)
			}
			return nil
		})
	}
}
