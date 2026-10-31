package driver_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyTree(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, p)
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(to, rel), 0o755)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(to, rel), b, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}

// Every example checks to the same output as units, cold, after a comment-only edit of each package, and again unchanged.
func TestUnitExamplesMatchWholeCheck(t *testing.T) {
	dirs, _ := filepath.Glob("../../../examples/*/mod.kg")
	for _, mod := range dirs {
		dir := filepath.Dir(mod)
		t.Run(filepath.Base(dir), func(t *testing.T) {
			root, cache := t.TempDir(), t.TempDir()
			copyTree(t, dir, root)
			entry := ""
			if _, err := os.Stat(filepath.Join(root, "main.kg")); err == nil {
				entry = "main.kg"
			}
			sameAsWhole(t, root, cache, entry)
			filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
				if err == nil && !d.IsDir() && strings.HasSuffix(p, ".kg") && !strings.HasSuffix(p, "mod.kg") && !strings.HasSuffix(p, "main.kg") {
					if b, rerr := os.ReadFile(p); rerr == nil {
						os.WriteFile(p, append(b, []byte("\n// touched\n")...), 0o644)
					}
				}
				return nil
			})
			ageTree(t, root)
			sameAsWhole(t, root, cache, entry)
			sameAsWhole(t, root, cache, entry)
		})
	}
}
