package cheader

import "strings"

// A renamed function keeps its C symbol through @linkName.
func renameKeyword(it item, taken map[string]bool) item {
	if it.skip != "" || it.kind != "fn" && it.kind != "const" || !kigumiKeywords[it.name] {
		return it
	}
	name := it.name + "_"
	for taken[name] {
		name += "_"
	}
	taken[name] = true
	head := it.kind + " " + it.name
	it.text = strings.Replace(it.text, head, it.kind+" "+name, 1)
	if it.kind == "fn" {
		it.link = it.name
	}
	it.name = name
	return it
}

func declaredNames(full string) map[string]bool {
	taken := map[string]bool{}
	for _, c := range splitTopLevel(full) {
		if _, n, ok := declHead(c); ok {
			taken[unquoteIdent(n)] = true
		}
	}
	return taken
}
