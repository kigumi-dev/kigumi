package verify_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

const inferredNoallocSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn peek(p: Point) -> Int {
    p.x
}

noalloc fn declared(p: Point) -> Int {
    p.y
}

fn main() -> Void! {}
`

func withBox(t *testing.T, p *mir.Program, name string) {
	t.Helper()
	f := mirfixture.FuncNamed(t, p, name)
	var typ midir.Local
	for _, l := range f.Locals {
		if p.R.DirectAgg(l.Type) {
			typ = l
			break
		}
	}
	if typ.Type == 0 {
		t.Fatalf("%s has no direct local", name)
	}
	f.Locals = append(f.Locals, midir.Local{Type: typ.Type})
	box := midir.Inst{Op: midir.OpBoxDirect, Dst: midir.LocalID(len(f.Locals) - 1), Type: typ.Type, Args: []midir.LocalID{f.Params[0]}, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]midir.Inst{box}, f.Blocks[0].Insts...)
}

func TestAllocClaimsReportABoxInAnInferredNoallocBody(t *testing.T) {
	p := mirfixture.Planned(t, inferredNoallocSrc)
	if !p.R.Summary(mirfixture.FuncNamed(t, p, "main.peek").Ent).Noalloc {
		t.Fatal("peek should be inferred noalloc")
	}
	withBox(t, p, "main.peek")
	if err := mirverify.Program(p); err != nil && strings.Contains(err.Error(), "alloc-claim") {
		t.Fatalf("a box in an inferred body is a diagnostic, not a verifier fault: %v", err)
	}
	claims := mirverify.CheckAllocClaims(p, mir.DemoteBoxedLends(p))
	if len(claims) != 1 || claims[0].Diag.Code != "E1770" || !strings.Contains(claims[0].Diag.Msg, "`peek` is inferred `noalloc`") {
		t.Fatalf("want one E1770 for peek, got %+v", claims)
	}
}

func TestAllocClaimsKeepTheDeclaredCode(t *testing.T) {
	p := mirfixture.Planned(t, inferredNoallocSrc)
	withBox(t, p, "main.declared")
	claims := mirverify.CheckAllocClaims(p, mir.DemoteBoxedLends(p))
	if len(claims) != 1 || claims[0].Diag.Code != "E1570" {
		t.Fatalf("want one E1570 for declared, got %+v", claims)
	}
}

func TestAllocClaimsAcceptABodyThatMayAllocate(t *testing.T) {
	p := mirfixture.Planned(t, `
type Point = {
    pub x Int
    pub y Int
}

fn grow(p: Point) -> Array[Point] {
    let mut out = Array.empty[Point]()
    out.push(p)
    out
}

fn main() -> Void! {}
`)
	if claims := mirverify.CheckAllocClaims(p, mir.DemoteBoxedLends(p)); len(claims) != 0 {
		t.Fatalf("a body without a noalloc summary may box: %+v", claims)
	}
}
