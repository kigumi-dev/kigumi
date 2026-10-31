package sem

import "kigumi/internal/compiler/syntax"

// markImportsUsed keeps a rejected initializer from also reporting its imports as unused.
func (r *Result) markImportsUsed(f FileID, v syntax.View) {
	if v.Kind() == syntax.Ident {
		if b, _, ok := r.lookup(r.fileScopes[f], v.Tok().Text()); ok && b.Ent != 0 && r.Entities[b.Ent].Kind == EntImport {
			r.Entities[b.Ent].Flags |= EfUsed
		}
	}
	for child := range v.Children() {
		r.markImportsUsed(f, child)
	}
}
