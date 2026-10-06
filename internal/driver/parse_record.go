package driver

import (
	"bytes"
	"fmt"
	"slices"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
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
	recs, ds := recordsOf(t, t.Children(t.Root), allowed)
	return recs, append(append([]diag.Diagnostic{}, t.Diags...), ds...)
}

// recordsOf decodes the given top-level statements of an already parsed
// tree (a manifest file, or the leading records of a script).
func recordsOf(t *syntax.Tree, stmts []syntax.NodeID, allowed map[string][]string) ([]record, []diag.Diagnostic) {
	f := t.File
	var ds []diag.Diagnostic
	var recs []record
	for _, st := range stmts {
		span := t.Span(st)
		if t.Kind(st) != syntax.ExprStmt || t.Kind(syntax.NodeID(t.Nodes[st].Lhs)) != syntax.RecordLit {
			ds = append(ds, diag.Errorf(diag.Location{Span: span}, "expected a record such as `Require { name: \"x\", version: \"v1.0.0\" }`"))
			continue
		}
		lit := syntax.NodeID(t.Nodes[st].Lhs)
		head := syntax.NodeID(t.Nodes[lit].Lhs)
		kind := t.TokText(t.Nodes[head].Tok)
		fields, ok := allowed[kind]
		if t.Kind(head) != syntax.Ident || !ok {
			ds = append(ds, diag.Errorf(diag.Location{Span: t.Span(head)}, fmt.Sprintf("unknown record %s", kind)))
			continue
		}
		// Closing tokens are not nodes, so widen the span to the `}`.
		if i := bytes.IndexByte(f.Src[span.End:], '}'); i >= 0 {
			span.End += token.Pos(i) + 1
		}
		r := record{Kind: kind, Fields: map[string]string{}, Span: span}
		for _, e := range t.Children(syntax.NodeID(t.Nodes[lit].Rhs)) {
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
	name := t.TokText(t.Nodes[e].Tok)
	if !slices.Contains(allowed, name) {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(e)}, fmt.Sprintf("%s has no field %s", r.Kind, name)))
	}
	if _, dup := r.Fields[name]; dup {
		return append(ds, diag.Errorf(diag.Location{Span: t.Span(e)}, fmt.Sprintf("field %s given twice", name)))
	}
	val := syntax.NodeID(t.Nodes[e].Lhs)
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
