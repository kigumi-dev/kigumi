package sem

import (
	"bytes"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// namedOutside reports that a token outside the import declarations spells name, so a skipped body may use it.
func (r *Result) namedOutside(f FileID, name string) bool {
	t := r.tree(f)
	from, inImport := uint32(1), false
	for decl := range t.View(t.Root).Children() {
		lo, _ := tokRange(t, decl.ID())
		if !inImport && spells(t, from, lo, name) {
			return true
		}
		// An import's path tokens are no node's own token, so the import runs up to the next declaration.
		from, inImport = lo, decl.Kind() == syntax.ImportDecl
	}
	return !inImport && spells(t, from, uint32(len(t.Toks)), name)
}

func spells(t *syntax.Tree, lo, hi uint32, name string) bool {
	for i := lo; i < hi && t.Toks[i].Kind != token.EOF; i++ {
		text := t.File.Src[t.Toks[i].Start:t.Toks[i].End]
		if string(text) == name || (t.Toks[i].Kind == token.String && bytes.Contains(text, []byte(name))) {
			return true
		}
	}
	return false
}

// BodySkipped reports a function whose body planLazyBodies left unchecked.
func (r *Result) BodySkipped(id EntityID) bool {
	return int(id) < len(r.skipBody) && r.skipBody[id]
}

// LazyMissed reports that a checked body names a skipped body or a lazy package reported something, so the result cannot stand.
func (r *Result) LazyMissed() bool {
	if r.skipBody == nil {
		return false
	}
	for f := 1; f < len(r.Files); f++ {
		fi := &r.Files[f]
		if r.lazyPkg[fi.Pkg] && len(fi.Diags) > 0 {
			return true
		}
		for _, id := range fi.Uses {
			if r.BodySkipped(id) {
				return true
			}
		}
		for _, ws := range fi.CoerceWitness {
			for _, id := range ws {
				if r.BodySkipped(id) {
					return true
				}
			}
		}
	}
	for i := range r.Fns {
		for _, edge := range r.Fns[i].Edges {
			if r.BodySkipped(edge.Target) {
				return true
			}
		}
	}
	for i := range r.Closures {
		for _, edge := range r.Closures[i].Edges {
			if r.BodySkipped(edge.Target) {
				return true
			}
		}
	}
	return false
}

// SkippedBodies counts the bodies planLazyBodies left unchecked.
func (r *Result) SkippedBodies() int {
	n := 0
	for _, skip := range r.skipBody {
		if skip {
			n++
		}
	}
	return n
}
