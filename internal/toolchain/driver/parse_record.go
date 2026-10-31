package driver

import (
	"bytes"
	"fmt"
	"slices"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// record is one top-level record literal of a manifest-like file.
type record struct {
	Kind   string
	Fields map[string]string
	Span   token.Span
}

// parseRecords reads the top-level record literals of f. allowed maps each
// record kind to its string fields; "name" is required where it is a field.
func parseRecords(f *token.File, allowed map[string][]string) ([]record, []diag.Diagnostic) {
	t := syntax.Parse(f)
	root := t.List(t.Root)
	recs, ds := recordsOf(t, root, root.Len(), allowed)
	return recs, append(append([]diag.Diagnostic{}, t.Diags...), ds...)
}

// recordsOf decodes the first n top-level statements of an already parsed
// tree (a manifest file, or the leading records of a script).
func recordsOf(t *syntax.Tree, stmts syntax.ListView, n int, allowed map[string][]string) ([]record, []diag.Diagnostic) {
	f := t.File
	var ds []diag.Diagnostic
	var recs []record
	for i := range n {
		st := stmts.At(i)
		span := st.Span()
		if st.Kind() != syntax.ExprStmt || st.AsExprStmt().Expr().Kind() != syntax.RecordLit {
			ds = append(ds, diag.Errorf(diag.Location{Span: span}, "expected a record such as `Require { name: \"x\", version: \"v1.0.0\" }`"))
			continue
		}
		lit := st.AsExprStmt().Expr().AsRecordLit()
		head := lit.Head()
		kind := head.Tok().Text()
		fields, ok := allowed[kind]
		if head.Kind() != syntax.Ident || !ok {
			ds = append(ds, diag.Errorf(diag.Location{Span: head.Span()}, fmt.Sprintf("unknown record %s", kind)))
			continue
		}
		// Closing tokens are not nodes, so widen the span to the `}`.
		if i := bytes.IndexByte(f.Src[span.End:], '}'); i >= 0 {
			span.End += token.Pos(i) + 1
		}
		r := record{Kind: kind, Fields: map[string]string{}, Span: span}
		for e := range lit.Fields().IDs() {
			ds = r.field(t, e, fields, ds)
		}
		if r.Fields["name"] == "" && slices.Contains(fields, "name") {
			ds = append(ds, diag.Errorf(diag.Location{Span: span}, fmt.Sprintf("%s needs a name", kind)))
		}
		recs = append(recs, r)
	}
	return recs, ds
}

func (r *record) field(t *syntax.Tree, e syntax.NodeID, allowed []string, ds []diag.Diagnostic) []diag.Diagnostic {
	if t.Kind(e) != syntax.FieldInit {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(e)}, "manifest records take `field: \"value\"` entries only"))
	}
	entry := t.View(e).AsFieldInit()
	name := entry.Name().Text()
	if !slices.Contains(allowed, name) {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(e)}, fmt.Sprintf("%s has no field %s", r.Kind, name)))
	}
	if _, dup := r.Fields[name]; dup {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(e)}, fmt.Sprintf("field %s given twice", name)))
	}
	val := entry.Value().ID()
	s, ok := plainString(t, val)
	if !ok {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(val)}, fmt.Sprintf("%s must be a plain string literal", name)))
	}
	r.Fields[name] = s
	return ds
}

// plainString decodes a string literal without interpolation.
func plainString(t *syntax.Tree, n syntax.NodeID) (string, bool) {
	if t.Kind(n) != syntax.StringLit {
		return "", false
	}
	out := ""
	for _, p := range t.StringParts(n) {
		if p.Expr != 0 {
			return "", false
		}
		out += syntax.DecodeString(string(t.File.Src[p.Text.Start:p.Text.End]))
	}
	return out, true
}
