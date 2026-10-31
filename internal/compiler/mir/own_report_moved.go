package mir

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
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
	a.f.Diags = append(a.f.Diags, d.WithHelp(a.moveHelp(l)))
}

func (a *owner) moveHelp(l LocalID) string {
	if a.p.R.HoldsResource(a.f.Locals[l].Type) {
		return "borrow it with `&` where it is used before the move, or make the move its last use"
	}
	return "clone it before the move, borrow it with `&`, or make the move its last use"
}

func (a *owner) reportLoopMove(n syntax.NodeID, name string) {
	if n == 0 || a.reported[n] {
		return
	}
	a.reported[n] = true
	t := a.p.R.Tree(a.f.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Code: diagCode("use-after-move"),
		Msg: "`" + name + "` was moved in an earlier iteration of the loop; it cannot be moved again"}
	d = d.WithHelp("create the value inside the loop, or leave the loop right after moving it")
	a.f.Diags = append(a.f.Diags, d)
}
