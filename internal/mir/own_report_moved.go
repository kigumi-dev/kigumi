package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

// reportMoved is a use of a moved local with a note at the move.
func (a *owner) reportMoved(n syntax.NodeID, l LocalID, name string) {
	if n == 0 || a.reported[n] {
		return
	}
	a.reported[n] = true
	t := a.p.R.Tree(a.f.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: diagCode("use-after-move"),
		Msg: "`" + name + "` was moved; it cannot be used again"}
	if at := a.cur.at.get(l); at != 0 && at != n {
		d = d.WithNote(diag.At(t.File, t.Span(at)), "`"+name+"` is moved here")
	}
	a.f.Diags = append(a.f.Diags, d)
}
