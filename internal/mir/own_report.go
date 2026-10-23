package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

type owner struct {
	p        *Program
	f        *Func
	pl       *places
	tracked  map[LocalID]bool
	in       []*dstate
	reported map[syntax.NodeID]bool
	defNode  map[LocalID]syntax.NodeID
	cur      *dstate
	capSrc   map[LocalID]LocalID
	pos      movePos
	moves    []moveEvent
	guardSrc map[LocalID]LocalID
	fieldMv  map[LocalID]bool
	payLine  map[[2]int]bool
	cellDefs map[LocalID]*Inst
}

// merge joins a predecessor's exit state into a block; it reports
// whether the block's entry state changed.
func (a *owner) merge(bid BlockID, out *dstate) bool {
	cur := a.in[bid]
	if cur == nil {
		a.in[bid] = cloneD(out)
		return true
	}
	c1 := cur.val.join(&out.val)
	c2 := cur.field.join(&out.field)
	cur.at.join(&out.at)
	cur.callAt.join(&out.callAt)
	c3 := cur.called.join(&out.called)
	c4 := cur.rel.join(&out.rel)
	return c1 || c2 || c3 || c4
}

func (a *owner) check(in *Inst, l LocalID, s ownState, report bool) {
	if !report || in == nil {
		return
	}
	name := a.f.Locals[l].Name
	if n, ok := a.pl.name[l]; ok {
		name = n
	}
	if (s == ownMoved || s == ownConflict) && a.reportCleanupRead(in.Node, l, name) {
		return
	}
	if in.Node == 0 {
		return
	}
	if par, ok := a.pl.parent[l]; ok && a.pl.payloadStep(l) && (s == ownMoved || s == ownConflict) && !a.firstForMove(par, a.cur.at.get(l)) {
		return
	}
	switch s {
	case ownMoved:
		a.reportMoved(in.Node, l, name)
	case ownConflict:
		if a.cur.at.get(l) == in.Node {
			a.reportLoopMove(in.Node, name)
			return
		}
		a.report(in.Node, "move-join-mismatch", "`"+name+"` is moved on some paths but not others")
	}
}

// checkWhole reports a use of l as a whole value while one of its own
// fields is still moved out (E1025), naming that field.
func (a *owner) checkWhole(n syntax.NodeID, l LocalID, st *dstate, report bool) {
	if !report {
		return
	}
	fields := a.movedFields(l, st)
	if len(fields) == 0 {
		return
	}
	rep := a.pl.children[l][fields[0]]
	if !a.pl.payloadStep(rep) {
		code, msg := partialMoveDiag(a.fieldName(l, fields[0]), a.ownerName(l))
		a.report(n, code, msg)
		return
	}
	// A taken payload leaves nothing of the value to name, so this reads as a use of the whole after its move.
	if n != 0 && a.firstForMove(l, st.at.get(rep)) {
		s := st.field.get(rep)
		if gone(st.val.get(rep)) {
			s = st.val.get(rep)
		}
		a.check(&Inst{Node: n}, l, s, true)
	}
}

// firstForMove keeps one diagnostic per taken payload: a match re-reads the tag in every arm and then the payload.
func (a *owner) firstForMove(root LocalID, at syntax.NodeID) bool {
	k := [2]int{int(root), int(at)}
	if a.payLine == nil {
		a.payLine = map[[2]int]bool{}
	}
	if a.payLine[k] {
		return false
	}
	a.payLine[k] = true
	return true
}

// partialMoveDiag renders E1025 from the sem catalog (codes_body.go's
// cMovePartialUse) so own.go and own_partial.go share one wording instead
// of two hand-written literals.
func partialMoveDiag(field, owner string) (string, string) {
	c, _ := sem.CodeByName("move-partial-use")
	msg := sem.FillTemplate(c.Template, field, owner)
	if c.Help != "" {
		msg += "; " + c.Help
	}
	return c.Num, msg
}

// fieldName renders root's field idx the way it was written at its move.
func (a *owner) fieldName(root LocalID, idx int) string {
	rep := a.pl.children[root][idx]
	if n, ok := a.pl.name[rep]; ok {
		return n
	}
	return a.f.Locals[rep].Name
}

func (a *owner) ownerName(l LocalID) string {
	if n, ok := a.pl.name[l]; ok {
		return n
	}
	return a.f.Locals[l].Name
}

func (a *owner) report(n syntax.NodeID, code, msg string) {
	if n == 0 || a.reported[n] {
		return
	}
	a.reported[n] = true
	t := a.p.R.Tree(a.f.File)
	a.f.Diags = append(a.f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: diagCode(code)})
}

// HasErrors reports whether any function carries an ownership error.
func (p *Program) HasErrors() bool {
	for _, f := range p.Funcs {
		if len(f.Diags) > 0 {
			return true
		}
	}
	return false
}

// Render prints the ownership diagnostics of every function.
func (p *Program) Render() string {
	out := ""
	for _, f := range p.Funcs {
		if len(f.Diags) == 0 {
			continue
		}
		out += diag.RenderAll(p.R.Tree(f.File).File, f.Diags)
	}
	return out
}
