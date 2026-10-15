package sem

import "kigumi/internal/syntax"

// checkSchemas re-validates and re-checks duplicates for field metadata
// once signatures are known (resolveMetadata ran before they existed).
func (r *Result) checkSchemas(f FileID, refs []MetaRef) {
	t := r.tree(f)
	seen := map[string]bool{}
	for _, m := range refs {
		path := t.View(m.Node).AsMetaItem().Path()
		key := path.Join(".")
		if m.Head != 0 {
			if !r.isSchema(m.Head) {
				r.errAt(f, path.ID(), cAttrNotSchema, key)
				continue
			}
			if seen[key] && !r.schemaRepeatable(m.Head) {
				r.errAt(f, m.Node, cMetadataDuplicate, key)
			}
		}
		seen[key] = true
	}
}

// isSchema accepts a function whose return type is marked `@schema`; a
// same-named but unmarked type does not qualify.
func (r *Result) isSchema(head EntityID) bool {
	e := &r.Entities[head]
	if e.Kind != EntFn {
		return false
	}
	n := r.Types.Node(r.Types.Node(r.Fn(head).Sig).Elem)
	return n.Kind == KNamed && r.typeDecl(n.Ent).Schema
}

// schemaRepeatable lets the same metadata item appear more than once
// without cMetadataDuplicate.
func (r *Result) schemaRepeatable(head EntityID) bool {
	if head == 0 || r.Entities[head].Kind != EntFn {
		return false
	}
	n := r.Types.Node(r.Types.Node(r.Fn(head).Sig).Elem)
	return n.Kind == KNamed && r.typeDecl(n.Ent).SchemaRepeatable
}

// schemaAttr resolves `@schema(repeatable, ordered, target)`: up to three
// optional positional literals (Bool, Bool, String).
func (r *Result) schemaAttr(id EntityID, f FileID, a syntax.NodeID) {
	e := &r.Entities[id]
	if e.Kind != EntType {
		r.errAt(f, a, cSchemaWrongTarget)
		return
	}
	info := r.typeDecl(id)
	if info.Schema {
		r.errAt(f, a, cSchemaDuplicate)
		return
	}
	info.Schema = true
	t := r.tree(f)
	args := t.View(a).AsAttr().Args()
	lits := make([]Literal, 0, args.Len())
	for arg := range args.IDs() {
		v, ok := r.evalConst(f, arg)
		if !ok {
			r.errAt(f, arg, cSchemaBadArg)
			return
		}
		lit, ok := v.literal()
		if !ok {
			r.errAt(f, arg, cSchemaBadArg)
			return
		}
		lits = append(lits, lit)
	}
	if len(lits) > 0 && lits[0].Kind == LitBool {
		info.SchemaRepeatable = lits[0].Bool
	} else if len(lits) > 0 {
		r.errAt(f, a, cSchemaBadArg)
	}
	if len(lits) > 1 && lits[1].Kind == LitBool {
		info.SchemaOrdered = lits[1].Bool
	} else if len(lits) > 1 {
		r.errAt(f, a, cSchemaBadArg)
	}
	if len(lits) > 2 && lits[2].Kind == LitString {
		info.SchemaTarget = lits[2].Str
	} else if len(lits) > 2 {
		r.errAt(f, a, cSchemaBadArg)
	}
	if len(lits) > 3 {
		r.errAt(f, a, cSchemaBadArg)
	}
}

// derivableAttr marks an interface derivable without a compiler-side
// package/name table.
func (r *Result) derivableAttr(id EntityID, f FileID, a syntax.NodeID) {
	info := r.iface(id)
	if info.Derivable {
		r.errAt(f, a, cDerivableDuplicate)
		return
	}
	info.Derivable = true
}
