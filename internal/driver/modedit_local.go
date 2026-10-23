package driver

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/token"
)

// AppendLocalReplace sets a Replace for name in dir/mod.local.kg; an existing record for name is rewritten in place rather than left as a second one.
func AppendLocalReplace(dir, name, path string) error {
	mf, _, err := ReadModFile(dir)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(mf.Requires, func(r Require) bool { return r.Name == name }) {
		return fmt.Errorf("%s is not required in %s; add it with `kigumi mod add` first", name, ManifestName)
	}
	target := path
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	if st, err := os.Stat(target); err != nil || !st.IsDir() {
		return fmt.Errorf("%s is not a directory; nothing to resolve %s from", target, name)
	}
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
	next, changed := dropRecords(f, recs, false, func(r record) bool { return r.Kind == "Replace" && names[r.Fields["name"]] })
	if !changed {
		return nil
	}
	return writeFileAtomic(file, next, 0o644)
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
