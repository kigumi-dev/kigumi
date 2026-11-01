package reach_test

import (
	"math/big"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

const staticReachSrc = `
type Leaf = { x Int, y Int }
type Mid = { leaf Leaf, m Int }
type Top = { mid Mid, n Int }
type Scratch = { v Int }
type Idle = { v Int }

fn mkTop() -> Top {
    let s = Scratch { v: 4 }
    Top { mid: Mid { leaf: Leaf { x: 1, y: 2 }, m: 3 }, n: s.v }
}

fn unused() -> Int {
    let i = Idle { v: 1 }
    i.v
}

static top: Top = comptime mkTop()
static mut count: Int = 0

fn main() {
    print "ok"
}
`

func intLit(n int64) sem.Literal { return sem.Literal{Kind: sem.LitInt, Int: big.NewInt(n)} }

func recordLit(fields ...sem.Literal) sem.Literal {
	return sem.Literal{Kind: sem.LitRecord, Fields: fields}
}

func staticNamed(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for _, id := range r.StaticEntities() {
		if r.Entity(id).Name == name {
			return id
		}
	}
	t.Fatalf("no static named %q", name)
	return 0
}

func TestReachableKeepsStaticValueTypes(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(staticReachSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	top := staticNamed(t, res, "top")
	res.SetLiteral(res.Entity(top).File, res.Static(top).Init, recordLit(recordLit(recordLit(intLit(1), intLit(2)), intLit(3)), intLit(4)))
	prog := mirbuild.Program(res)
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)
	for _, name := range []string{"Top", "Mid", "Leaf"} {
		if !rs.KeepType(entType(t, res, name)) {
			t.Errorf("%s: held by a static's value, should get a descriptor although no reachable code names it", name)
		}
	}
	for _, name := range []string{"Scratch", "Idle"} {
		if rs.KeepType(entType(t, res, name)) {
			t.Errorf("%s: named only by a comptime initializer or unreachable code, should not get a descriptor", name)
		}
	}
}
