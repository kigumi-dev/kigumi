package driver

import (
	"fmt"
	"os"
	"path/filepath"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

var scriptFields = map[string][]string{
	"Kigumi":  {"version"},
	"Zig":     {"version"},
	"Require": {"name", "version", "url"},
	"Link":    {"library", "search"},
}

// scriptManifest reads a script's leading manifest records and removes them from the tree so the checker never sees them; inside a module they belong in mod.kg.
func (m *Module) scriptManifest(hasMod bool) error {
	for _, p := range m.Packages {
		for _, t := range p.Files {
			n := leadingRecords(t)
			if n == 0 {
				continue
			}
			if hasMod {
				return fmt.Errorf("%s: manifest records go in %s, not in a file of a module", t.File.Name, ManifestName)
			}
			if m.ScriptFile != "" {
				return fmt.Errorf("%s and %s both carry manifest records; only the entry may", m.ScriptFile, t.File.Name)
			}
			stmts := t.List(t.Root)
			recs, ds := recordsOf(t, stmts, n, scriptFields)
			mf, more := modFileOf(recs, false)
			ds = append(ds, more...)
			if mf.Kigumi == "" && len(ds) == 0 {
				ds = append(ds, diag.Errorf(diag.Location{Span: stmts.At(0).Span()}, "a script with manifest records needs `Kigumi { version: \"...\" }` first"))
			}
			if len(ds) > 0 {
				return fmt.Errorf("%s", diag.RenderAll(t.File, ds[:1]))
			}
			t.DropRootItems(n)
			m.Manifest, m.ScriptFile = mf, t.File.Name
		}
	}
	return nil
}

// leadingRecords counts the manifest records at the top of a file.
func leadingRecords(t *syntax.Tree) int {
	n := 0
	for st := range t.List(t.Root).Views() {
		if st.Kind() != syntax.ExprStmt {
			break
		}
		lit := st.AsExprStmt().Expr()
		if lit.Kind() != syntax.RecordLit {
			break
		}
		head := lit.AsRecordLit().Head()
		if head.Kind() != syntax.Ident {
			break
		}
		if _, ok := scriptFields[head.Tok().Text()]; !ok {
			break
		}
		n++
	}
	return n
}

// ReadScriptManifest reads the leading records of a script file on disk;
// ok is false when it carries none.
func ReadScriptManifest(path string) (ModFile, bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return ModFile{}, false, err
	}
	return parseScriptManifest(path, src)
}

// parseScriptManifest is ReadScriptManifest's decoding, split out so a
// writer can re-parse a prospective next version before committing it.
func parseScriptManifest(path string, src []byte) (ModFile, bool, error) {
	t := syntax.Parse(token.NewFile(filepath.Base(path), src))
	n := leadingRecords(t)
	if n == 0 {
		return ModFile{}, false, nil
	}
	recs, ds := recordsOf(t, t.List(t.Root), n, scriptFields)
	mf, more := modFileOf(recs, false)
	if ds = append(ds, more...); len(ds) > 0 {
		return mf, true, fmt.Errorf("%s: %s", path, diag.RenderAll(t.File, ds[:1]))
	}
	return mf, true, nil
}
