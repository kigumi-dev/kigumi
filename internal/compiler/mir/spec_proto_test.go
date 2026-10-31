package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
)

const specProtoSrc = `
type Point = {
    pub x Int
    pub y Int
}

type Named = {
    pub name String
    pub n Int
}

fn same(a: Point, b: Point) -> Bool { a == b }

fn fingerprint(a: Point) -> u64 { hash(&a) }

fn sameTuple(a: (Int, Bool), b: (Int, Bool)) -> Bool { a == b }

fn sameNamed(a: Named, b: Named) -> Bool { a == b }

fn main() -> Void! {}
`

func builtinNamed(f *mir.Func, name string) (mir.Inst, bool) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == mir.OpBuiltin && in.Str == name {
				return in, true
			}
		}
	}
	return mir.Inst{}, false
}

func TestSpecializeTurnsADerivedEqualsIntoTheDirectComparison(t *testing.T) {
	p := specialized(t, specProtoSrc)
	same := funcNamed(t, p, "main.same")
	in, ok := builtinNamed(same, "direct.eq")
	if !ok || countOp(same, mir.OpCall) != 0 || countOp(same, mir.OpBorrow) != 0 {
		t.Fatalf("the comparison is still a call over a borrow:\n%s", p.Dump())
	}
	if f := same; !f.IsDirect(in.Args[0]) || !f.IsDirect(in.Args[1]) {
		t.Errorf("an operand of the comparison is boxed:\n%s", p.Dump())
	}
	if n := countOp(same, mir.OpBoxDirect) + countOp(same, mir.OpUnboxDirect); n != 0 {
		t.Errorf("the comparison crosses %d times", n)
	}
	body := funcNamed(t, p, "main.Point.equals")
	if body.Proto != "equals" || len(body.Blocks) != 1 || len(body.Blocks[0].Insts) != 1 {
		t.Errorf("the derived body was kept:\n%s", p.Dump())
	}
	if err := p.Verify(); err != nil {
		t.Errorf("verify: %v", err)
	}
}

func TestSpecializeTurnsTheHashOfARecordIntoTheDirectHash(t *testing.T) {
	p := specialized(t, specProtoSrc)
	f := funcNamed(t, p, "main.fingerprint")
	if _, ok := builtinNamed(f, "direct.hash"); !ok || countOp(f, mir.OpCall) != 0 || countOp(f, mir.OpBoxDirect) != 0 {
		t.Errorf("the hash is still a call or a box:\n%s", p.Dump())
	}
}

func TestSpecializeGivesATupleOfScalarsItsOwnComparison(t *testing.T) {
	p := specialized(t, specProtoSrc)
	f := funcNamed(t, p, "main.sameTuple")
	if _, ok := builtinNamed(f, "direct.eq"); !ok || countOp(f, mir.OpCall) != 0 {
		t.Errorf("the tuple comparison is still a call:\n%s", p.Dump())
	}
	found := false
	for _, name := range instances(p) {
		found = found || strings.Contains(name, "Tuple2.equals")
	}
	if !found {
		t.Errorf("no instantiation of the tuple's equals: %v", instances(p))
	}
}

func TestSpecializeLeavesARecordWithAStringAlone(t *testing.T) {
	p := specialized(t, specProtoSrc)
	f := funcNamed(t, p, "main.sameNamed")
	if _, ok := builtinNamed(f, "direct.eq"); ok || countOp(f, mir.OpCall) == 0 {
		t.Errorf("a record that is not direct was compared in place:\n%s", p.Dump())
	}
	if funcNamed(t, p, "main.Named.equals").Proto != "" {
		t.Error("the derived equals of a record holding a String was replaced")
	}
}

func TestVerifyRejectsAComparisonOfAnotherType(t *testing.T) {
	p := specialized(t, specProtoSrc)
	same := funcNamed(t, p, "main.same")
	for bi := range same.Blocks {
		for k, in := range same.Blocks[bi].Insts {
			if in.Op == mir.OpBuiltin && in.Str == "direct.eq" {
				same.Blocks[bi].Insts[k].Args = []mir.LocalID{in.Args[0]}
			}
		}
	}
	wantRule(t, p, "builtins", "direct.eq takes 2 to 2 arguments")
}
