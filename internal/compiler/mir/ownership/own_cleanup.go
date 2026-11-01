package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

func (a *owner) cleanupAround(n syntax.NodeID) syntax.NodeID {
	if n == 0 {
		return 0
	}
	t := a.p.R.Tree(a.f.File)
	sp := t.Span(n)
	var best syntax.NodeID
	for _, c := range a.f.Cleanups {
		cs := t.Span(c)
		if sp.Start < cs.Start || sp.End > cs.End {
			continue
		}
		if best == 0 || cs.End-cs.Start < t.Span(best).End-t.Span(best).Start {
			best = c
		}
	}
	return best
}

func (a *owner) inCleanup(n syntax.NodeID) bool { return a.cleanupAround(n) != 0 }

func (a *owner) reportCleanupRead(n syntax.NodeID, l midir.LocalID, name string) bool {
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
	at := a.cur.at.get(l)
	selfMove := at != 0 && at == n
	loc := n
	if selfMove {
		loc = a.cleanupAround(n)
	}
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(loc)), Msg: msg, Code: code.Num}
	if help != "" {
		d.Help = []string{help}
	}
	if at != 0 {
		note := "`" + name + "` is moved here"
		if selfMove {
			note += " by the previous run of this cleanup"
		}
		d = d.WithNote(diag.At(t.File, t.Span(at)), note)
	}
	a.f.Diags = append(a.f.Diags, d)
	return true
}
