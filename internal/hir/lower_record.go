package hir

import (
	"fmt"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// A spread lists the fields it fills, in declaration order, skipping the
// ones already set before it.
func (l *lowerer) record(n syntax.NodeID) *Expr {
	call := l.info.Calls[n]
	info := l.r.TypeDecl(call.Callee)
	out := l.node(n, Record)
	out.Ent = call.Callee
	set := make([]bool, len(info.Fields))
	for en := range l.t.View(n).AsRecordLit().Fields().Views() {
		entry := en.ID()
		value := l.entryValue(en)
		re := &RecordEntry{Node: entry, Value: l.expr(value)}
		if en.Kind() != syntax.Spread {
			re.Index = l.r.Field(l.info.Uses[entry]).Index
			set[re.Index] = true
			out.Entries = append(out.Entries, re)
			continue
		}
		re.Spread = true
		srcType := l.typeOf(value)
		if l.r.Types.Node(srcType).Kind == sem.KRef {
			srcType = l.r.Types.Node(srcType).Elem
		}
		srcEnt := l.r.Types.Node(srcType).Ent
		for i, fld := range info.Fields {
			if set[i] {
				continue
			}
			sf := l.r.FindField(srcEnt, l.r.Entity(fld).Name)
			if sf == 0 {
				continue
			}
			re.Fields = append(re.Fields, SpreadField{Index: i, SrcIndex: l.r.Field(sf).Index, Type: l.r.Entity(fld).Type})
			set[i] = true
		}
		out.Entries = append(out.Entries, re)
	}
	return out
}

func (l *lowerer) entryValue(entry syntax.View) syntax.NodeID {
	if entry.Kind() == syntax.Spread {
		return entry.AsSpread().Value().ID()
	}
	return entry.AsFieldInit().Value().ID()
}

// tupleLit reuses Record's shape over the checker's predeclared TupleN
// entity; each element sets the field at its position.
func (l *lowerer) tupleLit(n syntax.NodeID) *Expr {
	call := l.info.Calls[n]
	out := l.node(n, Record)
	out.Ent = call.Callee
	for i, iv := range l.t.List(n).Enumerate() {
		item := iv.ID()
		out.Entries = append(out.Entries, &RecordEntry{Node: item, Index: i, Value: l.expr(item)})
	}
	return out
}

// shellLit records the plan words: each text piece as a string, each
// interpolation as an argument; "|" separates commands.
func (l *lowerer) shellLit(n syntax.NodeID) *Expr {
	out := l.node(n, Shell)
	for i, cmd := range l.t.View(n).AsShellLit().Plan().Enumerate() {
		if i > 0 {
			out.Strs = append(out.Strs, "|")
		}
		for part := range cmd.AsShellCmd().Views() {
			word := part
			if part.Kind() == syntax.ShellRedirect {
				r := part.AsShellRedirect()
				out.Strs = append(out.Strs, fmt.Sprintf("r%d:%d", r.Op(), r.Fd()))
				word = r.Target().View
			} else {
				out.Strs = append(out.Strs, " ")
			}
			for piece := range word.AsShellWord().Views() {
				if piece.Kind() == syntax.ShellText {
					text := piece.AsShellText()
					out.Strs = append(out.Strs, "="+syntax.DecodeString(string(l.t.File.Src[text.Start():text.End()])))
					continue
				}
				out.Strs = append(out.Strs, "$")
				out.Args = append(out.Args, l.expr(piece.AsShellInterp().Expr().ID()))
			}
		}
	}
	return out
}
