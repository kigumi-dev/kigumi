package module

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

// ScriptManifest decodes the leading manifest records of a parsed script; n is how many top-level statements they take.
func ScriptManifest(t *syntax.Tree) (mf ModFile, n int, ds []diag.Diagnostic) {
	n = leadingRecords(t)
	if n == 0 {
		return ModFile{}, 0, nil
	}
	recs, ds := recordsOf(t, t.List(t.Root), n, scriptFields)
	mf, more := modFileOf(recs, false)
	return mf, n, append(ds, more...)
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
	mf, n, ds := ScriptManifest(t)
	if n == 0 {
		return ModFile{}, false, nil
	}
	if len(ds) > 0 {
		return mf, true, fmt.Errorf("%s: %s", path, diag.RenderAll(t.File, ds[:1]))
	}
	return mf, true, nil
}
