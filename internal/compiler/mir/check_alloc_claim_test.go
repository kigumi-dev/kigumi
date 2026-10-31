package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
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
	f := funcNamed(t, p, name)
	var typ mir.Local
	for _, l := range f.Locals {
		if p.R.DirectAgg(l.Type) {
			typ = l
			break
		}
	}
	if typ.Type == 0 {
		t.Fatalf("%s has no direct local", name)
	}
	f.Locals = append(f.Locals, mir.Local{Type: typ.Type})
	box := mir.Inst{Op: mir.OpBoxDirect, Dst: mir.LocalID(len(f.Locals) - 1), Type: typ.Type, Args: []mir.LocalID{f.Params[0]}, Node: f.Blocks[0].Insts[0].Node}
	f.Blocks[0].Insts = append([]mir.Inst{box}, f.Blocks[0].Insts...)
}

func TestAllocClaimsReportABoxInAnInferredNoallocBody(t *testing.T) {
	p := planned(t, inferredNoallocSrc)
	if !p.R.Summary(funcNamed(t, p, "main.peek").Ent).Noalloc {
		t.Fatal("peek should be inferred noalloc")
	}
	withBox(t, p, "main.peek")
	if err := p.Verify(); err != nil && strings.Contains(err.Error(), "alloc-claim") {
		t.Fatalf("a box in an inferred body is a diagnostic, not a verifier fault: %v", err)
	}
	claims := mir.CheckAllocClaims(p)
	if len(claims) != 1 || claims[0].Diag.Code != "E1770" || !strings.Contains(claims[0].Diag.Msg, "`peek` is inferred `noalloc`") {
		t.Fatalf("want one E1770 for peek, got %+v", claims)
	}
}

func TestAllocClaimsKeepTheDeclaredCode(t *testing.T) {
	p := planned(t, inferredNoallocSrc)
	withBox(t, p, "main.declared")
	claims := mir.CheckAllocClaims(p)
	if len(claims) != 1 || claims[0].Diag.Code != "E1570" {
		t.Fatalf("want one E1570 for declared, got %+v", claims)
	}
}

func TestAllocClaimsAcceptABodyThatMayAllocate(t *testing.T) {
	p := planned(t, `
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
	if claims := mir.CheckAllocClaims(p); len(claims) != 0 {
		t.Fatalf("a body without a noalloc summary may box: %+v", claims)
	}
}
