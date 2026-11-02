package borrow

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Sem stops these shapes, so the tests rewrite lowered MIR: the Copy field read becomes a read of
// the move-only field, and the call to keep becomes the instruction the rule watches.
const borrowedBackstopSrc = `
type Inner = resource {
    pub id Int
}

fn Inner.drop(move self) {}

type Outer = {
    pub inner Inner
    pub n Int
}

fn mk(n: Int) -> Inner {
    Inner { id: n }
}

fn keep(i: Inner) -> Inner {
    i
}

fn Outer.steal(self) -> Inner {
    keep(mk(self.n))
}

fn take(o: &Outer) -> Inner {
    keep(mk(o.n))
}

type GBox[T] = {
    pub v T
    pub n Int
}

fn mkT[T](n: Int, d: T) -> T {
    d
}

fn keepT[T](v: T) -> T {
    v
}

fn GBox[T].stealT(self, d: T) -> T {
    keepT(mkT(self.n, d))
}
`

func TestBorrowedMoveBackstop(t *testing.T) {
	cases := []struct {
		name, fn string
		rewrite  func(p *fixture, call *midir.Inst, field midir.LocalID) bool
	}{
		{"join copy returned", "steal", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			*call = midir.Inst{Op: midir.OpCopy, Dst: call.Dst, Args: []midir.LocalID{field}}
			return true
		}},
		{"record built", "take", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			*call = midir.Inst{Op: midir.OpRecord, Dst: call.Dst, Args: []midir.LocalID{field}, Node: call.Node}
			return false
		}},
		{"variant built", "take", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			*call = midir.Inst{Op: midir.OpVariant, Dst: call.Dst, Args: []midir.LocalID{field}, Node: call.Node}
			return false
		}},
		{"consuming call", "take", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			call.Args = []midir.LocalID{field}
			return false
		}},
		{"primitive move self", "steal", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			call.Ent, call.Args = stdMethod(t, p.R, "Box", "into"), []midir.LocalID{field}
			return false
		}},
		// A type-parameter field is as move-only as a concrete one; nothing exempts it.
		{"generic field consumed", "stealT", func(p *fixture, call *midir.Inst, field midir.LocalID) bool {
			call.Args = []midir.LocalID{field}
			return false
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, body := srcProgram(t, borrowedBackstopSrc)
			g := body(tc.fn)
			read, call := fieldAndKeep(t, p, g)
			g.Locals[read.Dst].Type = g.Locals[call.Dst].Type
			holder := p.R.Types.Node(g.Locals[read.Args[0]].Type)
			if holder.Kind == sem.KRef {
				holder = p.R.Types.Node(holder.Elem)
			}
			read.Index = p.R.Field(p.R.TypeDecl(holder.Ent).Fields[0]).Index
			callNode, readNode := call.Node, read.Node
			atField := tc.rewrite(p, call, read.Dst)
			g.Diags = nil
			checkBorrowedMoves(New(p.input, p.bodyByEnt), g)
			want := callNode
			if atField {
				want = readNode
			}
			if len(g.Diags) != 1 || g.Diags[0].Code != "move-from-borrow" || g.Diags[0].Loc.Span != p.R.Tree(g.File).Span(want) {
				t.Fatalf("diagnostics = %+v, want one move-from-borrow at node %d", g.Diags, want)
			}
		})
	}
}

// fieldAndKeep finds the field read and the call to keep, and drops the call to mk that read the field.
func fieldAndKeep(t *testing.T, p *fixture, g *midir.Func) (read, call *midir.Inst) {
	t.Helper()
	for bi := range g.Blocks {
		for ii := range g.Blocks[bi].Insts {
			in := &g.Blocks[bi].Insts[ii]
			if in.Op == midir.OpField && read == nil {
				read = in
			}
			if in.Op != midir.OpCall || in.Ent == 0 {
				continue
			}
			switch p.R.Entity(in.Ent).Name {
			case "mk", "mkT":
				*in = midir.Inst{Op: midir.OpNop}
			case "keep", "keepT":
				call = in
			}
		}
	}
	if read == nil || call == nil {
		t.Fatalf("%s has no field read and call to keep", g.Name)
	}
	return read, call
}

func stdMethod(t *testing.T, r *sem.Result, owner, name string) sem.EntityID {
	for id := 1; id < len(r.Entities); id++ {
		e := r.Entity(sem.EntityID(id))
		if e.Kind == sem.EntFn && e.Name == name && e.Flags&sem.EfStd != 0 {
			if o := r.Fn(sem.EntityID(id)).Owner; o != 0 && r.Entity(o).Name == owner {
				return sem.EntityID(id)
			}
		}
	}
	t.Fatalf("no std %s.%s", owner, name)
	return 0
}
