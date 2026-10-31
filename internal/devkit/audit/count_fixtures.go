package audit

import (
	"bufio"
	"io/fs"
	"kigumi/internal/devkit/fixture"
	"os"
	"path/filepath"
	"strings"
)

// FixtureCounts tallies the corpus by kind, plus Kigumi `test "..." { }`
// blocks found in .kg sources (std, examples, testdata).
type FixtureCounts struct {
	ByKind     map[string]int `json:"byKind"`
	Total      int            `json:"total"`
	TestBlocks int            `json:"testBlocks"`
}

// CountFixtures scans the manifest's testdata kinds and every .kg file
// under repoRoot for `test "..."` blocks.
func CountFixtures(repoRoot string) (FixtureCounts, error) {
	fc := FixtureCounts{ByKind: map[string]int{}}
	for _, kind := range fixture.Kinds {
		files, err := filepath.Glob(filepath.Join(repoRoot, "testdata", kind, "*.txtar"))
		if err != nil {
			return fc, err
		}
		fc.ByKind[kind] = len(files)
		fc.Total += len(files)
	}
	n, err := countTestBlocks(repoRoot)
	if err != nil {
		return fc, err
	}
	fc.TestBlocks = n
	return fc, nil
}

func countTestBlocks(repoRoot string) (int, error) {
	n := 0
	err := filepath.WalkDir(repoRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "node_modules" || d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}
		if filepath.Ext(p) != ".kg" {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if strings.HasPrefix(strings.TrimSpace(sc.Text()), `test "`) {
				n++
			}
		}
		return sc.Err()
	})
	return n, err
}
