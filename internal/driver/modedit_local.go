package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// AppendLocalReplace sets a Replace for name in dir/mod.local.kg; an existing record for name is rewritten in place rather than left as a second one.
func AppendLocalReplace(dir, name, path string) error {
	file := filepath.Join(dir, LocalName)
	src, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	next, replaced := replaceLocalRecord(src, name, path)
	if !replaced {
		var sb strings.Builder
		if len(src) == 0 {
			sb.WriteString("// Local overrides for development; never read from a dependency.\n")
		}
		writeRecord(&sb, "Replace", "name", name, "path", path)
		next, err = nextFileContent(file, sb.String())
		if err != nil {
			return err
		}
	}
	if _, ds := ParseLocal(token.NewFile(LocalName, next), dir); len(ds) > 0 {
		return fmt.Errorf("%s would become invalid: %s", file, diag.RenderAll(token.NewFile(LocalName, next), ds[:1]))
	}
	return writeFileAtomic(file, next, 0o644)
}

// replaceLocalRecord rewrites an existing Replace record's path in place; ok is false when src has none (or doesn't parse), so the caller appends instead.
func replaceLocalRecord(src []byte, name, path string) (next []byte, ok bool) {
	if len(src) == 0 {
		return nil, false
	}
	f := token.NewFile(LocalName, src)
	recs, ds := parseRecords(f, localFields)
	if len(ds) > 0 {
		return nil, false
	}
	for _, r := range recs {
		if r.Kind == "Replace" && r.Fields["name"] == name {
			var sb strings.Builder
			writeRecord(&sb, "Replace", "name", name, "path", path)
			out := append(append([]byte{}, src[:r.Span.Start]...), sb.String()...)
			out = append(out, src[r.Span.End:]...)
			return out, true
		}
	}
	return nil, false
}

// RemoveLocalReplaces deletes the named modules' Replace records from dir/mod.local.kg; a missing file, or one without any of names, is not an error.
func RemoveLocalReplaces(dir string, names map[string]bool) error {
	file := filepath.Join(dir, LocalName)
	src, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	f := token.NewFile(LocalName, src)
	recs, ds := parseRecords(f, localFields)
	if len(ds) > 0 {
		return fmt.Errorf("%s has errors; fix them first", file)
	}
	drop := map[int]bool{}
	for _, r := range recs {
		if r.Kind == "Replace" && names[r.Fields["name"]] {
			for line := f.Line(r.Span.Start); line <= f.Line(r.Span.End-1); line++ {
				drop[line] = true
			}
		}
	}
	if len(drop) == 0 {
		return nil
	}
	var sb strings.Builder
	for i, text := range strings.SplitAfter(string(src), "\n") {
		if !drop[i+1] {
			sb.WriteString(text)
		}
	}
	return writeFileAtomic(file, []byte(sb.String()), 0o644)
}

// nextFileContent is what path's content becomes after appending text, without writing it, so a caller can validate the result first.
func nextFileContent(path, text string) ([]byte, error) {
	src, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	if len(src) > 0 && src[len(src)-1] != '\n' {
		src = append(src, '\n')
	}
	return append(src, text...), nil
}
