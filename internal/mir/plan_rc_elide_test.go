package mir_test

import (
	"slices"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func elideOps(t *testing.T, locals []mir.Local, insts []mir.Inst) []mir.Opcode {
	t.Helper()
	p := program(t)
	f := fn("f", sem.TyUnit, locals, mir.Block{Insts: insts, Term: mir.Term{Op: mir.TermUnreachable}})
	p.Funcs = []*mir.Func{f}
	if err := mir.ElideRC(p); err != nil {
		t.Fatal(err)
	}
	var ops []mir.Opcode
	for _, in := range f.Blocks[0].Insts {
		ops = append(ops, in.Op)
	}
	return ops
}

func arg(l mir.LocalID) []mir.LocalID { return []mir.LocalID{l} }

func TestElideRCImmortalCounts(t *testing.T) {
	locals := []mir.Local{{Name: "b", Type: sem.TyBool}, {Name: "c", Type: sem.TyBool}, {Name: "u", Type: sem.TyUnit}}
	got := elideOps(t, locals, []mir.Inst{
		{Op: mir.OpShare, Dst: 1, Args: arg(0)},
		{Op: mir.OpRelease, Args: arg(0)},
		{Op: mir.OpRelease, Args: arg(1)},
		{Op: mir.OpRelease, Args: arg(2)},
	})
	if want := []mir.Opcode{mir.OpAlias}; !slices.Equal(got, want) {
		t.Errorf("ops = %v, want %v", got, want)
	}
}

func TestElideRCRetainThenReleaseSource(t *testing.T) {
	locals := []mir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyString}}
	got := elideOps(t, locals, []mir.Inst{
		{Op: mir.OpShare, Dst: 1, Args: arg(0)},
		{Op: mir.OpRelease, Args: arg(0)},
	})
	if want := []mir.Opcode{mir.OpMove}; !slices.Equal(got, want) {
		t.Errorf("ops = %v, want %v", got, want)
	}
}

func TestElideRCRetainThenReleaseCopy(t *testing.T) {
	locals := []mir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyString}}
	got := elideOps(t, locals, []mir.Inst{
		{Op: mir.OpShare, Dst: 1, Args: arg(0)},
		{Op: mir.OpRelease, Args: arg(1)},
	})
	if len(got) != 0 {
		t.Errorf("ops = %v, want none", got)
	}
}

func TestElideRCKeepsWhatItCannotProve(t *testing.T) {
	str := sem.TyString
	cases := map[string]struct {
		locals []mir.Local
		insts  []mir.Inst
	}{
		"copy": {
			[]mir.Local{{Name: "t0", Type: str}, {Name: "t1", Type: str}},
			[]mir.Inst{{Op: mir.OpShare, Dst: 1, Args: arg(0), Copy: true}, {Op: mir.OpRelease, Args: arg(0)}},
		},
		"named source": {
			[]mir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}},
			[]mir.Inst{{Op: mir.OpShare, Dst: 1, Args: arg(0)}, {Op: mir.OpRelease, Args: arg(0)}},
		},
		"something between": {
			[]mir.Local{{Name: "t0", Type: str}, {Name: "t1", Type: str}, {Name: "u", Type: sem.TyUnit}},
			[]mir.Inst{{Op: mir.OpShare, Dst: 1, Args: arg(0)}, {Op: mir.OpUnit, Dst: 2}, {Op: mir.OpRelease, Args: arg(0)}},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := elideOps(t, c.locals, c.insts); len(got) != len(c.insts) {
				t.Errorf("ops = %v, want all %d kept", got, len(c.insts))
			}
		})
	}
}
