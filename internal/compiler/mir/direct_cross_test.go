package mir_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func TestCountlessRefsFollowWhereTheReferenceWasMade(t *testing.T) {
	p := program(t)
	ref := p.R.Types.Ref(sem.TyI64, false)
	locals := []midir.Local{
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
	insts := []midir.Inst{
		{Op: midir.OpCall, Dst: 2, Args: []midir.LocalID{0}},
		{Op: midir.OpField, Dst: 3, Args: []midir.LocalID{0}},
		{Op: midir.OpShare, Dst: 4, Args: []midir.LocalID{0}},
		{Op: midir.OpBorrow, Dst: 5, Args: []midir.LocalID{1}},
		{Op: midir.OpLendBox, Dst: 6, Args: []midir.LocalID{0}},
		{Op: midir.OpBuiltin, Dst: 7, Str: "iter.at_ref", Args: []midir.LocalID{0, 1}},
		{Op: midir.OpAlias, Dst: 8, Args: []midir.LocalID{0}},
		{Op: midir.OpAlias, Dst: 9, Args: []midir.LocalID{2}},
		{Op: midir.OpCopy, Dst: 10, Args: []midir.LocalID{7}},
		{Op: midir.OpCopy, Dst: 11, Args: []midir.LocalID{7}, Share: true},
		{Op: midir.OpCall, Dst: 12, Args: []midir.LocalID{0}},
	}
	f := fn("refs", sem.TyI64, locals, midir.Block{Insts: insts, Term: ret(12)})
	want := map[string]bool{"borrow": true, "lendbox": true, "atref": true, "aliasNamed": true, "copyAtRef": true}
	for l, got := range p.CountlessRefs(f) {
		if name := locals[l].Name; got != want[name] {
			t.Errorf("%s holds no count of its own: %v, want %v", name, got, want[name])
		}
	}
}
