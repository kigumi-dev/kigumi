package mir

import (
	"maps"
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/lang/diag"
)

func (a *owner) guardSources() map[midir.LocalID]midir.LocalID {
	if a.guardSrc == nil {
		a.guardSrc = map[midir.LocalID]midir.LocalID{}
		for _, blk := range a.f.Blocks {
			for _, g := range blk.Insts {
				if g.Op != midir.OpCopy || !g.Guarded || a.f.Locals[g.Dst].Ent == 0 {
					continue
				}
				if l := a.pl.canon(g.Args[0]); a.cellTracked(l) || a.holdable(g.Args[0]) && a.tracked[l] {
					a.guardSrc[g.Dst] = l
				}
			}
		}
	}
	return a.guardSrc
}

// checkGuardHeld rejects taking or changing l while a guarded binding other than own holds a place overlapping it: its guard has neither passed nor failed (st.rel).
func (a *owner) checkGuardHeld(in *midir.Inst, l midir.LocalID, st *dstate, report bool, own midir.LocalID) {
	if !report || in.Node == 0 {
		return
	}
	srcs := a.guardSources()
	for _, bind := range slices.Sorted(maps.Keys(srcs)) {
		src := srcs[bind]
		if bind == own || st.val.get(bind) != ownValid || gone(st.val.get(src)) || gone(st.field.get(src)) || st.rel.get(src) == ownValid || !a.heldOverlap(in, src, l) {
			continue
		}
		if a.reported[in.Node] {
			return
		}
		a.reported[in.Node] = true
		t := a.p.R.Tree(a.f.File)
		verb := "moved"
		if in.Op == midir.OpBorrow || in.Op == midir.OpSetField || in.Op == midir.OpSetIndex || in.Op == midir.OpCellSet || in.Op.Yields() && in.Dst == l {
			verb = "changed"
		}
		msg := "`" + a.ownerName(l) + "` is held by the pattern binding `" + a.f.Locals[bind].Name + "` until its guard passes, so it cannot be " + verb + " here"
		a.f.Diags = append(a.f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(in.Node)), Code: "move-while-borrowed", Msg: msg})
		return
	}
}

func (a *owner) within(p, root midir.LocalID) bool {
	for {
		if p == root {
			return true
		}
		up, ok := a.pl.parent[p]
		if !ok {
			return false
		}
		p = up
	}
}

// heldOverlap: a store into a field of a container overlaps only the field it names, so a sibling of the held place stays writable.
func (a *owner) heldOverlap(in *midir.Inst, src, l midir.LocalID) bool {
	if a.within(l, src) {
		return true
	}
	if !a.within(src, l) {
		return false
	}
	if in.Op != midir.OpSetField || src == l {
		return true
	}
	c := src
	for a.pl.parent[c] != l {
		c = a.pl.parent[c]
	}
	return a.pl.children[l][in.Index] == c
}

func (a *owner) assignHeld(in *midir.Inst, st *dstate, report bool) {
	if in.Op.Yields() && a.f.Locals[in.Dst].Ent != 0 && !in.Guarded {
		a.checkGuardHeld(in, in.Dst, st, report, 0)
	}
}
