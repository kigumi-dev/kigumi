package syntax

import (
	"iter"

	"kigumi/internal/token"
)

// Children yields the child nodes of v in slot order; absent slots are skipped.
func (v View) Children() iter.Seq[View] {
	return func(yield func(View) bool) {
		t := v.t
		n := t.nodes[v.id]
		info := &kindTable[n.Kind]
		child := func(c uint32) bool { return c == 0 || yield(View{t, NodeID(c)}) }
		switch info.layout {
		case layL:
			child(n.Lhs)
		case layLR:
			if child(n.Lhs) {
				child(n.Rhs)
			}
		case layFlagR, layTokR:
			child(n.Rhs)
		case layList:
			for _, c := range t.extra[n.Lhs:n.Rhs] {
				if !yield(View{t, NodeID(c)}) {
					return
				}
			}
		case layRec:
			for _, i := range info.children {
				if !child(t.extra[n.Lhs+uint32(i)]) {
					return
				}
			}
		}
	}
}

// HasErrors reports whether parsing produced at least one error.
func (t *Tree) HasErrors() bool {
	for _, d := range t.Diags {
		if d.Severity == 0 {
			return true
		}
	}
	return false
}

// HasTopLevelStatements reports whether the file is an entry file candidate:
// it has a bare statement or `let` at top level.
func (t *Tree) HasTopLevelStatements() bool {
	for d := range t.List(t.Root).IDs() {
		switch t.Kind(d) {
		case LetStmt, ExprStmt, AssignStmt, DeferStmt, DeferFailStmt:
			return true
		}
	}
	return false
}

// HasExplicitMain reports whether the file declares the other entry form:
// a top-level, receiverless `fn main`.
func (t *Tree) HasExplicitMain() bool {
	for d := range t.List(t.Root).Views() {
		if d.Kind() != FnDecl || d.AsFnDecl().Recv().Valid() {
			continue
		}
		if name := d.AsFnDecl().Name(); name.Valid() && name.Text() == "main" {
			return true
		}
	}
	return false
}

// VisKind decodes a Visibility node: kind is "pub", "module", "super" or "in";
// path is the ancestor Path for "in".
func (t *Tree) VisKind(id NodeID) (kind string, path NodeID) {
	n := t.nodes[id]
	if n.Kind != Visibility {
		return "", 0
	}
	if n.Lhs == 0 {
		return "pub", 0
	}
	return t.TokText(n.Lhs), NodeID(n.Rhs)
}

// StringPart is one piece of a string literal: literal text (still escaped,
// decode with DecodeString) or an interpolated expression.
type StringPart struct {
	Text token.Span
	Expr NodeID
}

// StringParts splits a StringLit into text spans and interpolation nodes.
func (t *Tree) StringParts(id NodeID) []StringPart {
	n := t.nodes[id]
	tk := t.Toks[n.Tok]
	exprs := t.View(id).AsStringLit().Interps()
	var parts []StringPart
	pos := int(tk.Start) + 1
	for i, r := range interpolationRanges(t.File.Src, int(tk.Start), int(tk.End), t.interp) {
		if start := r[0] - 2; start > pos {
			parts = append(parts, StringPart{Text: token.Span{Start: token.Pos(pos), End: token.Pos(start)}})
		}
		if i < exprs.Len() {
			parts = append(parts, StringPart{Expr: exprs.ItemID(i)})
		}
		pos = r[1] + 1
	}
	if end := int(tk.End) - 1; end > pos {
		parts = append(parts, StringPart{Text: token.Span{Start: token.Pos(pos), End: token.Pos(end)}})
	}
	return parts
}
