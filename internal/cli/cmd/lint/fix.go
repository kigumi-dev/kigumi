package lint

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"kigumi/internal/base/errors"
	"kigumi/internal/base/fsx"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/driver"
)

// edit is one replacement in a file, addressed by byte offsets.
type edit struct {
	start, end int
	text       string
}

// applySafeFixes writes the fixes the checker marked safe, one diagnostic at a
// time so alternate fixes of the same diagnostic never race. It reports
// whether any file changed.
func applySafeFixes(m *driver.Module, ds []diag.Diagnostic) (bool, error) {
	root, err := filepath.Abs(m.Root)
	if err != nil {
		return false, errors.WrapErr(err, "resolve module root")
	}
	external := externalSources(m)
	byPath := map[string][]edit{}
	seen := map[string]bool{}
	for _, d := range ds {
		if len(d.Fixes) == 0 {
			continue
		}
		// The fixes sharing the first title are one action; later titles are
		// alternatives.
		title := d.Fixes[0].Title
		for _, f := range d.Fixes {
			if f.Title != title {
				continue
			}
			path, ok := fixPath(root, m.FileStore, external, d, f)
			if !ok {
				continue
			}
			start, end := int(f.Loc.Span.Start), int(f.Loc.Span.End)
			key := path + "\x00" + strconv.Itoa(start) + "\x00" + strconv.Itoa(end) + "\x00" + f.NewText
			if seen[key] {
				continue
			}
			seen[key] = true
			byPath[path] = append(byPath[path], edit{start: start, end: end, text: f.NewText})
		}
	}
	changed := false
	for _, path := range sortedKeys(byPath) {
		ok, err := applyFile(path, byPath[path])
		if err != nil {
			return changed, err
		}
		changed = changed || ok
	}
	return changed, nil
}

// fixPath resolves a fix's source to a file the module owns. Generated sources
// and files outside the module root (std, dependencies) are not rewritten.
func fixPath(root string, files diag.Sources, external map[string]bool, d diag.Diagnostic, f diag.Fix) (string, bool) {
	if files == nil {
		return "", false
	}
	sid := f.Loc.Source
	if sid == 0 {
		sid = d.Loc.Source
	}
	src := files.Source(sid)
	if src == nil || src.Generated || src.Name == "" || external[src.Name] {
		return "", false
	}
	path := filepath.FromSlash(src.Name)
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// applyFile applies non-overlapping edits right to left, leaving a file that
// overlaps an earlier edit untouched rather than corrupting it.
func applyFile(path string, edits []edit) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, errors.WrapErrf(err, "read %s", path)
	}
	original := src
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].start != edits[j].start {
			return edits[i].start > edits[j].start
		}
		return edits[i].end > edits[j].end
	})
	last := len(src) + 1
	for _, e := range edits {
		if e.start < 0 || e.end < e.start || e.end > len(src) || e.end > last {
			continue
		}
		next := make([]byte, 0, len(src)-(e.end-e.start)+len(e.text))
		next = append(next, src[:e.start]...)
		next = append(next, e.text...)
		next = append(next, src[e.end:]...)
		src = next
		last = e.start
	}
	if bytes.Equal(src, original) {
		return false, nil
	}
	perm := os.FileMode(0o644)
	if fi, err := os.Stat(path); err == nil {
		perm = fi.Mode().Perm()
	}
	if err := fsx.WriteFileAtomic(path, src, perm); err != nil {
		return false, errors.WrapErrf(err, "write %s", path)
	}
	return true, nil
}

// externalSources names std and dependency files, which lint must never
// rewrite; the module root owns only its own packages.
func externalSources(m *driver.Module) map[string]bool {
	out := map[string]bool{}
	for _, p := range m.Packages {
		if !p.Std && p.Module == "" {
			continue
		}
		for _, t := range p.Files {
			out[t.File.Name] = true
		}
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
