package driver

import "kigumi/internal/compiler/syntax"

func importPaths(t *syntax.Tree) []string {
	var out []string
	for d := range t.List(t.Root).Views() {
		if d.Kind() != syntax.ImportDecl {
			continue
		}
		out = append(out, d.AsImportDecl().Path().Join("/"))
	}
	return out
}
