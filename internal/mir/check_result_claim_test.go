package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
)

const resultClaimSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn mk(x: Int) -> Point { Point { x: x, y: x } }

fn peek(p: Point) -> Int { p.x }

noalloc fn declared(p: Point) -> Int { p.y }

fn main() -> Unit! {}
`

// withValueOf forms a value of fn in name's body, whose entry boxes fn's result on every call.
func withValueOf(t *testing.T, p *mir.Program, name, fn string) {
	t.Helper()
	f, g := funcNamed(t, p, name), funcNamed(t, p, fn)
	f.Locals = append(f.Locals, mir.Local{Type: p.R.Fn(g.Ent).Sig})
	f.Direct = append(f.Direct, false)
	item := mir.Inst{Op: mir.OpFnItem, Dst: mir.LocalID(len(f.Locals) - 1), Ent: g.Ent, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]mir.Inst{item}, f.Blocks[0].Insts...)
}

func TestAllocClaimsReportAValueWhoseEntryBoxes(t *testing.T) {
	p := planned(t, resultClaimSrc)
	if !p.R.Summary(funcNamed(t, p, "main.peek").Ent).Noalloc {
		t.Fatal("peek should be inferred noalloc")
	}
	withValueOf(t, p, "main.peek", "main.mk")
	withValueOf(t, p, "main.declared", "main.mk")
	codes := map[string]string{}
	for _, c := range mir.CheckAllocClaims(p) {
		codes[c.Diag.Code] = c.Diag.Msg
	}
	if !strings.Contains(codes["E1843"], "`peek` is inferred `noalloc`") || !strings.Contains(codes["E1842"], "`declared` is declared `noalloc`") || len(codes) != 2 {
		t.Fatalf("want E1843 for peek and E1842 for declared, got %v", codes)
	}
}

func TestAllocClaimsIgnoreAValueWhoseEntryBoxesNothing(t *testing.T) {
	p := planned(t, resultClaimSrc)
	withValueOf(t, p, "main.declared", "main.peek")
	if claims := mir.CheckAllocClaims(p); len(claims) != 0 {
		t.Fatalf("peek's entry only unboxes, yet got %+v", claims)
	}
}
