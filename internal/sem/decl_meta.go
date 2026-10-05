package sem

import "kigumi/internal/syntax"

// metaAttrs resolves the dotted metadata attributes of a declaration or
// parameter (e.g. `@web.get("/x")`).
func (r *Result) metaAttrs(f FileID, list syntax.NodeID, onType bool) []MetaRef {
	if list == 0 {
		return nil
	}
	t := r.tree(f)
	var dotted []syntax.NodeID
	for _, a := range t.Children(list) {
		if t.Kind(a) == syntax.UnsafeGuarantee {
			continue
		}
		path := syntax.NodeID(t.Nodes[a].Lhs)
		switch name := pathText(t, path, "."); {
		case len(t.PathToks(path)) > 1:
			dotted = append(dotted, a)
		case name == "impl":
			if !onType {
				r.errAt(f, a, cAttrWrongTarget)
			}
		case name == "linkName" || name == "section":
			// Target and argument validation is resolveLinkSection's job.
		case name == "provides":
			if !onType {
				r.errAt(f, a, cAttrWrongTarget)
			}
		case name == "schema":
			if !onType {
				r.errAt(f, a, cSchemaWrongTarget)
			}
		default:
			r.errAt(f, a, cAttrUnknown, name)
		}
	}
	if len(dotted) == 0 {
		return nil
	}
	return r.resolveMetadata(f, 0, t.AddList(syntax.Metadata, t.Nodes[dotted[0]].Tok, dotted))
}

func (r *Result) rejectAttrs(f FileID, list syntax.NodeID) {
	if list == 0 {
		return
	}
	t := r.tree(f)
	for _, a := range t.Children(list) {
		if t.Kind(a) == syntax.UnsafeGuarantee {
			continue
		}
		name := pathText(t, syntax.NodeID(t.Nodes[a].Lhs), ".")
		switch name {
		case "impl", "provides":
			r.errAt(f, a, cAttrWrongTarget)
		case "derivable":
			// declareNominalAttrs already applied this.
		default:
			r.errAt(f, a, cAttrUnknown, name)
		}
	}
}

// resolveMetadata resolves `json.name("id")` heads by ordinary name lookup;
// schema validation belongs to library comptime.
func (r *Result) resolveMetadata(f FileID, fld, meta syntax.NodeID) []MetaRef {
	if meta == 0 {
		return nil
	}
	t := r.tree(f)
	var out []MetaRef
	seen := map[string]bool{}
	for _, item := range t.Children(meta) {
		path := syntax.NodeID(t.Nodes[item].Lhs)
		head := r.resolveValueName(f, r.fileScopes[f], path)
		key := pathText(t, path, ".")
		// sigReady is false for field metadata; checkSchemas re-checks once
		// signatures exist.
		sigReady := head != 0 && r.Entities[head].Kind == EntFn && r.Fn(head).Sig != 0
		if sigReady && seen[key] && !r.schemaRepeatable(head) {
			r.errAt(f, item, cMetadataDuplicate, key)
		}
		seen[key] = true
		if head != 0 {
			r.Files[f].Uses[item] = head
			if sigReady && !r.isSchema(head) {
				r.errAt(f, path, cAttrNotSchema, pathText(t, path, "."))
				continue
			}
		}
		ref := MetaRef{Head: head, Node: item}
		for _, a := range t.Children(syntax.NodeID(t.Nodes[item].Rhs)) {
			if v, ok := r.evalConst(f, a); ok {
				lit, _ := v.literal()
				ref.Args = append(ref.Args, lit)
			}
		}
		out = append(out, ref)
	}
	return out
}
