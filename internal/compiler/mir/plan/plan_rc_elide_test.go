package plan_test

import (
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

func elideOps(t *testing.T, locals []midir.Local, insts []midir.Inst) []midir.Opcode {
	t.Helper()
	p := mirfixture.Program(t)
	f := mirfixture.Fn("f", sem.TyUnit, locals, midir.Block{Insts: insts, Term: midir.Term{Op: midir.TermUnreachable}})
	p.Funcs = []*midir.Func{f}
	if err := mirplan.ElideRC(p); err != nil {
		t.Fatal(err)
	}
	var ops []midir.Opcode
	for _, in := range f.Blocks[0].Insts {
		ops = append(ops, in.Op)
	}
	return ops
}

func arg(l midir.LocalID) []midir.LocalID { return []midir.LocalID{l} }

func TestElideRCImmortalCounts(t *testing.T) {
	locals := []midir.Local{{Name: "b", Type: sem.TyBool}, {Name: "c", Type: sem.TyBool}, {Name: "u", Type: sem.TyUnit}}
	got := elideOps(t, locals, []midir.Inst{
		{Op: midir.OpShare, Dst: 1, Args: arg(0)},
		{Op: midir.OpRelease, Args: arg(0)},
		{Op: midir.OpRelease, Args: arg(1)},
		{Op: midir.OpRelease, Args: arg(2)},
	})
	if want := []midir.Opcode{midir.OpAlias}; !slices.Equal(got, want) {
		t.Errorf("ops = %v, want %v", got, want)
	}
}

func TestElideRCRetainThenReleaseSource(t *testing.T) {
	locals := []midir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyString}}
	got := elideOps(t, locals, []midir.Inst{
		{Op: midir.OpShare, Dst: 1, Args: arg(0)},
		{Op: midir.OpRelease, Args: arg(0)},
	})
	if want := []midir.Opcode{midir.OpMove}; !slices.Equal(got, want) {
		t.Errorf("ops = %v, want %v", got, want)
	}
}

func TestElideRCRetainThenReleaseCopy(t *testing.T) {
	locals := []midir.Local{{Name: "t0", Type: sem.TyString}, {Name: "t1", Type: sem.TyString}}
	got := elideOps(t, locals, []midir.Inst{
		{Op: midir.OpShare, Dst: 1, Args: arg(0)},
		{Op: midir.OpRelease, Args: arg(1)},
	})
	if len(got) != 0 {
		t.Errorf("ops = %v, want none", got)
	}
}

func TestElideRCKeepsWhatItCannotProve(t *testing.T) {
	str := sem.TyString
	cases := map[string]struct {
		locals []midir.Local
		insts  []midir.Inst
	}{
		"copy of a type whose copy allocates": {
			[]midir.Local{{Name: "t0", Type: sem.TyBytes}, {Name: "t1", Type: sem.TyBytes}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0), Copy: true}, {Op: midir.OpRelease, Args: arg(0)}},
		},
		"named source released like a temporary": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpRelease, Args: arg(0)}},
		},
		"temporary source dropped": {
			[]midir.Local{{Name: "t0", Type: str}, {Name: "t1", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(0)}},
		},
		"borrowed receiver": {
			[]midir.Local{{Name: "self", Type: str, Ent: 1, Borrowed: true}, {Name: "t1", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(0)}},
		},
		"cell": {
			[]midir.Local{{Name: "c", Type: str, Ent: 1, Cell: true}, {Name: "t1", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(0)}},
		},
		"source dropped after another read": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}, {Name: "t2", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpCopy, Dst: 2, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(0)}},
		},
		"something dropped is the new reference": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str, Ent: 2}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(1)}, {Op: midir.OpDrop, Args: arg(0)}},
		},
		"something between": {
			[]midir.Local{{Name: "t0", Type: str}, {Name: "t1", Type: str}, {Name: "u", Type: sem.TyUnit}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpUnit, Dst: 2}, {Op: midir.OpRelease, Args: arg(0)}},
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

func TestElideRCMovesTheSourceThatDiesAtTheShare(t *testing.T) {
	str, num := sem.TyString, sem.TyI64
	cases := map[string]struct {
		locals []midir.Local
		insts  []midir.Inst
		want   []midir.Opcode
	}{
		"named source dropped next": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(0)}},
			[]midir.Opcode{midir.OpMove},
		},
		"copy of a scalar": {
			[]midir.Local{{Name: "n", Type: num, Ent: 1}, {Name: "t1", Type: num}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0), Copy: true}, {Op: midir.OpDrop, Args: arg(0)}},
			[]midir.Opcode{midir.OpMove},
		},
		"copy of a temporary scalar": {
			[]midir.Local{{Name: "t0", Type: num}, {Name: "t1", Type: num}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0), Copy: true}, {Op: midir.OpRelease, Args: arg(0)}},
			[]midir.Opcode{midir.OpMove},
		},
		"other locals dropped in between": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}, {Name: "o", Type: str, Ent: 2}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(2)}, {Op: midir.OpDrop, Args: arg(0)}},
			[]midir.Opcode{midir.OpMove, midir.OpDrop},
		},
		"the new reference released after other drops": {
			[]midir.Local{{Name: "s", Type: str, Ent: 1}, {Name: "t1", Type: str}, {Name: "o", Type: str, Ent: 2}},
			[]midir.Inst{{Op: midir.OpShare, Dst: 1, Args: arg(0)}, {Op: midir.OpDrop, Args: arg(2)}, {Op: midir.OpRelease, Args: arg(1)}},
			[]midir.Opcode{midir.OpDrop},
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := elideOps(t, c.locals, c.insts); !slices.Equal(got, c.want) {
				t.Errorf("ops = %v, want %v", got, c.want)
			}
		})
	}
}
