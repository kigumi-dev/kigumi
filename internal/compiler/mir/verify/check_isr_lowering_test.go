package verify_test

import (
	"strings"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const isrPairSrc = `
type Pair = { pub x Int, pub y Int }

noalloc fn poke() {}

export(interrupt) {
    pub fn tick() {
        let p = Pair { x: 1, y: 2 }
        _ = p.y
        poke()
    }
}
`

// A backend lowering that boxed a value the checker proved unboxed must still be caught after lowering.
func TestInterruptLoweringCatchesReintroducedBox(t *testing.T) {
	prog := mirfixture.Planned(t, isrPairSrc)
	if claims := mirverify.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("the clean handler must pass: %v", claims[0].Diag.Msg)
	}
	tick := mirfixture.FuncNamed(t, prog, "main.tick")
	var p midir.LocalID
	for i, l := range tick.Locals {
		if l.Name == "p" {
			p = midir.LocalID(i)
		}
	}
	blk := &tick.Blocks[0]
	blk.Insts = append(blk.Insts, midir.Inst{Op: midir.OpBoxDirect, Dst: p, Type: tick.Locals[p].Type, Args: []midir.LocalID{p}})
	claims := mirverify.CheckInterruptLowering(prog)
	if len(claims) != 1 || claims[0].Diag.Code != "E1090" || !strings.Contains(claims[0].Diag.Msg, "boxes a `Pair` value") {
		t.Fatalf("a reintroduced box must be rejected: %+v", claims)
	}
}

func TestInterruptLoweringFollowsCallees(t *testing.T) {
	prog := mirfixture.Planned(t, isrPairSrc)
	poke := mirfixture.FuncNamed(t, prog, "main.poke")
	poke.Blocks[0].Insts = append(poke.Blocks[0].Insts, midir.Inst{Op: midir.OpPanic, Str: "lowered trap"})
	claims := mirverify.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "may panic") || len(claims[0].Diag.Notes) != 2 || !strings.Contains(claims[0].Diag.Notes[0].Msg, "calls `poke`") {
		t.Fatalf("a panic a callee's lowering introduced must be traced from the handler: %+v", claims)
	}
}

const isrMeasureSrc = `
fn measure(s: String) {
    let r = s.len()
    _ = r
}

export(interrupt) {
    pub fn tick() {
        measure("abc")
    }
}
`

// A parameter's release is inert only while the handler's call path feeds it an immortal value.
func TestInterruptLoweringParamFromCaller(t *testing.T) {
	prog := mirfixture.Planned(t, isrMeasureSrc)
	if claims := mirverify.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("a literal argument must keep the callee's release inert: %v", claims[0].Diag.Msg)
	}
	tick := mirfixture.FuncNamed(t, prog, "main.tick")
	replaced := false
	for b := range tick.Blocks {
		for i, in := range tick.Blocks[b].Insts {
			if in.Op == midir.OpConst && prog.R.TypeString(tick.Locals[in.Dst].Type) == "String" {
				tick.Blocks[b].Insts[i].Op = midir.OpGlobalGet
				replaced = true
			}
		}
	}
	if !replaced {
		t.Fatalf("no String literal in tick:\n%s", prog.Dump())
	}
	claims := mirverify.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "releases a `String`") {
		t.Fatalf("a non-immortal argument must make the release forbidden: %+v", claims)
	}
}

const isrOwnerSrc = `
type Pt = { pub x Int, pub y Int }
type Wrap = { pub p Pt, pub ok Bool }
type Shape = | Dot(Pt) | Line(Pt, Pt)
type Named = { pub name String }
type Handle = resource { pub id Int }

fn Handle.drop(move self) {}

fn probe(pt Pt, wrap Wrap, shape Shape, grid FixedArray[Int, 100000], opt Option[Int], names FixedArray[String, 4], named Named, items Array[Int], text String, handle Handle, view &Named, edit &mut Named) {}

export(interrupt) {
    pub fn tick() {}
}
`

// A release of a type that cannot hold anything is inert; one that may own a heap value or a resource stays forbidden.
func TestInterruptLoweringReleaseByType(t *testing.T) {
	cases := []struct {
		param  string
		reject bool
	}{
		{"pt", false}, {"wrap", false}, {"shape", false}, {"grid", false}, {"opt", false}, {"view", false}, {"edit", false},
		{"names", true}, {"named", true}, {"items", true}, {"text", true}, {"handle", true},
	}
	for _, c := range cases {
		prog := mirfixture.Planned(t, isrOwnerSrc)
		probe, tick := mirfixture.FuncNamed(t, prog, "main.probe"), mirfixture.FuncNamed(t, prog, "main.tick")
		var typ sem.TypeID
		for _, p := range probe.Params {
			if probe.Locals[p].Name == c.param {
				typ = probe.Locals[p].Type
			}
		}
		if typ == 0 {
			t.Fatalf("no parameter %s in probe", c.param)
		}
		tick.Locals = append(tick.Locals, midir.Local{Name: "held", Type: typ})
		held := midir.LocalID(len(tick.Locals) - 1)
		blk := &tick.Blocks[0]
		blk.Insts = append(blk.Insts, midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{held}})
		claims := mirverify.CheckInterruptLowering(prog)
		switch {
		case c.reject && (len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "releases a `"+prog.R.TypeString(typ)+"`")):
			t.Errorf("%s: a release that may free memory must be rejected: %+v", c.param, claims)
		case !c.reject && len(claims) != 0:
			t.Errorf("%s: a release of a type that owns nothing must pass: %s", c.param, claims[0].Diag.Msg)
		}
	}
}
