package ownership

import midir "kigumi/internal/compiler/mir/ir"

// A Rest move empties a scrutinee whose parts the pattern already took; the
// payload moves before it carry the state, so it changes nothing here.
func (a *owner) moveInst(in *midir.Inst, st *dstate, report bool) {
	if in.Rest {
		return
	}
	src := a.moveSource(in, st, report)
	if !a.tracked[src] {
		return
	}
	a.check(in, src, st.val.get(src), report && !in.Guarded)
	a.checkWhole(in.Node, src, st, report)
	a.checkDropOwner(in.Node, src, report)
	a.checkGuardHeld(in, src, st, report && !in.Guarded, 0)
	st.val.set(src, ownMoved)
	st.at.set(src, in.Node)
	a.noteMove(in, src, report)
	a.moveGuardSource(in, src, st, report)
}
