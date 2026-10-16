package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (a *owner) inCleanup(n syntax.NodeID) bool {
	if n == 0 {
		return false
	}
	t := a.p.R.Tree(a.f.File)
	sp := t.Span(n)
	for _, c := range a.f.Cleanups {
		if cs := t.Span(c); sp.Start >= cs.Start && sp.End <= cs.End {
			return true
		}
	}
	return false
}

func (a *owner) reportCleanupRead(n syntax.NodeID, l LocalID, name string) bool {
	if !a.inCleanup(n) {
		return false
	}
	if a.reported[n] {
		return true
	}
	a.reported[n] = true
	code, _ := sem.CodeByName("cleanup-reads-moved")
	msg, help := sem.FillCode(code, name)
	t := a.p.R.Tree(a.f.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: code.Num}
	if help != "" {
		d.Help = []string{help}
	}
	if at := a.cur.at[l]; at != 0 {
		d = d.WithNote(diag.At(t.File, t.Span(at)), "`"+name+"` is moved here")
	}
	a.f.Diags = append(a.f.Diags, d)
	return true
}
