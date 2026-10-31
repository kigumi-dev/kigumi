package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func TestCountlessRefsFollowWhereTheReferenceWasMade(t *testing.T) {
	p := program(t)
	ref := p.R.Types.Ref(sem.TyI64, false)
	locals := []mir.Local{
		{Name: "r", Type: ref, Ent: 1},
		{Name: "n", Type: sem.TyI64, Ent: 2},
		{Name: "call", Type: ref},
		{Name: "field", Type: ref},
		{Name: "share", Type: ref},
		{Name: "borrow", Type: ref},
		{Name: "lendbox", Type: ref},
		{Name: "atref", Type: ref},
		{Name: "aliasNamed", Type: ref},
		{Name: "aliasCall", Type: ref},
		{Name: "copyAtRef", Type: ref},
		{Name: "sharedCopy", Type: ref},
		{Name: "value", Type: sem.TyI64},
	}
	insts := []mir.Inst{
		{Op: mir.OpCall, Dst: 2, Args: []mir.LocalID{0}},
		{Op: mir.OpField, Dst: 3, Args: []mir.LocalID{0}},
		{Op: mir.OpShare, Dst: 4, Args: []mir.LocalID{0}},
		{Op: mir.OpBorrow, Dst: 5, Args: []mir.LocalID{1}},
		{Op: mir.OpLendBox, Dst: 6, Args: []mir.LocalID{0}},
		{Op: mir.OpBuiltin, Dst: 7, Str: "iter.at_ref", Args: []mir.LocalID{0, 1}},
		{Op: mir.OpAlias, Dst: 8, Args: []mir.LocalID{0}},
		{Op: mir.OpAlias, Dst: 9, Args: []mir.LocalID{2}},
		{Op: mir.OpCopy, Dst: 10, Args: []mir.LocalID{7}},
		{Op: mir.OpCopy, Dst: 11, Args: []mir.LocalID{7}, Share: true},
		{Op: mir.OpCall, Dst: 12, Args: []mir.LocalID{0}},
	}
	f := fn("refs", sem.TyI64, locals, mir.Block{Insts: insts, Term: ret(12)})
	want := map[string]bool{"borrow": true, "lendbox": true, "atref": true, "aliasNamed": true, "copyAtRef": true}
	for l, got := range p.CountlessRefs(f) {
		if name := locals[l].Name; got != want[name] {
			t.Errorf("%s holds no count of its own: %v, want %v", name, got, want[name])
		}
	}
}
