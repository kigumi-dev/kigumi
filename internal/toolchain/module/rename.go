package module

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"kigumi/internal/base/fsx"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// RenameModule renames every reference to old inside the module rooted at
// root: the Module record when it names old, Require records naming it, the
// nearest mod.local.kg's Replace records and every import path that belongs
// to old. It is all-or-nothing: every next file is parsed and checked before
// the first write, and mod.lock.kg is never touched, because a moved module
// is a new identity that has to be fetched again.
//
// old must be the module's own name or one of its Requires; new must not
// already be named. The returned changes list the files rewritten, and the
// warnings say what the rename leaves for `kigumi get` and `kigumi mod
// vendor`.
func RenameModule(root, old, next string) (changes, warnings []string, err error) {
	if err := ValidModuleName(old); err != nil {
		return nil, nil, err
	}
	if err := ValidModuleName(next); err != nil {
		return nil, nil, err
	}
	if old == next {
		return nil, nil, fmt.Errorf("%s and %s are the same module path", old, next)
	}
	mf, ok, err := ReadModFile(root)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, fmt.Errorf("%s has no %s", root, ManifestName)
	}
	own := mf.Name == old
	var renamed []Require
	for _, r := range mf.Requires {
		if r.Name == old {
			renamed = append(renamed, r)
		}
	}
	if !own && len(renamed) == 0 {
		return nil, nil, fmt.Errorf("%s names no module %s", ManifestName, old)
	}
	if mf.Name == next {
		return nil, nil, fmt.Errorf("the module is already named %s", next)
	}
	for _, r := range mf.Requires {
		if r.Name == next {
			return nil, nil, fmt.Errorf("%s is already required; a rename cannot merge two modules", next)
		}
	}
	// A /vN rename changes the versions the path may carry: refuse to write a
	// manifest whose existing version no longer matches the new path.
	for _, r := range renamed {
		if err := CheckVersionForName(next, r.Version); err != nil {
			return nil, nil, fmt.Errorf("cannot rename %s to %s: %v", old, next, err)
		}
	}

	writes := map[string][]byte{}
	add := func(path string, data []byte) { writes[path] = data }
	manifestPath := filepath.Join(root, ManifestName)
	src, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	kinds := map[string]bool{"Require": true}
	if own {
		kinds["Module"] = true
	}
	edits, err := nameFieldEdits(ManifestName, src, kinds, old, next)
	if err != nil {
		return nil, nil, err
	}
	nextManifest := applyEdits(src, edits)
	if len(edits) > 0 {
		if _, ds := ParseModFile(token.NewFile(ManifestName, nextManifest)); len(ds) > 0 {
			return nil, nil, fmt.Errorf("%s would become invalid: %s", manifestPath, diag.RenderAll(token.NewFile(ManifestName, nextManifest), ds[:1]))
		}
		add(manifestPath, nextManifest)
	}

	if localFile := nearestLocal(root); localFile != "" {
		src, err := os.ReadFile(localFile)
		if err != nil {
			return nil, nil, err
		}
		edits, err := nameFieldEdits(filepath.Base(localFile), src, map[string]bool{"Replace": true}, old, next)
		if err != nil {
			return nil, nil, err
		}
		if len(edits) > 0 {
			nextLocal := applyEdits(src, edits)
			f := token.NewFile(LocalName, nextLocal)
			if _, ds := ParseLocal(f, filepath.Dir(localFile)); len(ds) > 0 {
				return nil, nil, fmt.Errorf("%s would become invalid: %s", localFile, diag.RenderAll(f, ds[:1]))
			}
			add(localFile, nextLocal)
		}
	}

	files, err := sourceFiles(root)
	if err != nil {
		return nil, nil, err
	}
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		edits, err := importEdits(path, src, old, next)
		if err != nil {
			return nil, nil, err
		}
		if len(edits) > 0 {
			add(path, applyEdits(src, edits))
		}
	}

	paths := make([]string, 0, len(writes))
	for p := range writes {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	for _, p := range paths {
		if err := fsx.WriteFileAtomic(p, writes[p], 0o644); err != nil {
			return nil, nil, err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			rel = p
		}
		changes = append(changes, filepath.ToSlash(rel))
	}
	if len(changes) == 0 {
		return nil, nil, fmt.Errorf("nothing refers to %s", old)
	}

	// The lock and vendor tree are stale after a rename; report it rather than
	// failing a rename whose writes already succeeded.
	if lk, err := ReadLock(root); err == nil {
		if _, ok := lk.Entries[old]; ok {
			warnings = append(warnings, fmt.Sprintf("%s still records %s; run `kigumi get` to fetch and lock %s", LockName, old, next))
		}
	}
	if v, _, err := ReadVendor(root); err == nil {
		if _, ok := v.Entries[old]; ok {
			warnings = append(warnings, fmt.Sprintf("%s still has %s; run `kigumi get` and `kigumi mod vendor`", filepath.Join(VendorDirName, VendorFileName), old))
		}
	}
	return changes, warnings, nil
}

// nearestLocal finds the mod.local.kg a rename must update: the one the
// module would resolve with, at or above root.
func nearestLocal(root string) string {
	for d := filepath.Clean(root); ; d = filepath.Dir(d) {
		path := filepath.Join(d, LocalName)
		if _, err := os.Lstat(path); err == nil {
			return path
		}
		if filepath.Dir(d) == d {
			return ""
		}
	}
}

// sourceFiles lists the .kg sources of the module at root, skipping
// manifests, the vendor tree and nested modules (which own their sources).
func sourceFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == VendorDirName || IsModuleRoot(path) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".kg") && !IsManifest(name) {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// sourceEdit replaces the bytes [start, end) of a file.
type sourceEdit struct {
	start, end int
	text       string
}

// nameFieldEdits replaces the `name` string of the given record kinds when it
// is exactly old. Parsing the record text (not searching for the string)
// leaves comments, aliases and unrelated fields alone.
func nameFieldEdits(name string, src []byte, kinds map[string]bool, old, next string) ([]sourceEdit, error) {
	f := token.NewFile(name, src)
	t := syntax.Parse(f)
	if len(t.Diags) > 0 {
		return nil, fmt.Errorf("%s: %s", name, diag.RenderAll(f, t.Diags[:1]))
	}
	var edits []sourceEdit
	for st := range t.List(t.Root).Views() {
		if st.Kind() != syntax.ExprStmt {
			continue
		}
		expr := st.AsExprStmt().Expr()
		if expr.Kind() != syntax.RecordLit {
			continue
		}
		lit := expr.AsRecordLit()
		if !kinds[lit.Head().Tok().Text()] {
			continue
		}
		for id := range lit.Fields().IDs() {
			if t.Kind(id) != syntax.FieldInit {
				continue
			}
			field := t.View(id).AsFieldInit()
			if field.Name().Text() != "name" {
				continue
			}
			value := field.Value().ID()
			if t.Kind(value) != syntax.StringLit {
				continue
			}
			parts := t.StringParts(value)
			if len(parts) != 1 || parts[0].Expr != 0 {
				continue
			}
			if string(f.Src[parts[0].Text.Start:parts[0].Text.End]) != old {
				continue
			}
			edits = append(edits, sourceEdit{int(parts[0].Text.Start), int(parts[0].Text.End), next})
		}
	}
	return edits, nil
}

// importEdits rewrites the import paths owned by old. A file that already
// imports next would make the rename ambiguous, so it is refused.
func importEdits(name string, src []byte, old, next string) ([]sourceEdit, error) {
	f := token.NewFile(name, src)
	t := syntax.Parse(f)
	if len(t.Diags) > 0 {
		return nil, fmt.Errorf("%s: %s", name, diag.RenderAll(f, t.Diags[:1]))
	}
	var edits []sourceEdit
	for d := range t.List(t.Root).Views() {
		if d.Kind() != syntax.ImportDecl {
			continue
		}
		path := d.AsImportDecl().Path()
		if path.Len() == 0 {
			continue
		}
		text := path.Join("/")
		if text == next || strings.HasPrefix(text, next+"/") {
			return nil, fmt.Errorf("%s already imports %s; a rename cannot merge two modules", name, next)
		}
		if text != old && !strings.HasPrefix(text, old+"/") {
			continue
		}
		first, last := path.At(0).Span(), path.Last().Span()
		edits = append(edits, sourceEdit{int(first.Start), int(last.End), next + strings.TrimPrefix(text, old)})
	}
	return edits, nil
}

// applyEdits splices edits into src; they never overlap because each is one
// import path or one record field.
func applyEdits(src []byte, edits []sourceEdit) []byte {
	sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
	out := append([]byte{}, src...)
	for _, e := range edits {
		next := make([]byte, 0, len(out)-(e.end-e.start)+len(e.text))
		next = append(next, out[:e.start]...)
		next = append(next, e.text...)
		next = append(next, out[e.end:]...)
		out = next
	}
	return out
}
