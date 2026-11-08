package sem

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

// An unlisted package reads as LayerFoundation, so neither `--sys none` nor a
// layer ceiling would reject importing it.
func TestStdLayerListsEveryPackage(t *testing.T) {
	root := "../../../std"
	seen := map[string]bool{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".kg") {
			return err
		}
		rel, _ := filepath.Rel(root, filepath.Dir(path))
		pkg := "std/" + filepath.ToSlash(rel)
		if _, ok := stdLayer[pkg]; !ok && !seen[pkg] {
			t.Errorf("%s has no entry in stdLayer", pkg)
		}
		seen[pkg] = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
