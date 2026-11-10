package verify_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

// A local held through a live `&mut` may not be borrowed mutably again: the
// second callee could write it while the first holder still reads it.
func TestVerifyRejectsAliasedMutBorrows(t *testing.T) {
	p := mirfixture.Program(t)
	ref := p.R.Types.Ref(sem.TyI64, true)
	locals := []midir.Local{
		{Name: "x", Type: sem.TyI64},
		{Name: "b1", Type: ref},
		{Name: "b2", Type: ref},
		{Name: "u", Type: sem.TyUnit},
	}
	f := mirfixture.Fn("overlap", sem.TyUnit, locals, midir.Block{
		Insts: []midir.Inst{
			{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
			{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}, Index: 1},
			{Op: midir.OpBorrow, Dst: 2, Args: []midir.LocalID{0}, Index: 1},
			{Op: midir.OpUnit, Dst: 3},
		},
		Term: mirfixture.Ret(3),
	})
	expectRule(t, p, f, "exclusive", "borrows %x mutably while %b1 still holds a mutable borrow of it")
}

// Borrows that do not overlap are what the rule must leave alone: sequential
// borrows of one local end with the reference, and two field borrows name two
// different locals.
func TestVerifyAllowsSequentialAndDisjointMutBorrows(t *testing.T) {
	p := mirfixture.Built(t, `
type P = { pub mut a Int, pub mut b Int }
fn bump(k: &mut Int) { k = k + 1 }
fn twice(k: &mut Int) {
    bump(k)
    bump(k)
}
fn two(x: &mut Int, y: &mut Int) -> Int {
    x = x + 1
    y = y + 2
    x + y
}
let mut p = P { a: 1, b: 2 }
let mut n = 0
twice(&mut n)
let r = two(&mut p.a, &mut p.b)
print "${n} ${r}"
`)
	if n := mirfixture.CountOp(mirfixture.FuncNamed(t, p, "main"), midir.OpBorrow); n < 2 {
		t.Fatalf("main has %d borrows, want the sequential and disjoint ones to exercise the rule", n)
	}
}

// A borrow carried across a block edge conflicts while its reference is still
// live there, even though the second borrow is in a later block.
func TestVerifyRejectsMutBorrowLiveAcrossBlocks(t *testing.T) {
	p := mirfixture.Program(t)
	ref := p.R.Types.Ref(sem.TyI64, true)
	locals := []midir.Local{{Name: "x", Type: sem.TyI64}, {Name: "r", Type: ref}, {Name: "s", Type: ref}}
	f := mirfixture.Fn("carried", ref, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
			{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}, Index: 1},
		}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{1}}},
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpBorrow, Dst: 2, Args: []midir.LocalID{0}, Index: 1},
		}, Term: mirfixture.Ret(1)})
	expectRule(t, p, f, "exclusive", "borrows %x mutably while %r still holds a mutable borrow of it")
}

// A `&mut` nothing reads ends with its block, so a loop that borrows the same
// local again on the next iteration does not overlap with it.
func TestVerifyAllowsUnreadMutBorrowAcrossALoop(t *testing.T) {
	p := mirfixture.Program(t)
	ref := p.R.Types.Ref(sem.TyI64, true)
	locals := []midir.Local{{Name: "x", Type: sem.TyI64}, {Name: "r", Type: ref}, {Name: "s", Type: ref}, {Name: "u", Type: sem.TyUnit}}
	unit := midir.Inst{Op: midir.OpUnit, Dst: 3}
	f := mirfixture.Fn("looped", sem.TyUnit, locals,
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpConst, Dst: 0, Type: sem.TyI64, Lit: sem.Literal{Kind: sem.LitInt}},
			{Op: midir.OpBorrow, Dst: 1, Args: []midir.LocalID{0}, Index: 1},
			unit,
		}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{1}}},
		midir.Block{Insts: []midir.Inst{
			{Op: midir.OpBorrow, Dst: 2, Args: []midir.LocalID{0}, Index: 1},
			unit,
		}, Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{0}}})
	p.Funcs = []*midir.Func{f}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("unread borrows in a loop: %v", err)
	}
}
