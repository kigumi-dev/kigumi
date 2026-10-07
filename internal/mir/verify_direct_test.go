package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
)

func directRule(t *testing.T, p *mir.Program, fragment string) {
	t.Helper()
	ve, ok := p.Verify().(*mir.Error)
	if !ok || ve.Rule != "direct" || !strings.Contains(ve.Msg, fragment) {
		t.Fatalf("got %v, want a direct error containing %q", ve, fragment)
	}
}

func TestVerifyDirectRejectsAMissingPart(t *testing.T) {
	p := planned(t, directPointSrc)
	f := funcNamed(t, p, "main.local")
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op == mir.OpField && f.IsDirect(in.Args[0]) {
				f.Blocks[bi].Insts[k].Index = 9
				directRule(t, p, "names a field its base does not have")
				return
			}
		}
	}
	t.Fatalf("no field read of a direct record:\n%s", p.Dump())
}

func TestVerifyDirectRejectsCallingARecord(t *testing.T) {
	p := planned(t, closureEnvSrc)
	f := funcNamed(t, p, "main.local")
	call, ok := firstOp(f, mir.OpCallValue)
	if !ok {
		t.Fatalf("no value call:\n%s", p.Dump())
	}
	for i := range f.Locals {
		if l := mir.LocalID(i); f.IsDirect(l) && p.R.DirectAgg(f.Locals[l].Type) && !p.R.DirectClosure(f.Locals[l].Type) {
			for bi := range f.Blocks {
				for k, in := range f.Blocks[bi].Insts {
					if in.Op == mir.OpCallValue && in.Dst == call.Dst {
						f.Blocks[bi].Insts[k].Args = append([]mir.LocalID{l}, in.Args[1:]...)
					}
				}
			}
			directRule(t, p, "calls a value that is not a closure")
			return
		}
	}
	t.Fatalf("no direct record local to call:\n%s", p.Dump())
}
