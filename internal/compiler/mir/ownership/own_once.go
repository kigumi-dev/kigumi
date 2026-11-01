package ownership

import (
	"strconv"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
)

// onceCall tracks calls of a callable that may run only once; the callable keeps its value, so its drop stays.
func (a *owner) onceCall(in *midir.Inst, st *dstate, report bool) {
	if in.Op == midir.OpClosure {
		a.onceCapture(in, st, report)
		return
	}
	if in.Op != midir.OpCallValue || in.Ent != 0 || len(in.Args) == 0 {
		return
	}
	callee := in.Args[0]
	l := a.pl.canon(callee)
	name, place, ok := a.onceCallee(callee)
	if !ok {
		return
	}
	if n := a.pl.name[l]; a.pl.isPlace(callee) && n != "" {
		name = n
	}
	if a.borrowedRoot(l) {
		if report {
			a.reportBorrowedOnce(in, name)
		}
		return
	}
	if !a.tracked[l] {
		return
	}
	if s := st.called.get(l); report && (s == ownMoved || s == ownConflict) {
		a.reportOnce(in, l, name, place)
	}
	st.called.set(l, ownMoved)
	st.callAt.set(l, in.Node)
}

func (a *owner) onceCallee(l midir.LocalID) (name, place string, ok bool) {
	tt := a.p.R.Types
	t := a.f.Locals[l].Type
	name = a.f.Locals[l].Name
	switch tt.Kind(t) {
	case sem.KClosure:
		info := a.p.R.Closure(tt.Node(t).Ent)
		return name, info.Once, info.Once != ""
	case sem.KParam:
		for _, con := range a.p.R.TypeParam(tt.Node(t).Ent).Constraints {
			if con.Kind == sem.CFnOnce {
				return name, "", true
			}
		}
	}
	return "", "", false
}

func (a *owner) reportOnce(in *midir.Inst, l midir.LocalID, name, place string) {
	if a.reported[in.Node] {
		return
	}
	a.reported[in.Node] = true
	var code sem.Code
	var msg, help string
	if place != "" {
		code, _ = sem.CodeByName("closure-called-once")
		msg, help = sem.FillCode(code, name, place, place)
	} else {
		code, _ = sem.CodeByName("fn-once-called-once")
		msg, help = sem.FillCode(code, name)
	}
	t := a.p.R.Tree(a.f.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(in.Node)), Msg: msg, Code: code.Num}
	if help != "" {
		d.Help = []string{help}
	}
	if at := a.cur.callAt.get(l); at != 0 {
		d = d.WithNote(diag.At(t.File, t.Span(at)), "`"+name+"` may already be called here")
	}
	a.f.Diags = append(a.f.Diags, d)
}

// borrowedRoot reports a place reached through a borrow: every call of the borrowing function may call it again.
func (a *owner) borrowedRoot(l midir.LocalID) bool {
	k := a.pl.key[l]
	if k == "" {
		return false
	}
	id, err := strconv.Atoi(strings.SplitN(k, ".", 2)[0])
	if err != nil {
		return false
	}
	root := a.f.Locals[id]
	return root.Borrowed || a.p.R.Types.Kind(root.Type) == sem.KRef
}

func (a *owner) reportBorrowedOnce(in *midir.Inst, name string) {
	if a.reported[in.Node] {
		return
	}
	a.reported[in.Node] = true
	code, _ := sem.CodeByName("once-called-through-borrow")
	msg, help := sem.FillCode(code, name)
	t := a.p.R.Tree(a.f.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(in.Node)), Msg: msg, Code: code.Num}
	if help != "" {
		d.Help = []string{help}
	}
	a.f.Diags = append(a.f.Diags, d)
}

// Making a once-only closure counts as a call of each once-only callable it captures.
func (a *owner) onceCapture(in *midir.Inst, st *dstate, report bool) {
	if _, place, ok := a.onceCallee(in.Dst); !ok || place == "" {
		return
	}
	for _, arg := range in.Args {
		l := a.captureSource(arg)
		name, place, ok := a.onceCallee(l)
		if !ok || !a.tracked[l] || a.pl.isPlace(l) {
			continue
		}
		if s := st.called.get(l); report && (s == ownMoved || s == ownConflict) {
			a.reportOnce(in, l, name, place)
		}
		st.called.set(l, ownMoved)
		st.callAt.set(l, in.Node)
	}
}

func (a *owner) captureSource(arg midir.LocalID) midir.LocalID {
	if a.capSrc == nil {
		a.capSrc = map[midir.LocalID]midir.LocalID{}
		caps := map[midir.LocalID]bool{}
		for _, b := range a.f.Blocks {
			for _, in := range b.Insts {
				if in.Op == midir.OpClosure {
					for _, x := range in.Args {
						caps[x] = true
					}
				}
			}
		}
		for _, b := range a.f.Blocks {
			for _, in := range b.Insts {
				if (in.Op == midir.OpCopy || in.Op == midir.OpMove) && caps[in.Dst] && a.f.Locals[in.Dst].Ent == 0 {
					a.capSrc[in.Dst] = in.Args[0]
				}
			}
		}
	}
	if s, ok := a.capSrc[arg]; ok {
		return s
	}
	return arg
}
