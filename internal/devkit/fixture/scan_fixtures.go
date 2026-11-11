package fixture

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
)

// Kinds lists the testdata directories this manifest covers, in generation order.
var Kinds = []string{"parser", "sem", "mir", "fmt", "run", "oom"}

// kindExt maps a fixture kind to the file extension its corpus uses.
var kindExt = map[string]string{
	"parser": ".txtar",
	"sem":    ".txtar",
	"mir":    ".txtar",
	"fmt":    ".txtar",
	"run":    ".txtar",
	"oom":    ".kg",
}

// Ext returns the corpus file extension for kind (".txtar" for an unknown
// kind so a typo cannot silently scan nothing).
func Ext(kind string) string {
	if ext, ok := kindExt[kind]; ok {
		return ext
	}
	return ".txtar"
}

// Files walks dir recursively and returns every file with ext, sorted.
func Files(dir, ext string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ext {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// Corpus lists every fixture file of kind under root/testdata, walks the kind
// directory recursively, and returns the paths sorted. Callers never name a
// corpus directory themselves, so a group is a physical detail only.
func Corpus(root, kind string) ([]string, error) {
	return Files(filepath.Join(root, "testdata", kind), Ext(kind))
}

// IDFromPath maps a fixture path to "<kind>/<basename>": the component after
// the testdata directory names the kind, and the file's basename (without its
// extension) is the ID that stays fixed when the file moves between groups.
func IDFromPath(path string) string {
	clean := filepath.ToSlash(filepath.Clean(path))
	parts := strings.Split(clean, "/")
	ext := filepath.Ext(clean)
	base := strings.TrimSuffix(parts[len(parts)-1], ext)
	for i := len(parts) - 2; i >= 0; i-- {
		if parts[i] == "testdata" && i+1 < len(parts)-1 {
			return parts[i+1] + "/" + base
		}
	}
	return base
}

// Find returns the path of the fixture with the given id ("<kind>/<name>")
// under root/testdata. It walks the kind directory recursively, because the
// group holding a fixture is not part of its ID.
func Find(root, id string) (string, error) {
	kind, base, ok := strings.Cut(id, "/")
	if !ok || kind == "" || base == "" || strings.Contains(base, "/") {
		return "", fmt.Errorf("fixture id %q: want <kind>/<name>", id)
	}
	dir := filepath.Join(root, "testdata", kind)
	ext := Ext(kind)
	found := ""
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ext {
			return nil
		}
		if strings.TrimSuffix(filepath.Base(path), ext) != base {
			return nil
		}
		if found != "" {
			return fmt.Errorf("fixture id %q matches both %s and %s", id, found, path)
		}
		found = path
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", fmt.Errorf("no fixture with id %q under %s", id, dir)
	}
	return found, nil
}

// ScanDisk maps every fixture id ("<kind>/<basename>") to its path relative
// to repoRoot. Basenames must be unique within a kind; a collision is an
// error, because two fixtures would then share one manifest id.
func ScanDisk(repoRoot string) (map[string]string, error) {
	out := map[string]string{}
	for _, kind := range Kinds {
		files, err := Corpus(repoRoot, kind)
		if err != nil {
			return nil, err
		}
		for _, f := range files {
			id := IDFromPath(f)
			if prev, dup := out[id]; dup {
				return nil, fmt.Errorf("fixtures %s and %s share id %q", prev, f, id)
			}
			rel, err := filepath.Rel(repoRoot, f)
			if err != nil {
				return nil, err
			}
			out[id] = filepath.ToSlash(rel)
		}
	}
	return out, nil
}

// SortFixtures orders a fixture slice by id, the order Save expects for a
// stable diff.
func SortFixtures(fs []Fixture) {
	sort.Slice(fs, func(i, j int) bool { return fs[i].ID < fs[j].ID })
}
