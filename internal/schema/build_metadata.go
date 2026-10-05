package schema

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// metadataOf resolves SchemaID from the schema function itself, never from
// Name.
func metadataOf(res *sem.Result, t *syntax.Tree, refs []sem.MetaRef) []Metadata {
	out := make([]Metadata, 0, len(refs))
	for _, m := range refs {
		item := Metadata{Name: dottedName(t, m.Node)}
		if ref := declRefOf(res, m.Head); ref != nil {
			item.SchemaID = ref.ID
		}
		for _, lit := range m.Args {
			item.Args = append(item.Args, literalValue(lit))
		}
		out = append(out, item)
	}
	return out
}

func dottedName(t *syntax.Tree, item syntax.NodeID) string {
	path := syntax.NodeID(t.Nodes[item].Lhs)
	var segs string
	for _, tk := range t.PathToks(path) {
		if segs != "" {
			segs += "."
		}
		segs += t.TokText(tk)
	}
	return segs
}

func literalValue(lit sem.Literal) any {
	switch lit.Kind {
	case sem.LitInt:
		if lit.Int.IsInt64() {
			return lit.Int.Int64()
		}
		return lit.Int.String()
	case sem.LitFloat:
		f, _ := lit.Float.Float64()
		return f
	case sem.LitBool:
		return lit.Bool
	case sem.LitChar:
		return string(lit.Char)
	}
	return lit.Str
}

// schemaDeclOf reports a's SchemaDecl when it was declared `@schema`.
func schemaDeclOf(res *sem.Result, ent sem.EntityID) *SchemaDecl {
	info := res.TypeDecl(ent)
	if !info.Schema {
		return nil
	}
	return &SchemaDecl{Repeatable: info.SchemaRepeatable, Ordered: info.SchemaOrdered, Target: info.SchemaTarget}
}
