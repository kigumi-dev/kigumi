package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
)

const isrPairSrc = `
type Pair = { pub x Int, pub y Int }

noalloc fn poke() -> Unit {}

export(interrupt) {
    pub fn tick() -> Unit {
        let p = Pair { x: 1, y: 2 }
        _ = p.y
        poke()
    }
}
`

// A backend lowering that boxed a value the checker proved unboxed must still be caught after lowering.
func TestInterruptLoweringCatchesReintroducedBox(t *testing.T) {
	prog := planned(t, isrPairSrc)
	if claims := mir.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("the clean handler must pass: %v", claims[0].Diag.Msg)
	}
	tick := funcNamed(t, prog, "main.tick")
	var p mir.LocalID
	for i, l := range tick.Locals {
		if l.Name == "p" {
			p = mir.LocalID(i)
		}
	}
	blk := &tick.Blocks[0]
	blk.Insts = append(blk.Insts, mir.Inst{Op: mir.OpBoxDirect, Dst: p, Type: tick.Locals[p].Type, Args: []mir.LocalID{p}})
	claims := mir.CheckInterruptLowering(prog)
	if len(claims) != 1 || claims[0].Diag.Code != "E1090" || !strings.Contains(claims[0].Diag.Msg, "boxes a `Pair` value") {
		t.Fatalf("a reintroduced box must be rejected: %+v", claims)
	}
}

func TestInterruptLoweringFollowsCallees(t *testing.T) {
	prog := planned(t, isrPairSrc)
	poke := funcNamed(t, prog, "main.poke")
	poke.Blocks[0].Insts = append(poke.Blocks[0].Insts, mir.Inst{Op: mir.OpPanic, Str: "lowered trap"})
	claims := mir.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "may panic") || len(claims[0].Diag.Notes) != 2 || !strings.Contains(claims[0].Diag.Notes[0].Msg, "calls `poke`") {
		t.Fatalf("a panic a callee's lowering introduced must be traced from the handler: %+v", claims)
	}
}

const isrMeasureSrc = `
fn measure(s: String) -> Unit {
    let r = s.len()
    _ = r
}

export(interrupt) {
    pub fn tick() -> Unit {
        measure("abc")
    }
}
`

// A parameter's release is inert only while the handler's call path feeds it an immortal value.
func TestInterruptLoweringParamFromCaller(t *testing.T) {
	prog := planned(t, isrMeasureSrc)
	if claims := mir.CheckInterruptLowering(prog); len(claims) != 0 {
		t.Fatalf("a literal argument must keep the callee's release inert: %v", claims[0].Diag.Msg)
	}
	tick := funcNamed(t, prog, "main.tick")
	replaced := false
	for b := range tick.Blocks {
		for i, in := range tick.Blocks[b].Insts {
			if in.Op == mir.OpConst && prog.R.TypeString(tick.Locals[in.Dst].Type) == "String" {
				tick.Blocks[b].Insts[i].Op = mir.OpGlobalGet
				replaced = true
			}
		}
	}
	if !replaced {
		t.Fatalf("no String literal in tick:\n%s", prog.Dump())
	}
	claims := mir.CheckInterruptLowering(prog)
	if len(claims) != 1 || !strings.Contains(claims[0].Diag.Msg, "releases a `String`") {
		t.Fatalf("a non-immortal argument must make the release forbidden: %+v", claims)
	}
}
