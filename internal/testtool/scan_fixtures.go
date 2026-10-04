package testtool

import (
	"path/filepath"
	"sort"
	"strings"
)

// Kinds lists the testdata directories this manifest covers, in generation order.
var Kinds = []string{"parser", "sem", "mir", "fmt", "run"}

// ScanDisk maps every fixture id ("<kind>/<basename>") to its path
// relative to repoRoot.
func ScanDisk(repoRoot string) (map[string]string, error) {
	out := map[string]string{}
	for _, kind := range Kinds {
		files, err := filepath.Glob(filepath.Join(repoRoot, "testdata", kind, "*.txtar"))
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			rel, err := filepath.Rel(repoRoot, f)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			base := strings.TrimSuffix(filepath.Base(f), ".txtar")
			out[kind+"/"+base] = rel
		}
	}
	return out, nil
}

// SortFixtures orders a fixture slice by id, the order Save expects for a
// stable diff.
func SortFixtures(fs []Fixture) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}
