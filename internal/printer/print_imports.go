package printer

import (
	"sort"
	"strings"

	"kigumi/internal/syntax"
)

// sortImports orders the file's first run of import declarations: std, then host-qualified paths, then `pub import`, each by path.
// Comments above an import or at the end of its line travel with it; tight marks the imports printed without a blank line before them.
func (p *printer) sortImports(decls []syntax.NodeID) (out []syntax.NodeID, tight map[syntax.NodeID]bool, carried *importRun) {
	tight = map[syntax.NodeID]bool{}
	start := -1
	end := 0
	for i, d := range decls {
		if p.t.Kind(d) == syntax.ImportDecl {
			if start < 0 {
				start = i
			}
			end = i + 1
		} else if start >= 0 {
			break
		}
	}
	if start < 0 || end-start < 2 {
		return decls, tight, nil
	}
	carried, ok := p.collectImportTrivia(decls, start, end)
	if !ok {
		return decls, tight, nil
	}
	run := append([]syntax.NodeID{}, decls[start:end]...)
	sort.SliceStable(run, func(i, j int) bool {
		gi, gj := p.importGroup(run[i]), p.importGroup(run[j])
		if gi != gj {
			return gi < gj
		}
		return p.importPath(run[i]) < p.importPath(run[j])
	})
	out = append(append(append([]syntax.NodeID{}, decls[:start]...), run...), decls[end:]...)
	for _, d := range run[1:] {
		tight[d] = true
	}
	carried.first = start
	return out, tight, carried
}

func (p *printer) importGroup(d syntax.NodeID) int {
	if p.t.View(d).AsImportDecl().Vis().Valid() {
		return 2
	}
	if strings.HasPrefix(p.importPath(d), "std/") || p.importPath(d) == "std" {
		return 0
	}
	return 1
}

func (p *printer) importPath(d syntax.NodeID) string {
	return p.t.View(d).AsImportDecl().Path().Join("/")
}

// sortedNames orders the `{a, b}` binding list of an import.
func (p *printer) sortedNames(names []syntax.NodeID) []syntax.NodeID {
	out := append([]syntax.NodeID{}, names...)
	sort.SliceStable(out, func(i, j int) bool { return p.t.View(out[i]).Tok().Text() < p.t.View(out[j]).Tok().Text() })
	return out
}
