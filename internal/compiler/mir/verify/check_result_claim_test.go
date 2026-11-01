package verify_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const resultClaimSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn mk(x: Int) -> Point { Point { x: x, y: x } }

fn peek(p: Point) -> Int { p.x }

noalloc fn declared(p: Point) -> Int { p.y }

fn main() -> Void! {}
`

// withValueOf forms a value of fn in name's body, whose entry boxes fn's result on every call.
func withValueOf(t *testing.T, p *mir.Program, name, fn string) {
	t.Helper()
	f, g := mirfixture.FuncNamed(t, p, name), mirfixture.FuncNamed(t, p, fn)
	f.Locals = append(f.Locals, midir.Local{Type: p.R.Fn(g.Ent).Sig})
	f.Direct = append(f.Direct, false)
	item := midir.Inst{Op: midir.OpFnItem, Dst: midir.LocalID(len(f.Locals) - 1), Ent: g.Ent, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]midir.Inst{item}, f.Blocks[0].Insts...)
}

func TestAllocClaimsReportAValueWhoseEntryBoxes(t *testing.T) {
	p := mirfixture.Planned(t, resultClaimSrc)
	if !p.R.Summary(mirfixture.FuncNamed(t, p, "main.peek").Ent).Noalloc {
		t.Fatal("peek should be inferred noalloc")
	}
	withValueOf(t, p, "main.peek", "main.mk")
	withValueOf(t, p, "main.declared", "main.mk")
	codes := map[string]string{}
	for _, c := range mirverify.CheckAllocClaims(p, mir.DemoteBoxedLends(p)) {
		codes[c.Diag.Code] = c.Diag.Msg
	}
	if !strings.Contains(codes["E1843"], "`peek` is inferred `noalloc`") || !strings.Contains(codes["E1842"], "`declared` is declared `noalloc`") || len(codes) != 2 {
		t.Fatalf("want E1843 for peek and E1842 for declared, got %v", codes)
	}
}

func TestAllocClaimsIgnoreAValueWhoseEntryBoxesNothing(t *testing.T) {
	p := mirfixture.Planned(t, resultClaimSrc)
	withValueOf(t, p, "main.declared", "main.peek")
	if claims := mirverify.CheckAllocClaims(p, mir.DemoteBoxedLends(p)); len(claims) != 0 {
		t.Fatalf("peek's entry only unboxes, yet got %+v", claims)
	}
}
