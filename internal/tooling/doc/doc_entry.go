package doc

import (
	"strings"

	"kigumi/internal/compiler/syntax"
)

type entry struct {
	head, doc string
	kind      syntax.NodeKind
	name      string
	recv      string
}

// declEntries lists what a top-level node documents: a declaration or `pub import`, or each member of an `extern`/`export` block.
// Statements, private imports and, unless all is set, private declarations give none.
func declEntries(t *syntax.Tree, d syntax.NodeID, all bool) []entry {
	switch t.Kind(d) {
	case syntax.FnDecl, syntax.TypeDecl, syntax.InterfaceDecl, syntax.ConstDecl, syntax.StaticDecl:
		if e, ok := declEntry(t, d, all, ""); ok {
			return []entry{e}
		}
	case syntax.ImportDecl:
		if e, ok := importEntry(t, d); ok {
			return []entry{e}
		}
	case syntax.AbiBlock:
		return abiEntries(t, d, all)
	}
	return nil
}

func declEntry(t *syntax.Tree, d syntax.NodeID, all bool, abi string) (entry, bool) {
	kind := t.Kind(d)
	decl := t.View(d).AsDecl()
	if !all && !decl.Vis().Valid() {
		return entry{}, false
	}
	name := decl.Name()
	if !name.Valid() {
		return entry{}, false
	}
	e := entry{kind: kind, name: name.Text(), head: declHead(t, d, all), doc: syntax.DocText(t, d)}
	if abi != "" {
		e.head = abi + " {\n    " + strings.ReplaceAll(e.head, "\n", "\n    ") + "\n}"
	}
	if kind == syntax.FnDecl {
		if recv := decl.AsFnDecl().Recv(); recv.Valid() {
			sp := recv.Span()
			e.recv = ownerName(string(t.File.Src[sp.Start:sp.End]))
		}
	}
	return e, true
}

func abiEntries(t *syntax.Tree, d syntax.NodeID, all bool) []entry {
	blk := t.View(d).AsAbiBlock()
	abi := t.TokText(blk.Tok().Index()) + "(" + blk.Abis().At(0).Tok().Text() + ")"
	var out []entry
	for m := range blk.Decls().IDs() {
		if k := t.Kind(m); k != syntax.FnDecl && k != syntax.StaticDecl {
			continue
		}
		if e, ok := declEntry(t, m, all, abi); ok {
			out = append(out, e)
		}
	}
	return out
}

func importEntry(t *syntax.Tree, d syntax.NodeID) (entry, bool) {
	imp := t.View(d).AsImportDecl()
	if !imp.Vis().Valid() {
		return entry{}, false
	}
	sp := t.Span(d)
	lines := t.File.LineSpan(t.File.Line(sp.Start))
	end := t.File.LineSpan(t.File.Line(max(sp.End, sp.Start+1) - 1))
	head := strings.TrimSpace(string(t.File.Src[lines.Start:end.End]))
	return entry{kind: syntax.ImportDecl, name: imp.Path().Join("/"), head: head}, true
}

// declHead is the declaration as written, minus the private fields of a record when all is unset.
func declHead(t *syntax.Tree, d syntax.NodeID, all bool) string {
	if all {
		return syntax.DeclHead(t, d)
	}
	return syntax.PublicDeclHead(t, d)
}

// ownerName strips type arguments: `Array[T]` and `Map[K, V]` -> `Array`.
func ownerName(recv string) string {
	if i := strings.IndexByte(recv, '['); i >= 0 {
		return recv[:i]
	}
	return recv
}
