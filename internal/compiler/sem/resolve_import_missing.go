package sem

import (
	"slices"
	"strings"

	"kigumi/internal/compiler/syntax"
)

func (r *Result) packageNotFound(f FileID, t *syntax.Tree, s importSlots, path string) {
	if d, ok := r.diagAt(f, s.Path, cUnresolvedPackage, path); ok {
		if r.inNestedModule(path) {
			d = d.WithNote(r.loc(f, r.Files[f].span(s.Path)), "`"+path+"` is a separate module (it has its own mod.kg), so it is not a package of this one")
			d.Help = append(d.Help, "add it as a dependency (`kigumi get`) and import it by its module name, or remove its mod.kg to make it a package of this module")
		}
		r.emitDiag(f, d)
	}
	r.markImportFailed(f, t, s)
}

// markImportFailed keeps a rejected import's bound names from cascading into E100 at every use.
func (r *Result) markImportFailed(f FileID, t *syntax.Tree, s importSlots) {
	if r.failedImports == nil {
		r.failedImports = map[FileID]map[string]bool{}
	}
	if r.failedImports[f] == nil {
		r.failedImports[f] = map[string]bool{}
	}
	if t.Kind(s.Binding) != syntax.List {
		r.failedImports[f][t.View(s.Binding).Tok().Text()] = true
		return
	}
	for n := range t.List(s.Binding).IDs() {
		if t.Kind(n) == syntax.ImportAlias {
			r.failedImports[f][t.View(n).AsImportAlias().Alias().Name().Text()] = true
		} else {
			r.failedImports[f][t.View(n).Tok().Text()] = true
		}
	}
}

func (r *Result) inNestedModule(path string) bool {
	return r.mod != nil && slices.ContainsFunc(r.mod.NestedModules, func(m string) bool {
		return path == m || strings.HasPrefix(path, m+"/")
	})
}

func (r *Result) afterFailedImport(f FileID, c Code, args []any) bool {
	if c.Name != cUnresolvedName.Name || len(args) == 0 {
		return false
	}
	name, _ := args[0].(string)
	return r.failedImports[f][name]
}
