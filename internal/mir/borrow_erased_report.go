package mir

import (
	"maps"
	"slices"
	"strings"

	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

// leaving reports a holder of a captured cell that leaves this closure body (returned, stored, passed on to keep), where
// the enclosing body's writes, which this body cannot see, may reach it.
func (e *erasedReaders) leaving() {
	outer := func(l LocalID) (LocalID, bool) {
		for _, cp := range e.caps[l] {
			if slices.Contains(e.f.Params, cp.root) {
				return cp.root, true
			}
		}
		return 0, false
	}
	for _, blk := range e.f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == OpDrop {
				continue
			}
			for _, h := range e.escapes(in, e.caps) {
				if r, ok := outer(h); ok {
					e.noteLeave(in.Node, r, h)
				}
			}
		}
		if blk.Term.Op != TermReturn {
			continue
		}
		for _, a := range blk.Term.Args {
			if r, ok := outer(a); ok {
				e.noteLeave(definedAt(blk, a, e.site[a]), r, a)
			}
		}
	}
}

// storeValue is the value a store puts into its place.
func storeValue(in Inst) (LocalID, bool) {
	if in.Op != OpCellSet && in.Op != OpSetField && in.Op != OpSetIndex || len(in.Args) < 2 {
		return 0, false
	}
	return in.Args[len(in.Args)-1], true
}

// definedAt is the node of the last instruction in blk that writes l, else fallback.
func definedAt(blk Block, l LocalID, fallback syntax.NodeID) syntax.NodeID {
	for i := len(blk.Insts) - 1; i >= 0; i-- {
		if in := blk.Insts[i]; in.Op.Yields() && in.Dst == l && in.Node != 0 {
			return in.Node
		}
	}
	return fallback
}

func (e *erasedReaders) noteLeave(n syntax.NodeID, root, holder LocalID) {
	if at := e.site[holder]; at != 0 && n != 0 {
		if _, ok := e.found[at]; !ok {
			e.found[at] = erasedWrite{write: n, root: root, how: leavesBody}
		}
	}
}

func (e *erasedReaders) note(n syntax.NodeID, root, holder LocalID) {
	at := e.site[holder]
	if at == 0 {
		at = n
	}
	t := e.p.R.Tree(e.f.File)
	if w, ok := e.found[at]; n == 0 || ok && t.Span(w.write).Start <= t.Span(n).Start {
		return
	}
	e.found[at] = erasedWrite{write: n, root: root}
}

func (e *erasedReaders) flush() {
	t := e.p.R.Tree(e.f.File)
	sites := slices.Collect(maps.Keys(e.found))
	slices.SortFunc(sites, func(a, b syntax.NodeID) int { return int(t.Span(a).Start) - int(t.Span(b).Start) })
	for _, at := range sites {
		w := e.found[at]
		name := "`" + e.f.Locals[w.root].Name + "`"
		d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(at)), Code: diagCode("closure-not-plain-fn"),
			Msg: "closure captures " + name + " (written outside this closure, so the closure shares it through a cell) and cannot be a plain `fn`"}
		text := erasedTexts[w.how]
		d = d.WithNote(diag.At(t.File, t.Span(w.write)), strings.ReplaceAll(text.note, "{name}", name))
		d = d.WithHelp(strings.ReplaceAll(text.help, "{name}", name))
		e.f.Diags = append(e.f.Diags, d)
	}
}

// erasedHow is how a write reaches the cell a plain `fn` reads after the closure was erased.
type erasedHow uint8

const (
	writtenLive erasedHow = iota
	leavesBody
	lentBeside
)

var erasedTexts = [...]struct{ note, help string }{
	writtenLive: {"{name} is written here while the plain `fn` may still run",
		"a plain `fn` cannot share a captured variable's cell; use it before writing {name}, or bind the closure without a `fn` type"},
	leavesBody: {"the plain `fn` leaves this closure here, and the enclosing body may write {name} before it runs",
		"a plain `fn` cannot share a captured variable's cell; use it before writing {name}, or bind the closure without a `fn` type"},
	lentBeside: {"this argument of the same call may write {name} while the call runs the plain `fn`",
		"a plain `fn` cannot share a captured variable's cell; read what it needs from {name} into a local before the call, or take the callback as `F: FnMut[...]` and declare the parameter `mut callback: F`"},
}
